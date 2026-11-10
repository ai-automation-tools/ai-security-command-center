package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// hookSettings is a Claude Code settings file that registers the ascc hook for each agent, the way
// the setup guide shows it. extra is spliced in at the top level; matcher and onFailure are spliced
// as written, so "" leaves the key out.
func hookSettings(extra, matcher, onFailure string, agents ...string) string {
	var handlers []string
	for _, a := range agents {
		handlers = append(handlers, `{"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "`+a+`"], "timeout": 10`+onFailure+`}`)
	}
	return `{` + extra + `"hooks": {"PreToolUse": [{` + matcher + `"hooks": [` + strings.Join(handlers, ",") + `]}]}}`
}

const (
	matchAll = `"matcher": "*",`
	block    = `, "onFailure": "block"`
)

// machine is a fixture tree standing in for one machine's Claude Code settings: a managed settings
// folder, a home folder and a project. Empty content means the file isn't there.
type machine struct {
	t    *testing.T
	root string
	args []string // --managed, --user, --project
}

func newMachine(t *testing.T, managed, user string) *machine {
	t.Helper()
	root := t.TempDir()
	m := &machine{t: t, root: root, args: []string{
		"--managed", filepath.Join(root, "managed", "managed-settings.json"),
		"--user", filepath.Join(root, "home", ".claude", "settings.json"),
		"--project", filepath.Join(root, "project"),
	}}
	m.write(filepath.Join("managed", "managed-settings.json"), managed)
	m.write(filepath.Join("home", ".claude", "settings.json"), user)
	return m
}

func (m *machine) write(rel, content string) {
	m.t.Helper()
	if content == "" {
		return
	}
	path := filepath.Join(m.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		m.t.Fatal(err)
	}
}

// snapshot is every file under the fixture tree with its content, to show posture writes nothing.
func (m *machine) snapshot() string {
	m.t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(m.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		b.WriteString(path + "\n" + string(data) + "\n")
		return err
	})
	if err != nil {
		m.t.Fatal(err)
	}
	return b.String()
}

