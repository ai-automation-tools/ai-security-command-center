package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// A fixture config tree: one governed project, and one with a planted unregistered MCP server whose
// config holds a credential. All of it is fictional.
func discoverFixture(t *testing.T) *machine {
	t.Helper()
	m := &machine{t: t, root: t.TempDir()}
	m.write(filepath.Join("governed", ".claude", "settings.json"), hookSettings("", matchAll, block, "dev-agent"))
	m.write(filepath.Join("shadow", ".claude", "settings.json"), `{"permissions": {"allow": ["Bash"]}}`)
	m.write(filepath.Join("shadow", ".mcp.json"), `{"mcpServers": {"paste": {"command": "npx", "args": ["paste-mcp", "--key=planted-arg-secret"],
		"env": {"PASTE_API_KEY": "sk-planted-not-a-real-key", "LOG_LEVEL": "debug"}}}}`)
	// Never searched: a dependency's own config isn't this machine's agent setup.
	m.write(filepath.Join("shadow", "node_modules", "pkg", ".mcp.json"), `{"mcpServers": {"vendored": {}}}`)
	m.write(filepath.Join("shadow", "notes.json"), `{"mcpServers": {"not-a-config-file": {}}}`)
	return m
}

// Spec criterion 15, through the CLI: the unregistered MCP server and the credential-looking
// variable name are in the document, no value is, and the document fits the suite schema.
func TestDiscoverCommand(t *testing.T) {
	m := discoverFixture(t)
	before := m.snapshot()
	registry := filepath.Join(examples, "ascc.registry.json")
	code, out, errOut := ascc(t, "", "discover", "--registry", registry, "--root", m.root)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{`"local:mcp-server/paste"`, `"local:credential/paste/PASTE_API_KEY"`, `"local:claude-code-setup/shadow"`, `"ascc/unregistered"`, `"coverage:unmonitored"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the document lacks %s", want)
		}
	}
	for _, not := range []string{"sk-planted-not-a-real-key", "planted-arg-secret", "paste-mcp", "debug", "vendored", "not-a-config-file",
		"claude-code-setup/governed", filepath.ToSlash(m.root), m.root} {
		if strings.Contains(out, not) {
			t.Errorf("the document holds %q", not)
		}
	}

	var doc any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	sch, err := c.Compile(filepath.Join("..", "..", "..", "schemas", "agent-inventory.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(doc); err != nil {
		t.Fatalf("the document doesn't validate against the suite schema: %v", err)
	}

	// --out writes the same document to a file and nothing to stdout; discover changes no config.
	file := filepath.Join(t.TempDir(), "discovered.json")
	if code, stdout, _ := ascc(t, "", "discover", "--registry", registry, "--root", m.root, "--out", file); code != 0 || stdout != "" {
		t.Fatalf("--out: exit %d, stdout %q", code, stdout)
	}
	if written, err := os.ReadFile(file); err != nil || !strings.Contains(string(written), `"local:mcp-server/paste"`) {
		t.Fatalf("--out file: %v", err)
	}
	if m.snapshot() != before {
		t.Error("discover changed a config file")
	}
}

// A config file ascc can't read is a hole in the report. It still prints what it found, names the
// file on stderr without quoting it, and exits 1.
func TestDiscoverSaysWhatItCouldntRead(t *testing.T) {
	m := discoverFixture(t)
	m.write(filepath.Join("broken", ".mcp.json"), `{"mcpServers": {"hidden": {"env": {"API_KEY": "sk-planted-not-a-real-key`)
	code, out, errOut := ascc(t, "", "discover", "--registry", filepath.Join(examples, "ascc.registry.json"), "--root", m.root)
	if code != 1 || !strings.Contains(out, `"local:mcp-server/paste"`) {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(errOut, "broken/.mcp.json isn't a JSON object") || strings.Count(errOut, "\n") != 1 || strings.Contains(errOut+out, "sk-planted") {
		t.Fatalf("stderr %q", errOut)
	}
}

// A linked folder isn't followed, so it is a gap; a linked root is resolved and searched.
func TestDiscoverAndLinks(t *testing.T) {
	m := discoverFixture(t)
	registry := filepath.Join(examples, "ascc.registry.json")
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(m.root, "shadow"), filepath.Join(outside, "linked")); err != nil {
		t.Skipf("this machine can't make a symlink: %v", err)
	}
	code, out, errOut := ascc(t, "", "discover", "--registry", registry, "--root", outside)
	if code != 1 || strings.Contains(out, "paste") || !strings.Contains(errOut, "linked is a link to a folder") {
		t.Fatalf("linked folder: exit %d, stderr %q\n%s", code, errOut, out)
	}
	code, out, errOut = ascc(t, "", "discover", "--registry", registry, "--root", filepath.Join(outside, "linked"))
	if code != 0 || !strings.Contains(out, `"local:mcp-server/paste"`) {
		t.Fatalf("linked root: exit %d, stderr %q\n%s", code, errOut, out)
	}
}

// --out never lands on ascc's own state: only the store writes the registry and the audit log, and
// nothing in ascc writes an agent config file such as Claude Code's settings.
func TestOutWontOverwriteAsccState(t *testing.T) {
	w := newWorkspace(t)
	if code, _, errOut := w.run("", "kill", "old-agent"); code != 0 { // so the audit log exists
		t.Fatal(errOut)
	}
	before := w.log()
	settings := filepath.Join(w.dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{
		"the registry":  filepath.Join(w.dir, "ascc.registry.json"),
		"the audit log": filepath.Join(w.dir, ".ascc", "audit.jsonl"),
		"a policy file": filepath.Join(w.dir, "policies", "allow.cedar"),
		"a new policy":  filepath.Join(w.dir, "policies", "new.cedar"),
		// A scheduled run that replaced a settings file would take the hook's registration with it.
		"project settings":        settings,
		"local settings":          filepath.Join(w.dir, ".claude", "settings.local.json"),
		"user settings, any case": filepath.Join(w.dir, "home", ".Claude", "Settings.json"),
		"managed settings":        filepath.Join(w.dir, "managed-settings.json"),
		"a managed drop-in":       filepath.Join(w.dir, "managed-settings.d", "10-ascc.json"),
		"an MCP config":           filepath.Join(w.dir, ".mcp.json"),
	} {
		for _, cmd := range []string{"discover", "inventory"} {
			args := []string{cmd, "--out", out}
			if cmd == "discover" {
				args = append(args, "--root", w.dir)
			}
			if code, _, errOut := w.run("", args...); code != 2 || !strings.Contains(errOut, "--out would write over") {
				t.Errorf("%s --out %s: exit %d, stderr %q", cmd, name, code, errOut)
			}
		}
	}
	if w.log() != before {
		t.Error("the audit log changed")
	}
	if b, err := os.ReadFile(settings); err != nil || string(b) != "{}\n" {
		t.Errorf("the project settings changed: %q, %v", b, err)
	}
	if code, _, errOut := w.run("", "audit", "verify"); code != 0 {
		t.Errorf("audit verify: %s", errOut)
	}
}

func TestDiscoverFailsClosed(t *testing.T) {
	m := discoverFixture(t)
	registry := filepath.Join(examples, "ascc.registry.json")
	tests := map[string][]string{
		"missing registry":  {"discover", "--registry", filepath.Join(m.root, "none.json"), "--root", m.root},
		"missing root":      {"discover", "--registry", registry, "--root", filepath.Join(m.root, "none")},
		"root is a file":    {"discover", "--registry", registry, "--root", filepath.Join(m.root, "shadow", ".mcp.json")},
		"a stray argument":  {"discover", "--registry", registry, m.root},
		"an unknown flag":   {"discover", "--registry", registry, "--root", m.root, "--fix"},
		"unwritable --out":  {"discover", "--registry", registry, "--root", m.root, "--out", filepath.Join(m.root, "none", "out.json")},
		"registry isn't it": {"discover", "--registry", filepath.Join(m.root, "shadow", ".mcp.json"), "--root", m.root},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := ascc(t, "", args...)
			if code != 2 || out != "" || errOut == "" || strings.Contains(errOut, "sk-planted") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
}
