package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

var (
	examples = filepath.Join("..", "..", "..", "examples")
	sample   = []string{"--registry", filepath.Join(examples, "ascc.registry.json"), "--policies", filepath.Join(examples, "policies")}
)

func ascc(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func decision(t *testing.T, stdout string) pdp.Decision {
	t.Helper()
	var d pdp.Decision
	if err := json.Unmarshal([]byte(stdout), &d); err != nil {
		t.Fatalf("stdout isn't a decision: %v\n%s", err, stdout)
	}
	return d
}

// Spec criterion 1, through the CLI and the committed sample files.
func TestDecideOnTheSamples(t *testing.T) {
	for file, want := range map[string]string{"read-repo-file.json": "allow", "exfil-curl.json": "deny", "push-to-main.json": "deny"} {
		args := append([]string{"decide", "--request", filepath.Join(examples, "requests", file)}, sample...)
		code, out, _ := ascc(t, "", args...)
		if d := decision(t, out); code != 0 || d.Decision != want || len(d.PolicyIDs) != 1 || d.Reason == "" {
			t.Errorf("%s: exit %d, %+v, want %s", file, code, d, want)
		}
	}
}

func TestDecideReadsStdin(t *testing.T) {
	req := `{"principal":"ghost-agent","action":"runtime.tool_use","resource":"claude-code/Read","context":{}}`
	code, out, _ := ascc(t, req, append([]string{"decide", "--request", "-"}, sample...)...)
	if d := decision(t, out); code != 0 || d.Decision != "deny" || !strings.Contains(d.Reason, "not in the registry") {
		t.Fatalf("exit %d, %+v", code, d)
	}
}

// When ascc can't run, it still prints a deny, says why on stderr in one line, and exits 2.
func TestDecideFailsClosed(t *testing.T) {
	empty := t.TempDir()
	tests := map[string][]string{
		"missing registry":   {"decide", "--request", "-", "--registry", filepath.Join(empty, "none.json"), "--policies", filepath.Join(examples, "policies")},
		"no policy files":    {"decide", "--request", "-", "--registry", filepath.Join(examples, "ascc.registry.json"), "--policies", empty},
		"no --request":       append([]string{"decide"}, sample...),
		"missing request":    append([]string{"decide", "--request", filepath.Join(empty, "none.json")}, sample...),
		"request isn't JSON": append([]string{"decide", "--request", "-"}, sample...),
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := ascc(t, "not json", args...)
			d := decision(t, out)
			if code != 2 || d.Decision != "deny" || d.Reason == "" || strings.Count(errOut, "\n") != 1 {
				t.Fatalf("exit %d, %+v, stderr %q", code, d, errOut)
			}
		})
	}
}

func TestInventoryCommand(t *testing.T) {
	out := filepath.Join(t.TempDir(), "inventory.json")
	// Point every evidence source at an empty folder, so the test never reads this machine's settings.
	none := t.TempDir()
	hermetic := append([]string{"--audit", filepath.Join(none, "audit.jsonl"), "--managed", filepath.Join(none, "managed-settings.json"),
		"--user", filepath.Join(none, "settings.json"), "--project", none}, sample...)
	for _, args := range [][]string{append([]string{"inventory"}, hermetic...), append([]string{"inventory", "--out", out}, hermetic...)} {
		code, stdout, errOut := ascc(t, "", args...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if args[1] != "--out" { // the document is on stdout
			var inv pdp.Inventory
			if err := json.Unmarshal([]byte(stdout), &inv); err != nil || len(inv.Identities) != 4 || inv.Producer.Name != "ascc" {
				t.Fatalf("bad inventory (%v): %s", err, stdout)
			}
		} else if stdout != "" {
			t.Fatalf("--out still wrote to stdout: %s", stdout)
		}
	}
}

func TestUsageAndUnknownCommand(t *testing.T) {
	if code, _, errOut := ascc(t, ""); code != 2 || !strings.Contains(errOut, "Usage:") {
		t.Fatalf("no args: exit %d, %q", code, errOut)
	}
	if code, _, errOut := ascc(t, "", "launch"); code != 2 || !strings.Contains(errOut, `unknown command "launch"`) {
		t.Fatalf("unknown command: exit %d, %q", code, errOut)
	}
	if code, out, _ := ascc(t, "", "version"); code != 0 || !strings.HasPrefix(out, "ascc ") {
		t.Fatalf("version: exit %d, %q", code, out)
	}
}