// asccOnPath puts a stand-in ascc executable on PATH, and nothing else, for the missing-binary check.
func asccOnPath(t *testing.T, present bool) {
	t.Helper()
	bin := t.TempDir()
	if present {
		name := "ascc"
		if runtime.GOOS == "windows" {
			name = "ascc.exe"
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte("stand-in"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

// Spec criterion 14: a pass with the hook in managed settings, matcher * and onFailure block, and a
// fail naming the reason for each way it can fall short. posture never writes a settings file.
func TestPostureCommand(t *testing.T) {
	registry := []string{"--registry", filepath.Join(examples, "ascc.registry.json"), "--agent", "dev-agent"}
	good := hookSettings("", matchAll, block, "dev-agent")
	tests := []struct {
		name, managed, user, dropIn string
		noBinary                    bool
		want                        string // a piece of the reason; empty means a pass
	}{
		{name: "hook in managed settings", managed: good},
		{name: "hook in managed settings, and disableAllHooks in user settings", managed: good, user: `{"disableAllHooks": true}`},
		{name: "hook only in user settings", user: good, want: "only in user settings"},
		{name: "onFailure missing", managed: hookSettings("", matchAll, "", "dev-agent"), want: `"onFailure" isn't "block"`},
		{name: "disableAllHooks set", managed: hookSettings(`"disableAllHooks": true,`, matchAll, block, "dev-agent"), want: "disableAllHooks is set"},
		{name: "disableAllHooks set by a drop-in", managed: good, dropIn: `{"disableAllHooks": true}`, want: "disableAllHooks is set"},
		{name: "a narrower matcher", managed: hookSettings("", `"matcher": "Bash",`, block, "dev-agent"), want: `matcher is "Bash"`},
		{name: "a missing binary", managed: good, noBinary: true, want: "doesn't resolve to an executable"},
		{name: "managed settings that aren't JSON", managed: `{"hooks": `, want: "can't be read as a JSON object"},
		{name: "no hook anywhere", want: "no ascc hook for agent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			asccOnPath(t, !tc.noBinary)
			m := newMachine(t, tc.managed, tc.user)
			m.write(filepath.Join("managed", "managed-settings.d", "10-extra.json"), tc.dropIn)
			m.write(filepath.Join("managed", "managed-settings.d", ".hidden.json"), "not json, and Claude Code ignores hidden files")
			before := m.snapshot()

			code, out, errOut := ascc(t, "", append(append([]string{"posture", "claude-code"}, registry...), m.args...)...)
			switch {
			case errOut != "":
				t.Fatalf("stderr: %s", errOut)
			case tc.want == "" && (code != 0 || !strings.Contains(out, "posture: dev-agent: PASS\n")):
				t.Fatalf("want a pass, got exit %d:\n%s", code, out)
			case tc.want != "" && (code != 1 || !strings.Contains(out, "posture: dev-agent: FAIL\n") || !strings.Contains(out, tc.want)):
				t.Fatalf("want exit 1 with %q, got exit %d:\n%s", tc.want, code, out)
			}
			if !strings.Contains(out, "server-managed or MDM") {
				t.Errorf("the output doesn't say what the check can't see:\n%s", out)
			}
			if m.snapshot() != before {
				t.Error("posture changed a settings file")
			}
		})
	}
}

// With no --agent, every Claude Code agent in the registry is checked, and one failure is exit 1.
func TestPostureChecksEveryAgent(t *testing.T) {
	asccOnPath(t, true)
	m := newMachine(t, hookSettings("", matchAll, block, "dev-agent", "ci-agent"), "")
	args := append([]string{"posture", "claude-code", "--registry", filepath.Join(examples, "ascc.registry.json")}, m.args...)
	code, out, _ := ascc(t, "", args...)
	for _, want := range []string{"ci-agent: PASS", "dev-agent: PASS", "old-agent: FAIL", "ownerless-agent: FAIL"} {
		if !strings.Contains(out, "posture: "+want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

// When ascc can't run the check it says so on one line and exits 2, never 0.
func TestPostureFailsClosed(t *testing.T) {
	m := newMachine(t, hookSettings("", matchAll, block, "dev-agent"), "")
	sampleRegistry := filepath.Join(examples, "ascc.registry.json")
	tests := map[string][]string{
		"no runtime named":   {"posture", "--registry", sampleRegistry},
		"another runtime":    {"posture", "other-runtime", "--registry", sampleRegistry},
		"missing registry":   {"posture", "claude-code", "--registry", filepath.Join(m.root, "none.json")},
		"agent not in it":    {"posture", "claude-code", "--registry", sampleRegistry, "--agent", "ghost-agent"},
		"unknown flag":       {"posture", "claude-code", "--registry", sampleRegistry, "--fix"},
		"registry isn't one": {"posture", "claude-code", "--registry", filepath.Join(m.root, "managed", "managed-settings.json")},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := ascc(t, "", append(args, m.args...)...)
			if code != 2 || strings.Contains(out, "PASS") || errOut == "" {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
}

// Spec criterion 14, the label half, end to end: unmonitored with no audit lines for the agent,
// observed with audit lines but no passing posture check, enforced only with both.
func TestInventoryLabelsComeFromEvidence(t *testing.T) {
	asccOnPath(t, true)
	w := newWorkspace(t)
	enforcing := newMachine(t, hookSettings("", matchAll, block, "dev-agent", "ci-agent"), "")
	userOnly := newMachine(t, "", hookSettings("", matchAll, block, "dev-agent", "ci-agent"))

	labels := func(m *machine) (map[string]string, string) {
		t.Helper()
		code, out, errOut := w.run("", append([]string{"inventory"}, m.args...)...)
		var inv pdp.Inventory
		if err := json.Unmarshal([]byte(out), &inv); code != 0 || err != nil {
			t.Fatalf("exit %d, %v: %s", code, err, errOut)
		}
		got := map[string]string{}
		for _, id := range inv.Identities[:4] { // the agents; the identities they use have no coverage label
			got[id.DisplayName] = strings.TrimPrefix(id.Tags[0], "coverage:")
		}
		return got, errOut
	}
	check := func(when string, m *machine, dev, ci string) {
		t.Helper()
		got, _ := labels(m)
		if got["dev-agent"] != dev || got["ci-agent"] != ci || got["old-agent"] != pdp.CoverageUnmonitored {
			t.Errorf("%s: dev-agent %s (want %s), ci-agent %s (want %s), old-agent %s", when, got["dev-agent"], dev, got["ci-agent"], ci, got["old-agent"])
		}
	}

	// A passing posture check alone proves nothing: the hook hasn't recorded a call yet.
	check("no audit log", enforcing, pdp.CoverageUnmonitored, pdp.CoverageUnmonitored)

	if code, _, errOut := w.hook("dev-agent", "Read", map[string]any{"file_path": filepath.Join(w.dir, "README.md")}); code != 0 {
		t.Fatalf("hook exit %d: %s", code, errOut)
	}
	check("audit lines and a passing posture check", enforcing, pdp.CoverageEnforced, pdp.CoverageUnmonitored)
	check("audit lines, hook only in user settings", userOnly, pdp.CoverageObserved, pdp.CoverageUnmonitored)

	// kill and revive are written by a person, not the hook: they aren't evidence that a hook runs.
	if code, _, errOut := w.run("", "kill", "ci-agent"); code != 0 {
		t.Fatalf("kill: %s", errOut)
	}
	check("a kill line for ci-agent", enforcing, pdp.CoverageEnforced, pdp.CoverageUnmonitored)

	// A log that doesn't verify is evidence of nothing.
	logPath := filepath.Join(w.dir, ".ascc", "audit.jsonl")
	if err := os.WriteFile(logPath, []byte(strings.Replace(w.log(), `"allow"`, `"deny"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, errOut := labels(enforcing)
	if got["dev-agent"] != pdp.CoverageUnmonitored || !strings.Contains(errOut, "the audit log isn't evidence") || strings.Count(errOut, "\n") != 1 {
		t.Errorf("tampered log: dev-agent %s, stderr %q", got["dev-agent"], errOut)
	}
}
