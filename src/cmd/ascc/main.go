// Command ascc is the AI Security Command Center CLI. `decide` answers one request against the agent
// registry and the Cedar policies; `hook claude-code` does the same for every Claude Code tool call,
// blocks the denied ones and records each decision in a hash-chained log; `audit verify` checks that
// log; `kill` and `revive` suspend and restore an agent; `inventory` writes the registry as an Agent
// Security Suite inventory document.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
  ascc kill <agent>                            suspend an agent: everything it asks for is denied
  ascc revive <agent>                          restore a suspended agent
  ascc inventory [--out <file>]                write the agent inventory document
  ascc version

Every command takes --registry <file> (default ascc.registry.json), --policies <dir> (default
policies, every *.cedar file in it) and --audit <file> (default .ascc/audit.jsonl).

Exit codes: 0 done (for decide and hook, a decision was made, allow or deny); 1 audit verify found
a broken log; 2 ascc couldn't run. hook blocks the tool call on 2 as well as on a deny.
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
		if fs.Parse(rest) != nil {
			return 2
		}
		return fail(stderr, inventory(*registry, *policies, *out, stdout))

	case "hook":
		agent := fs.String("agent", "", "the registry id this hook registration speaks for")
		repo := fs.String("repo", "", "the repo root (default: the payload's cwd)")
		if len(rest) == 0 || rest[0] != "claude-code" {
			return fail(stderr, errors.New("the only hook is `ascc hook claude-code`"))
		}
		if fs.Parse(rest[1:]) != nil {
			return 2
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

func inventory(registry, policies, out string, stdout io.Writer) error {
	p, err := load(registry, policies)
	if err != nil {
		return err
	}
	doc := p.Inventory(version.Version, time.Now())
	if out == "" {
		return writeJSON(stdout, doc)
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
