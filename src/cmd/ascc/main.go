// Command ascc is the AI Security Command Center CLI. `decide` answers one request against the agent
// registry and the Cedar policies; `hook claude-code` does the same for every Claude Code tool call,
// blocks the denied ones and records each decision in a hash-chained log; `audit verify` checks that
// log; `kill` and `revive` suspend and restore an agent; `posture claude-code` checks that the hook is
// registered so that it enforces; `discover` finds agent setups and MCP servers the registry doesn't
// know; `inventory` writes the registry, agents and the non-human identities they use, as an Agent
// Security Suite inventory document, merging in other suite tools' documents with --import.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/audit"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/claudecode"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/version"
)

const usage = `ascc decides what an AI agent may do, records it, and stops the agent when it shouldn't.

Usage:
  ascc decide    --request <file|->            answer one request; nothing is recorded
  ascc hook claude-code --agent <id> [--repo <dir>]
                                               Claude Code PreToolUse hook: payload on stdin
  ascc audit verify [--expect-head <hash>]     check the audit log's hash chain
  ascc kill <agent>                            suspend an agent: everything it asks for is denied.
                                               Prints the identities it uses, for you to rotate
  ascc revive <agent>                          restore a suspended agent
  ascc posture claude-code [--agent <id>]      check the hook is registered so that it enforces
  ascc discover [--root <dir>] [--out <file>]  report agent setups and MCP servers not in the registry
  ascc inventory [--import <file>]... [--out <file>]
                                               write the inventory document: agents and the identities
                                               they use, plus identities (not findings) from other
                                               suite documents
  ascc version

Every command takes --registry <file> (default ascc.registry.json), --policies <dir> (default
policies, every *.cedar file in it) and --audit <file> (default .ascc/audit.jsonl).

posture and inventory read the Claude Code settings, and never write them: --managed <file>
(default: the system managed-settings.json), --user <file> (default ~/.claude/settings.json) and
--project <dir> (default the current folder).

Exit codes: 0 done (for decide and hook, a decision was made, allow or deny); 1 audit verify found
a broken log, posture found a hook that doesn't enforce, or discover couldn't read everything under
its root; 2 ascc couldn't run. hook blocks the tool call on 2 as well as on a deny.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	// A panic must never look like success: for the hook, exit 2 blocks the call.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "ascc: internal error: %v\n", r)
			code = 2
		}
	}()
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("ascc "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	registry := fs.String("registry", "ascc.registry.json", "agent registry file")
	policies := fs.String("policies", "policies", "directory of *.cedar policy files")
	auditLog := fs.String("audit", filepath.Join(".ascc", "audit.jsonl"), "audit log file")

	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "ascc "+version.Version)
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0

	case "decide":
		request := fs.String("request", "", "request JSON file, or - for stdin")
		if fs.Parse(rest) != nil {
			return 2
		}
		d, err := decide(*registry, *policies, *request, stdin)
		writeJSON(stdout, d)
		return fail(stderr, err)

	case "inventory":
		out := fs.String("out", "", "write the document here instead of stdout")
		var imports []string
		fs.Func("import", "merge the identities of another suite inventory document (repeatable)", func(file string) error {
			imports = append(imports, file)
			return nil
		})
		sp := settingsFlags(fs)
		if fs.Parse(rest) != nil {
			return 2
		}
		if fs.NArg() > 0 {
			return fail(stderr, errors.New("usage: ascc inventory [--import <file>]... [--out <file>]"))
		}
		return fail(stderr, inventory(*registry, *policies, *auditLog, *out, imports, sp, stdout, stderr))

	case "discover":
		root := fs.String("root", ".", "folder to search for agent config files")
		out := fs.String("out", "", "write the document here instead of stdout")
		if fs.Parse(rest) != nil {
			return 2
		}
		if fs.NArg() > 0 {
			return fail(stderr, errors.New("usage: ascc discover [--root <dir>] [--out <file>]"))
		}
		return discover(*registry, *policies, *auditLog, *root, *out, stdout, stderr)

	case "posture":
		agent := fs.String("agent", "", "check only this agent (default: every claude-code agent in the registry)")
		sp := settingsFlags(fs)
		if len(rest) == 0 || rest[0] != "claude-code" {
			return fail(stderr, errors.New("the only posture check is `ascc posture claude-code`"))
		}
		if fs.Parse(rest[1:]) != nil {
			return 2
		}
		return posture(*registry, *agent, sp, stdout, stderr)

	case "hook":
		agent := fs.String("agent", "", "the registry id this hook registration speaks for")
		repo := fs.String("repo", "", "the repo root (default: the payload's cwd)")
		if len(rest) == 0 || rest[0] != "claude-code" {
			return fail(stderr, errors.New("the only hook is `ascc hook claude-code`"))
		}
		if fs.Parse(rest[1:]) != nil {
			return 2
		}
		if fs.NArg() > 0 {
			// Flags after a stray argument would be ignored, --agent among them. Exit 2 blocks the call.
			return fail(stderr, errors.New("blocked: the hook takes flags only; fix its registration"))
		}
		return hook(*registry, *policies, *auditLog, *agent, *repo, stdin, stdout, stderr)

	case "audit":
		expect := fs.String("expect-head", "", "fail unless the log ends at this hash")
		if len(rest) == 0 || rest[0] != "verify" {
			return fail(stderr, errors.New("the only audit command is `ascc audit verify`"))
		}
		if fs.Parse(rest[1:]) != nil {
			return 2
		}
		report, ok, err := verifyAudit(*auditLog, *expect)
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, report)
		if !ok {
			return 1
		}
		return 0

	case "kill", "revive":
		// The agent id may come before or after the flags.
		if fs.Parse(rest) != nil {
			return 2
		}
		agent, extra := fs.Arg(0), 0
		if fs.NArg() > 1 {
			if fs.Parse(fs.Args()[1:]) != nil {
				return 2
			}
			extra = fs.NArg()
		}
		if agent == "" || extra > 0 {
			return fail(stderr, fmt.Errorf("usage: ascc %s <agent>", cmd))
		}
		msg, err := setStatus(*registry, *auditLog, agent, map[string]string{"kill": audit.EventKill, "revive": audit.EventRevive}[cmd])
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, msg)
		return 0

	default:
		fmt.Fprintf(stderr, "ascc: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

func fail(stderr io.Writer, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, "ascc: "+err.Error())
		return 2
	}
	return 0
}

// hook answers one PreToolUse payload, following docs/reports/2026-claude-code-hook-contract.md:
// a policy deny is exit 0 with the deny JSON, an allow is exit 0 with no output so Claude Code's own
// permission rules still apply, and anything ascc can't do is stderr and exit 2, which also blocks.
// Never exit 1: Claude Code lets the call through on it.
func hook(registry, policies, auditLog, agent, repo string, stdin io.Reader, stdout, stderr io.Writer) int {
	raw, readErr := io.ReadAll(stdin)
	// The audit line gets a hash of the payload, never the payload: it can hold secrets.
	entry := audit.Entry{Event: audit.EventDecision, Agent: agent, Decision: "deny", InputSHA256: audit.SHA256(raw)}
	block := func(err error) int {
		entry.Reason = err.Error()
		if auditErr := appendAudit(auditLog, entry); auditErr != nil {
			fmt.Fprintln(stderr, "ascc: "+auditErr.Error())
		}
		fmt.Fprintln(stderr, "ascc: blocked: "+err.Error())
		return 2
	}
	if readErr != nil {
		return block(fmt.Errorf("couldn't read the hook payload: %v", readErr))
	}
	if agent == "" {
		return block(errors.New("the hook needs --agent <id> in its registration"))
	}
	p, err := load(registry, policies)
	if err != nil {
		return block(err)
	}
	protected := []string{registry, policies, auditLog, auditLog + ".lock"}
	for i, path := range protected {
		if protected[i], err = filepath.Abs(path); err != nil {
			return block(err)
		}
	}
	opts := claudecode.Options{Agent: agent, Repo: repo, Protected: protected}
	// Map once to learn the working directory, then again with the branch checked out there.
	call, err := claudecode.Map(raw, opts)
	if err != nil {
		return block(err)
	}
	if opts.Branch = gitBranch(call.Cwd); opts.Branch == "" && repo != "" {
		opts.Branch = gitBranch(repo)
	}
	if call, err = claudecode.Map(raw, opts); err != nil {
		return block(err)
	}

	d := p.Decide(call.Request)
	entry.Tool, entry.Decision, entry.PolicyIDs, entry.Reason = call.Request.Resource, d.Decision, d.PolicyIDs, d.Reason
	entry.Features, entry.SessionID, entry.AgentType, entry.Cwd = call.Request.Context, call.SessionID, call.AgentType, call.Cwd
	if err := appendAudit(auditLog, entry); err != nil {
		// A decision that can't be recorded isn't allowed to happen.
		fmt.Fprintln(stderr, "ascc: blocked: "+err.Error())
		return 2
	}
	if d.Decision != "allow" {
		if err := json.NewEncoder(stdout).Encode(hookDeny(d.Reason)); err != nil {
			fmt.Fprintln(stderr, "ascc: blocked: "+d.Reason)
			return 2
		}
	}
	return 0
}

// decide always returns a decision to print. When ascc can't load its inputs the decision is a deny
// carrying the error, and the error comes back too so the exit code says ascc couldn't run.
func decide(registry, policies, request string, stdin io.Reader) (pdp.Decision, error) {
	deny := func(err error) (pdp.Decision, error) { return pdp.Deny(err.Error()), err }
	p, err := load(registry, policies)
	if err != nil {
		return deny(err)
	}
	var data []byte
	switch request {
	case "":
		return deny(errors.New("decide needs --request <file|->"))
	case "-":
		data, err = io.ReadAll(stdin)
	default:
		data, err = os.ReadFile(request)
	}
	if err != nil {
		return deny(fmt.Errorf("request: %v", err))
	}
	req, err := pdp.ParseRequest(data)
	if err != nil {
		return deny(err)
	}
	return p.Decide(req), nil
}

// inventory writes the document. An --import file that can't be read or isn't a valid suite document
// stops it: a merged list with a silent hole in it would read as complete. Two imports that describe
// one id differently don't stop it, since the first still wins, but stderr names the id and both files.
// It names nothing else: what the two entries say stays out of the terminal.
func inventory(registry, policies, auditLog, out string, imports []string, sp settingsPaths, stdout, stderr io.Writer) error {
	p, err := load(registry, policies)
	if err != nil {
		return err
	}
	var docs []pdp.Inventory
	type seenIn struct {
		file     string
		identity pdp.Identity
	}
	seen := map[string]seenIn{}
	for _, file := range imports {
		st, err := os.Stat(file)
		if err != nil || !st.Mode().IsRegular() || st.Size() > maxConfigBytes {
			return fmt.Errorf("--import %s: not a file ascc can read", file)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("--import %s: not a file ascc can read", file)
		}
		doc, err := pdp.ParseInventory(data)
		if err != nil {
			return fmt.Errorf("--import %s: %v", file, err)
		}
		for _, n := range doc.Identities {
			if first, ok := seen[n.ID]; !ok {
				seen[n.ID] = seenIn{file, n}
			} else if first.file != file && !reflect.DeepEqual(first.identity, n) {
				fmt.Fprintf(stderr, "warning: --import %s describes %q differently from %s; the first is used unless the registry declares it\n", file, n.ID, first.file)
			}
		}
		docs = append(docs, doc)
	}
	return writeDoc(out, stdout, p.Inventory(version.Version, time.Now(), evidence(auditLog, sp, stderr), docs...), registry, policies, auditLog)
}

// writeDoc prints a document, or writes it to out when that is given. It won't write over ascc's own
// state (the registry, the audit log, anything in the policies folder): only store.go changes those.
func writeDoc(out string, stdout io.Writer, doc any, state ...string) error {
	if out == "" {
		return writeJSON(stdout, doc)
	}
	same := func(a, b string) bool {
		sa, errA := os.Stat(a)
		sb, errB := os.Stat(b)
		return errA == nil && errB == nil && os.SameFile(sa, sb)
	}
	for _, s := range state {
		if same(out, s) || same(filepath.Dir(out), s) {
			return errors.New("--out would write over ascc's own registry, policies or audit log; choose another file")
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := writeJSON(f, doc); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func load(registry, policies string) (*pdp.PDP, error) {
	reg, err := os.ReadFile(registry)
	if err != nil {
		return nil, fmt.Errorf("registry: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(policies, "*.cedar"))
	if err != nil {
		return nil, fmt.Errorf("policies: %v", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("policies: no .cedar file in %s", policies)
	}
	files := map[string][]byte{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("policies: %v", err)
		}
		files[filepath.Base(path)] = src
	}
	return pdp.New(reg, files)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
