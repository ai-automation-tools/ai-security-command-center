// Package claudecode turns a Claude Code PreToolUse hook payload into a PDP request. The contract it
// follows is docs/reports/2026-claude-code-hook-contract.md.
//
// It does no file, network or clock access, so the WebAssembly demo can replay payloads through the
// same code the hook runs.
//
// The command features are read from the command's text. That is a heuristic: an agent that encodes,
// aliases or downloads its command can dodge it. Policy on these features narrows what an agent does
// by accident or by a plain injected line; it is not a sandbox.
package claudecode

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// Options is what the payload can't say for itself.
type Options struct {
	Agent     string   // the registry id this hook registration speaks for (--agent)
	Repo      string   // the repo root; the payload's cwd when empty
	Branch    string   // the checked-out git branch, if the caller could read it
	Protected []string // absolute paths of ascc's own state: registry, policies directory, audit log
}

// Call is a mapped tool call: the request to decide, and context for the audit line.
type Call struct {
	Request   pdp.Request
	SessionID string
	AgentType string
	Cwd       string
}

type payload struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	Cwd           string         `json:"cwd"`
	SessionID     string         `json:"session_id"`
	AgentType     string         `json:"agent_type"`
}

var writeTools = []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}

// Map reads one payload. Any error means the caller must block the call.
func Map(raw []byte, o Options) (Call, error) {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Call{}, errors.New("hook payload isn't valid JSON")
	}
	if p.HookEventName != "PreToolUse" {
		return Call{}, fmt.Errorf("hook payload is for event %q, not PreToolUse", p.HookEventName)
	}
	if p.ToolName == "" {
		return Call{}, errors.New("hook payload has no tool_name")
	}

	cwd := norm(p.Cwd, "")
	repo := cwd
	if o.Repo != "" {
		repo = norm(o.Repo, "")
	}
	inRepo, network, secret, state := false, false, false, false
	ctx := map[string]any{}

	target, hasPath := "", false
	for _, key := range []string{"file_path", "notebook_path", "path"} {
		if s, ok := p.ToolInput[key].(string); ok && s != "" {
			target, hasPath = norm(s, cwd), true
			break
		}
	}
	if !hasPath && (p.ToolName == "Glob" || p.ToolName == "Grep") {
		target, hasPath = cwd, true // both search the working directory when no path is given
	}
	if hasPath {
		inRepo = under(repo, target)
		secret = secretPath(target)
		if slices.Contains(writeTools, p.ToolName) {
			state = claudeSettings(target)
			for _, prot := range o.Protected {
				state = state || under(norm(prot, ""), target)
			}
		}
	}

	if command, ok := p.ToolInput["command"].(string); ok {
		// Bash, PowerShell and Monitor all carry a command; they are parsed the same way.
		text := strings.ReplaceAll(command, `\`, "/")
		lower := strings.ToLower(text)
		inRepo = under(repo, cwd)
		network = networkRE.MatchString(lower)
		state = state || strings.Contains(lower, ".cedar") || strings.Contains(lower, ".claude/settings") ||
			strings.Contains(lower, "managed-settings.json")
		for _, prot := range o.Protected {
			state = state || strings.Contains(lower, path.Base(norm(prot, "")))
		}
		for _, segment := range segmentRE.Split(text, -1) {
			tokens := strings.FieldsFunc(segment, func(r rune) bool { return strings.ContainsRune(" \t\"'`=@,<>(){}", r) })
			for i, tok := range tokens {
				secret = secret || secretPath(strings.ToLower(tok))
				if prog := program(tok); prog == "ascc" {
					state = state || slices.ContainsFunc(tokens[i+1:], func(t string) bool { return t == "kill" || t == "revive" })
				} else if prog == "git" {
					if t, ok := gitPushTarget(tokens[i+1:], o.Branch); ok {
						ctx["git_push_target"], network = t, true
					}
				}
			}
		}
	}
	if p.ToolName == "WebFetch" || p.ToolName == "WebSearch" || strings.HasPrefix(p.ToolName, "mcp__") {
		network = true // an MCP server can reach anything; ascc can't see inside it
	}

	ctx["path_in_repo"], ctx["network"] = inRepo, network
	ctx["touches_secret_path"], ctx["touches_ascc_state"] = secret, state
	return Call{
		Request: pdp.Request{Principal: o.Agent, Action: pdp.ActionToolUse,
			Resource: "claude-code/" + p.ToolName, Context: ctx},
		SessionID: p.SessionID, AgentType: p.AgentType, Cwd: p.Cwd,
	}, nil
}

