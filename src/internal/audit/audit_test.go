package audit

import (
	"bytes"
	"strings"
	"testing"
)

func chain(t *testing.T, n int) (log []byte, head string) {
	t.Helper()
	seq := 0
	for i := 0; i < n; i++ {
		line, hash, err := Seal(seq, head, Entry{Time: "2026-11-09T12:00:00Z", Event: EventDecision, Agent: "dev-agent",
			Tool: "claude-code/Bash", Decision: "deny", PolicyIDs: []string{"p"}, Features: map[string]any{"network": true}})
		if err != nil {
			t.Fatal(err)
		}
		log, head, seq = append(log, line...), hash, seq+1
	}
	return log, head
}

func TestVerifyAcceptsAnIntactChain(t *testing.T) {
	log, head := chain(t, 5)
	count, got, err := Verify(log)
	if err != nil || count != 5 || got != head {
		t.Fatalf("count %d head %q err %v", count, got, err)
	}
	if seq, hash, err := Last(log); err != nil || seq != 5 || hash != head {
		t.Fatalf("Last: %d %q %v", seq, hash, err)
	}
	if count, head, err := Verify(nil); err != nil || count != 0 || head != "" {
		t.Fatalf("empty log: %d %q %v", count, head, err)
	}
}

// Spec criterion 4: an edited, deleted or reordered line fails, and the error names the line.
func TestVerifyNamesTheFirstBadLine(t *testing.T) {
	log, _ := chain(t, 5)
	lines := bytes.SplitAfter(log, []byte("\n"))[:5]
	join := func(ls ...[]byte) []byte { return bytes.Join(ls, nil) }

	tests := []struct {
		name string
		log  []byte
		want string
	}{
		{"edited", join(lines[0], lines[1], bytes.Replace(lines[2], []byte(`"deny"`), []byte(`"allow"`), 1), lines[3], lines[4]), "line 3 was edited"},
		{"spacing edited", join(lines[0], bytes.Replace(lines[1], []byte(`,"time"`), []byte(`, "time"`), 1), lines[2], lines[3], lines[4]), "line 2 was edited"},
		{"field added", join(bytes.Replace(lines[0], []byte(`{"seq"`), []byte(`{"extra":1,"seq"`), 1), lines[1]), "line 1 isn't a valid audit entry"},
		{"removed from the middle", join(lines[0], lines[1], lines[3], lines[4]), "line 3 doesn't follow line 2"},
		{"first line removed", join(lines[1], lines[2]), "line 1 doesn't follow line 0"},
		{"reordered", join(lines[0], lines[2], lines[1], lines[3], lines[4]), "line 2 doesn't follow line 1"},
		{"garbage", join(lines[0], []byte("not json\n"), lines[1]), "line 2 isn't a valid audit entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Verify(tt.log); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

// Cutting lines off the end leaves a valid, shorter chain. Only the head hash gives it away.
func TestTruncationChangesTheHead(t *testing.T) {
	log, head := chain(t, 5)
	cut := log[:bytes.LastIndexByte(bytes.TrimRight(log, "\n"), '\n')+1]
	count, got, err := Verify(cut)
	if err != nil || count != 4 || got == head {
		t.Fatalf("count %d, head changed %v, err %v", count, got != head, err)
	}
}

func TestLastRefusesABrokenTail(t *testing.T) {
	log, _ := chain(t, 2)
	if _, _, err := Last(append(log, []byte("half a li")...)); err == nil {
		t.Fatal("want an error for an unparsable last line")
	}
}
