package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
	"github.com/santhosh-tekuri/jsonschema/v6"
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

// The committed fixture (examples/) plants an agent with no owner. The document must validate against
// the vendored suite schema and carry that finding; CI runs this test as its own step.
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
			if err := json.Unmarshal([]byte(stdout), &inv); err != nil || len(inv.Identities) != 8 || inv.Producer.Name != "ascc" {
				t.Fatalf("bad inventory (%v): %s", err, stdout)
			}
			var doc any
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatal(err)
			}
			c := jsonschema.NewCompiler()
			c.AssertFormat()
			sch, err := c.Compile(filepath.Join("..", "..", "..", "schemas", "agent-inventory.schema.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := sch.Validate(doc); err != nil {
				t.Fatalf("the inventory doesn't validate against the suite schema: %v", err)
			}
			if !strings.Contains(stdout, `"id": "ascc/no-owner:ownerless-agent"`) {
				t.Fatal("the planted ownerless agent has no finding")
			}
		} else if stdout != "" {
			t.Fatalf("--out still wrote to stdout: %s", stdout)
		}
	}
}

// Spec criterion 16 through the CLI: --import adds another suite document's identities, once each.
func TestInventoryImportCommand(t *testing.T) {
	none := t.TempDir()
	hermetic := append([]string{"--audit", filepath.Join(none, "audit.jsonl"), "--managed", filepath.Join(none, "managed-settings.json"),
		"--user", filepath.Join(none, "settings.json"), "--project", none}, sample...)
	write := func(name, content string) string {
		file := filepath.Join(none, name)
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	first := write("ghost-grant.json", `{"schemaVersion":"0.1","generatedAt":"2026-11-01T00:00:00Z","producer":{"name":"ghost-grant","version":"1"},"identities":[
		{"id":"slack:bot/standup","kind":"bot","platform":"slack","displayName":"standup","status":"active","owner":null},
		{"id":"github:app/release-bot","kind":"bot","platform":"github","displayName":"IMPORTED-COPY","status":"unknown"}]}`)
	second := write("mcp-frisk.json", `{"schemaVersion":"0.1","generatedAt":"2026-11-01T00:00:00Z","producer":{"name":"mcp-frisk","version":"1"},"identities":[
		{"id":"slack:bot/standup","kind":"bot","platform":"slack","displayName":"SECOND-COPY","status":"active"},
		{"id":"mcp:server/tickets","kind":"mcp-server","platform":"mcp","displayName":"tickets","status":"active","owner":{"team":"support"}}]}`)

	code, out, errOut := ascc(t, "", append([]string{"inventory", "--import", first, "--import", second}, hermetic...)...)
	var inv pdp.Inventory
	if err := json.Unmarshal([]byte(out), &inv); code != 0 || err != nil || len(inv.Identities) != 10 {
		t.Fatalf("exit %d, %d identities, stderr %q", code, len(inv.Identities), errOut)
	}
	for _, want := range []string{`"imported-from:ghost-grant"`, `"imported-from:mcp-frisk"`, `"ascc/nhi-no-owner:slack:bot/standup"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the document lacks %s", want)
		}
	}
	if strings.Contains(out, "IMPORTED-COPY") || strings.Contains(out, "SECOND-COPY") {
		t.Error("an imported identity replaced or doubled one ascc already had")
	}
	// discover's own report can be merged back in.
	found := filepath.Join(none, "found.json")
	if code, _, errOut := ascc(t, "", "discover", "--registry", sample[1], "--root", discoverFixture(t).root, "--out", found); code != 0 {
		t.Fatal(errOut)
	}
	if code, out, errOut := ascc(t, "", append([]string{"inventory", "--import", found}, hermetic...)...); code != 0 || !strings.Contains(out, `"local:mcp-server/paste"`) {
		t.Fatalf("importing a discover report: exit %d, %q", code, errOut)
	}

	bad := map[string]string{
		"a missing file":    filepath.Join(none, "none.json"),
		"a folder":          none,
		"not JSON":          write("bad.json", `{"schemaVersion":"0.1","identities":[{"id":"sk-planted-not-a-real-key`),
		"another version":   write("v9.json", `{"schemaVersion":"9.9","identities":[]}`),
		"a broken identity": write("kind.json", `{"schemaVersion":"0.1","identities":[{"id":"x:1","kind":"sk-planted-not-a-real-key","platform":"slack","status":"active"}]}`),
		"a pasted secret":   write("fp.json", `{"schemaVersion":"0.1","identities":[{"id":"x:1","kind":"bot","platform":"slack","status":"active","credentials":[{"id":"k","type":"api-key","fingerprint":"sk-planted-not-a-real-key+/=="}]}]}`),
	}
	for name, file := range bad {
		// A good import beside the bad one doesn't rescue the run: nothing is printed.
		code, out, errOut := ascc(t, "", append([]string{"inventory", "--import", first, "--import", file}, hermetic...)...)
		if code != 2 || out != "" || !strings.Contains(errOut, "--import") || strings.Count(errOut, "\n") != 1 || strings.Contains(errOut, "sk-planted") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, out, errOut)
		}
	}
	if code, out, _ := ascc(t, "", append([]string{"inventory", first}, hermetic...)...); code != 2 || out != "" {
		t.Errorf("a stray argument: exit %d", code)
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
