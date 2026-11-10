// Package audit is the hash-chained decision log: one JSON object per line, each carrying the hash of
// the line before it and a hash of itself.
//
// It does no file, network or clock access. Callers hand it bytes and the time, so the WebAssembly
// demo imports exactly the code the CLI runs.
package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Events an entry can record.
const (
	EventDecision = "decision"
	EventKill     = "kill"
	EventRevive   = "revive"
)

// Entry is one audit line. It never holds a tool's raw input: InputSHA256 is a hash of the hook
// payload, and Features are the parsed facts policy saw.
type Entry struct {
	Seq         int            `json:"seq"`
	Time        string         `json:"time"`
	Prev        string         `json:"prev"` // Hash of the previous line; empty on the first
	Event       string         `json:"event"`
	Agent       string         `json:"agent"`
	Tool        string         `json:"tool,omitempty"`
	Decision    string         `json:"decision,omitempty"`
	PolicyIDs   []string       `json:"policy_ids,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Features    map[string]any `json:"features,omitempty"`
	InputSHA256 string         `json:"input_sha256,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	AgentType   string         `json:"agent_type,omitempty"`
	Cwd         string         `json:"cwd,omitempty"`
	Hash        string         `json:"hash"` // SHA-256 of this entry's JSON with Hash empty
}

// SHA256 is the lowercase hex digest ascc uses everywhere.
func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Seal links e after the entry with the given seq and hash (0 and "" for an empty log), and returns
// the line to append, newline included, with the new head hash.
func Seal(prevSeq int, prevHash string, e Entry) (line []byte, hash string, err error) {
	e.Seq, e.Prev, e.Hash = prevSeq+1, prevHash, ""
	body, err := json.Marshal(e)
	if err != nil {
		return nil, "", err
	}
	e.Hash = SHA256(body)
	line, err = json.Marshal(e)
	if err != nil {
		return nil, "", err
	}
	return append(line, '\n'), e.Hash, nil
}

// Last reads the seq and hash of the final line, which is all an append needs. tail may be just the
// end of the log, as long as it holds the whole last line. An unreadable last line is an error: ascc
// won't chain onto a log it can't trust.
func Last(tail []byte) (seq int, hash string, err error) {
	tail = bytes.TrimRight(tail, "\r\n")
	if len(tail) == 0 {
		return 0, "", nil
	}
	if i := bytes.LastIndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	var e Entry
	if err := json.Unmarshal(tail, &e); err != nil || e.Seq < 1 || len(e.Hash) != 64 {
		return 0, "", errors.New("the last audit line isn't a valid entry")
	}
	return e.Seq, e.Hash, nil
}

// Verify walks the whole log and returns the entry count and head hash. The error names the first
// line that was edited, removed or reordered. A log with lines cut off the end still verifies: only
// comparing the head with one recorded elsewhere catches that.
func Verify(log []byte) (count int, head string, err error) {
	lines := bytes.Split(bytes.TrimRight(log, "\n"), []byte("\n"))
	if len(lines) == 1 && len(lines[0]) == 0 {
		return 0, "", nil
	}
	for i, raw := range lines {
		n := i + 1
		var e Entry
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&e) != nil {
			return count, head, fmt.Errorf("line %d isn't a valid audit entry", n)
		}
		stored := e.Hash
		// Re-sealing must reproduce the line byte for byte, so any edit shows, even to spacing.
		resealed, hash, err := Seal(e.Seq-1, e.Prev, e)
		if err != nil || hash != stored || !bytes.Equal(bytes.TrimRight(resealed, "\n"), raw) {
			return count, head, fmt.Errorf("line %d was edited: its hash doesn't match its content", n)
		}
		if e.Seq != n || e.Prev != head {
			return count, head, fmt.Errorf("line %d doesn't follow line %d: a line was removed or reordered", n, n-1)
		}
		count, head = n, stored
	}
	return count, head, nil
}

// DecisionAgents returns the agents the hook has recorded a tool-call decision for: the evidence
// that a hook is running for them. A line with no tool is the hook failing before it mapped a call
// (an empty payload, a registry that won't load) and shows nothing. A log that doesn't verify is
// evidence of nothing, so it returns the error.
// ponytail: anyone who can run `ascc hook` against the log can still write such a line by hand.
// The sample policies forbid the agent that; binding lines to a session needs the instance key.
func DecisionAgents(log []byte) (map[string]bool, error) {
	if _, _, err := Verify(log); err != nil {
		return nil, err
	}
	agents := map[string]bool{}
	for _, raw := range bytes.Split(log, []byte("\n")) {
		var e Entry
		if json.Unmarshal(raw, &e) == nil && e.Event == EventDecision && e.Tool != "" {
			agents[e.Agent] = true
		}
	}
	return agents, nil
}