var (
	// A program name that can open a connection, or a URL, anywhere in the command.
	networkRE = regexp.MustCompile(`(^|[^a-z0-9_.-])(curl|wget|nc|ncat|netcat|ssh|scp|sftp|rsync|ftp|telnet|` +
		`invoke-webrequest|iwr|invoke-restmethod|irm|start-bitstransfer)(\.exe)?([^a-z0-9_-]|$)` +
		`|(https?|ftp|wss?)://|/dev/tcp/|(^|[^a-z0-9_.-])git(\.exe)?\s+(.*\s)?(push|fetch|pull|clone)(\s|$)`)
	segmentRE = regexp.MustCompile(`&&|\|\||[;|&\n]`)
	driveRE   = regexp.MustCompile(`^[a-z]:/`)
)

// norm makes a path comparable: forward slashes, lower case (Windows paths arrive in either case),
// relative paths joined to base, and "." and ".." resolved.
// ponytail: symlinks aren't resolved, so a link inside the repo that points outside it counts as
// inside. Resolve them in the caller if a policy ever depends on it.
func norm(p, base string) string {
	p = strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
	if p == "" {
		return ""
	}
	if base != "" && !strings.HasPrefix(p, "/") && !driveRE.MatchString(p) {
		p = base + "/" + p
	}
	return path.Clean(p)
}

func under(root, p string) bool {
	return root != "" && (p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/"))
}

func program(token string) string {
	return strings.TrimSuffix(path.Base(strings.ToLower(token)), ".exe")
}

// secretPath reports whether a lower-case, forward-slash path looks like a file that holds a secret.
func secretPath(p string) bool {
	base := path.Base(p)
	switch {
	case base == ".env":
		return true
	case strings.HasPrefix(base, ".env."):
		return !slices.Contains([]string{".env.example", ".env.sample", ".env.template"}, base)
	case slices.Contains([]string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", ".netrc", ".npmrc", ".pypirc",
		".git-credentials", "credentials.json"}, base):
		return true
	}
	for _, ext := range []string{".pem", ".key", ".pfx", ".p12"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	for _, dir := range []string{".ssh/", ".aws/", ".gnupg/"} {
		if strings.HasPrefix(p, dir) || strings.Contains(p, "/"+dir) {
			return true
		}
	}
	return false
}

func claudeSettings(p string) bool {
	return strings.Contains(p, "/.claude/settings") || path.Base(p) == "managed-settings.json"
}

// gitPushTarget reads the branch a `git push` would update from the tokens after `git`. It answers
// "*" when the push names several branches or all of them, the checked-out branch when the command
// names none, and "unknown" when that branch isn't known either.
func gitPushTarget(tokens []string, branch string) (string, bool) {
	i := 0
	for i < len(tokens) && strings.HasPrefix(tokens[i], "-") { // git's own flags, before the subcommand
		if tokens[i] == "-c" || tokens[i] == "-C" {
			i++
		}
		i++
	}
	if i >= len(tokens) || tokens[i] != "push" {
		return "", false
	}
	if branch == "" {
		branch = "unknown"
	}
	var args []string
	for i++; i < len(tokens); i++ {
		switch t := tokens[i]; {
		case t == "--all" || t == "--mirror" || t == "--branches":
			return "*", true
		case t == "-o" || t == "--push-option" || t == "--repo":
			i++
		case !strings.HasPrefix(t, "-"):
			args = append(args, t)
		}
	}
	switch {
	case len(args) < 2: // no refspec: git pushes the current branch
		return branch, true
	case len(args) > 2:
		return "*", true
	}
	ref := strings.TrimPrefix(args[1], "+")
	if j := strings.LastIndex(ref, ":"); j >= 0 {
		ref = ref[j+1:]
	}
	ref = strings.TrimPrefix(ref, "refs/heads/")
	if ref == "HEAD" || ref == "" {
		return branch, true
	}
	return ref, true
}
