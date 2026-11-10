package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/audit"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// Everything that touches the disk or the clock lives in this file and main.go, so the pdp, audit and
// claudecode packages stay importable by the WebAssembly demo.

var (
	lockWait   = 5 * time.Second  // inside the hook's 10 s timeout
	staleAfter = 30 * time.Second // past the hook's timeout, so whoever held the lock is gone
)

const tailBytes = 64 << 10 // far more than one audit line

// appendAudit seals e onto the log. Claude Code runs tool calls in parallel, so several hooks finish
// at once: the lock makes read-last-hash-then-write one step. No lock within lockWait is an error,
// and the caller turns every error into a blocked call.
func appendAudit(path string, e audit.Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	unlock, err := lock(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	tail := make([]byte, min(st.Size(), tailBytes))
	if _, err := f.ReadAt(tail, st.Size()-int64(len(tail))); err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	if len(tail) > 0 && tail[len(tail)-1] != '\n' {
		return errors.New("audit: the log doesn't end with a complete line; run `ascc audit verify`")
	}
	seq, hash, err := audit.Last(tail)
	if err != nil {
		return fmt.Errorf("audit: %v; run `ascc audit verify`", err)
	}
	e.Time = time.Now().UTC().Format(time.RFC3339)
	line, _, err := audit.Seal(seq, hash, e)
	if err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("audit: %v", err)
	}
	return f.Close()
}

// lock takes an exclusive lock by creating a file that must not exist yet.
// ponytail: a lock file, not an OS file lock, because the standard library has no portable one. A hook
// killed while holding it leaves the file behind; it is cleared once it is older than staleAfter.
// Switch to flock/LockFileEx if hooks ever legitimately run longer than that.
func lock(path string) (unlock func(), err error) {
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if st, statErr := os.Stat(path); statErr == nil && time.Since(st.ModTime()) > staleAfter {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("audit: couldn't lock the log within %s", lockWait)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// setStatus is the kill switch: it flips one agent's status in the registry file and records it.
// A retired agent stays retired: offboarding is one-way.
func setStatus(registryPath, auditPath, agentID, event string) (string, error) {
	want := map[string]string{audit.EventKill: pdp.StatusSuspended, audit.EventRevive: pdp.StatusActive}[event]
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return "", fmt.Errorf("registry: %v", err)
	}
	reg, err := pdp.ParseRegistry(data)
	if err != nil {
		return "", err
	}
	found := false
	for i := range reg.Agents {
		a := &reg.Agents[i]
		if a.ID != agentID {
			continue
		}
		found = true
		if a.Status == pdp.StatusRetired {
			return "", fmt.Errorf("agent %q is retired, and a retired agent can't be %sd", agentID, event)
		}
		a.Status = want
	}
	if !found {
		return "", fmt.Errorf("agent %q is not in the registry", agentID)
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, reg); err != nil {
		return "", err
	}
	// Write beside the registry and rename over it, so a crash never leaves half a registry.
	tmp := registryPath + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return "", fmt.Errorf("registry: %v", err)
	}
	if err := os.Rename(tmp, registryPath); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("registry: %v", err)
	}
	msg := fmt.Sprintf("%s is %s", agentID, want)
	if err := appendAudit(auditPath, audit.Entry{Event: event, Agent: agentID}); err != nil {
		return "", fmt.Errorf("%s in the registry, but that couldn't be recorded: %v", msg, err)
	}
	return msg, nil
}

// verifyAudit checks the chain and, when expectHead is given, that the log still ends where it did
// when that hash was written down. The bool is false when the log itself is at fault.
func verifyAudit(path, expectHead string) (report string, ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("audit: %v", err)
	}
	count, head, verr := audit.Verify(data)
	if verr != nil {
		return fmt.Sprintf("audit: FAILED: %v (%d entries before it verify)", verr, count), false, nil
	}
	report = fmt.Sprintf("audit: ok, %d entries, head %s", count, head)
	switch {
	case expectHead == "":
		report += "\naudit: the head wasn't checked. Lines cut off the end still verify; record the head somewhere else and pass it back with --expect-head."
	case !strings.EqualFold(expectHead, head):
		return report + "\naudit: FAILED: the log doesn't end at --expect-head " + expectHead + ": lines were cut off the end, or added since", false, nil
	default:
		report += "\naudit: the head matches --expect-head"
	}
	return report, true, nil
}

// gitBranch reads the checked-out branch without running git. Empty when dir isn't a plain checkout.
func gitBranch(dir string) string {
	head, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return ref
}

// hookDeny is the PreToolUse answer that blocks a call and shows Claude the reason.
func hookDeny(reason string) any {
	return map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": "ascc: " + reason,
	}}
}
