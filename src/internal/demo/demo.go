// Package demo is what the browser demo calls: the hook, the audit check and the kill switch over
// strings instead of files. It adds no logic of its own, so the page shows what the CLI would do.
// Like the packages it wraps it does no I/O and reads no clock; the caller passes the time.
package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/audit"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/claudecode"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// Answer is one hook decision and the audit log with its line added.
type Answer struct {
	Decision  string         `json:"decision"`
	PolicyIDs []string       `json:"policy_ids"`
	Reason    string         `json:"reason"`
	Tool      string         `json:"tool"`
	Features  map[string]any `json:"features"`
	Log       string         `json:"log"`
}

// Hook answers one PreToolUse payload for agent, as `ascc hook claude-code --agent` does: anything
// that can't be loaded or parsed is a deny with the reason, and a decision that can't be added to
// the log is a deny too. policies maps a file name to its Cedar text.
func Hook(registry string, policies map[string]string, payload, agent, log, now string) Answer {
	entry := audit.Entry{Event: audit.EventDecision, Agent: agent, Decision: "deny", Time: now, InputSHA256: audit.SHA256([]byte(payload))}
	finish := func(d pdp.Decision) Answer {
		entry.Decision, entry.PolicyIDs, entry.Reason = d.Decision, d.PolicyIDs, d.Reason
		ans := Answer{Decision: d.Decision, PolicyIDs: d.PolicyIDs, Reason: d.Reason, Tool: entry.Tool, Features: entry.Features, Log: log}
		next, err := appendLine(log, entry)
		if err != nil {
			// A call that can't be recorded doesn't happen.
			ans.Decision, ans.PolicyIDs, ans.Reason = "deny", nil, "blocked: "+err.Error()
			return ans
		}
		ans.Log = next
		return ans
	}
	files := map[string][]byte{}
	for name, src := range policies {
		files[name] = []byte(src)
	}
	p, err := pdp.New([]byte(registry), files)
	if err != nil {
		return finish(pdp.Deny(err.Error()))
	}
	// The planted repo keeps ascc's files where the README puts them.
	call, err := claudecode.Map([]byte(payload), claudecode.Options{Agent: agent})
	if err == nil {
		root := strings.TrimSuffix(call.Cwd, "/")
		call, err = claudecode.Map([]byte(payload), claudecode.Options{Agent: agent, Branch: "main",
			Protected: []string{root + "/ascc.registry.json", root + "/policies", root + "/.ascc/audit.jsonl", root + "/.ascc/audit.jsonl.lock"}})
	}
	if err != nil {
		return finish(pdp.Deny(err.Error()))
	}
	entry.Tool, entry.Features = call.Request.Resource, call.Request.Context
	entry.SessionID, entry.AgentType, entry.Cwd = call.SessionID, call.AgentType, call.Cwd
	return finish(p.Decide(call.Request))
}

// Verify checks the log's hash chain and reports it as `ascc audit verify` does with no --expect-head.
func Verify(log string) (ok bool, report string) {
	count, head, err := audit.Verify([]byte(log))
	if err != nil {
		return false, fmt.Sprintf("audit: FAILED: %v (%d entries before it verify)", err, count)
	}
	return true, fmt.Sprintf("audit: ok, %d entries, head %s\naudit: the head wasn't checked. Lines cut off the end still verify; the CLI catches that with --expect-head.", count, head)
}

// SetStatus is `ascc kill` (event "kill") or `ascc revive`: it returns the registry with the agent's
// status flipped and the log with the event added. As in the CLI the registry is changed first, so
// when the event can't be recorded the flipped registry still comes back, with the error. On any
// other error both come back unchanged.
func SetStatus(registry, agent, event, log, now string) (newRegistry, newLog string, err error) {
	want, known := map[string]string{audit.EventKill: pdp.StatusSuspended, audit.EventRevive: pdp.StatusActive}[event]
	if !known {
		return registry, log, fmt.Errorf("unknown event %q", event)
	}
	reg, err := pdp.ParseRegistry([]byte(registry))
	if err != nil {
		return registry, log, err
	}
	if err := reg.SetStatus(agent, want); err != nil {
		return registry, log, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(reg); err != nil {
		return registry, log, err
	}
	next, err := appendLine(log, audit.Entry{Event: event, Agent: agent, Time: now})
	if err != nil {
		return buf.String(), log, fmt.Errorf("%s is %s in the registry, but that couldn't be recorded: %v", agent, want, err)
	}
	return buf.String(), next, nil
}

// appendLine seals e onto the log, reading only its last line as the CLI does: a log whose last line
// is damaged takes no more lines, and one edited further up still does, and still fails Verify.
func appendLine(log string, e audit.Entry) (string, error) {
	if log != "" && !strings.HasSuffix(log, "\n") {
		return "", fmt.Errorf("audit: the log doesn't end with a complete line")
	}
	seq, hash, err := audit.Last([]byte(log))
	if err != nil {
		return "", fmt.Errorf("audit: %v", err)
	}
	line, _, err := audit.Seal(seq, hash, e)
	if err != nil {
		return "", fmt.Errorf("audit: %v", err)
	}
	return log + string(line), nil
}
