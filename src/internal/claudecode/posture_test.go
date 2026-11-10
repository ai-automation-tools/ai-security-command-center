package claudecode

import (
	"strings"
	"testing"
)

// settingsFile builds a settings file holding one PreToolUse group. group and handler are JSON fragments
// that replace the defaults when non-empty.
func settingsFile(extra, group, handler string) []byte {
	if group == "" {
		group = `"matcher": "*",`
	}
	if handler == "" {
		handler = `"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "dev-agent"], "timeout": 10, "onFailure": "block"`
	}
	return []byte(`{` + extra + `"hooks": {"PreToolUse": [{` + group + `"hooks": [{` + handler + `}]}]}}`)
}

const execForm = `"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "dev-agent"]`

// Spec criterion 14, and every other way the registration can stop short of enforcing.
func TestCheckPosture(t *testing.T) {
	good := settingsFile("", "", "")
	found := func(string) bool { return true }
	tests := []struct {
		name     string
		settings Settings
		resolves func(string) bool
		want     string // a piece of the reason; empty means the check passes
	}{
		{name: "managed, matcher *, onFailure block", settings: Settings{Managed: [][]byte{good}}},
		{name: "no matcher matches every tool", settings: Settings{Managed: [][]byte{settingsFile("", `"unused": 1,`, "")}}},
		{name: "empty matcher matches every tool", settings: Settings{Managed: [][]byte{settingsFile("", `"matcher": "",`, "")}}},
		{name: "hook in a managed drop-in file", settings: Settings{Managed: [][]byte{[]byte(`{}`), good}}},
		{name: "full path to the binary", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "/opt/ascc/bin/ascc.exe", "args": ["hook", "claude-code", "--agent=dev-agent"], "onFailure": "block"`)}}},
		// Per the hooks reference, only a managed disableAllHooks switches managed hooks off.
		{name: "disableAllHooks in user settings can't touch a managed hook",
			settings: Settings{Managed: [][]byte{good}, User: []byte(`{"disableAllHooks": true}`)}},
		{name: "a broken user file can't touch a managed hook", settings: Settings{Managed: [][]byte{good}, User: []byte(`{`)}},

		{name: "hook only in user settings", settings: Settings{User: good}, want: "only in user settings"},
		{name: "hook only in project and local settings", settings: Settings{Project: good, Local: good}, want: "only in project and local settings"},
		{name: "onFailure missing", settings: Settings{Managed: [][]byte{settingsFile("", "", execForm)}}, want: `"onFailure" isn't "block"`},
		{name: "onFailure continue", settings: Settings{Managed: [][]byte{settingsFile("", "", execForm+`, "onFailure": "continue"`)}}, want: `"onFailure" isn't "block"`},
		{name: "onFailure in the wrong case", settings: Settings{Managed: [][]byte{settingsFile("", "", execForm+`, "onfailure": "block"`)}}, want: `"onFailure" isn't "block"`},
		{name: "disableAllHooks set", settings: Settings{Managed: [][]byte{settingsFile(`"disableAllHooks": true,`, "", "")}}, want: "disableAllHooks is set"},
		{name: "disableAllHooks set in a drop-in", settings: Settings{Managed: [][]byte{good, []byte(`{"disableAllHooks": true}`)}}, want: "disableAllHooks is set"},
		{name: "a narrower matcher", settings: Settings{Managed: [][]byte{settingsFile("", `"matcher": "Bash",`, "")}}, want: `matcher is "Bash"`},
		{name: "a regex matcher", settings: Settings{Managed: [][]byte{settingsFile("", `"matcher": ".*",`, "")}}, want: `matcher is ".*"`},
		{name: "a missing binary", settings: Settings{Managed: [][]byte{good}}, resolves: func(string) bool { return false }, want: "doesn't resolve to an executable"},
		{name: "shell form", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc hook claude-code --agent dev-agent || true", "onFailure": "block"`)}}, want: "shell form"},
		{name: "async", settings: Settings{Managed: [][]byte{settingsFile("", "", execForm+`, "onFailure": "block", "async": true`)}}, want: "background"},
		{name: "an if filter", settings: Settings{Managed: [][]byte{settingsFile("", "", execForm+`, "onFailure": "block", "if": "Bash(git *)"`)}}, want: `"if" filter`},
		{name: "not a command hook", settings: Settings{Managed: [][]byte{settingsFile("", "", strings.Replace(execForm, `"type": "command"`, `"type": "http"`, 1)+`, "onFailure": "block"`)}}, want: `type isn't "command"`},
		{name: "the hook speaks for another agent", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "ci-agent"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		{name: "the last --agent wins, as it does in the hook", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "dev-agent", "--agent", "ci-agent"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		// Go's flag parsing stops at "x", so the real hook would refuse these arguments.
		{name: "a stray argument before --agent", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "ci-agent", "x", "--agent", "dev-agent"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		{name: "a value flag swallows --agent", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--registry", "--agent", "dev-agent"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		{name: "a flag the hook doesn't have", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "dev-agent", "--dry-run"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		{name: "another program's hook", settings: Settings{Managed: [][]byte{settingsFile("", "",
			`"type": "command", "command": "other", "args": ["hook", "claude-code", "--agent", "dev-agent"], "onFailure": "block"`)}}, want: "no ascc hook for agent"},
		{name: "hooks key in the wrong case", settings: Settings{Managed: [][]byte{[]byte(strings.Replace(string(good), `"hooks": {"PreToolUse"`, `"Hooks": {"PreToolUse"`, 1))}}, want: "no ascc hook for agent"},
		{name: "a PostToolUse hook", settings: Settings{Managed: [][]byte{[]byte(strings.Replace(string(good), "PreToolUse", "PostToolUse", 1))}}, want: "no ascc hook for agent"},
		{name: "no settings at all", settings: Settings{}, want: "no ascc hook for agent"},
		{name: "managed settings aren't JSON", settings: Settings{Managed: [][]byte{[]byte(`{"hooks": `)}}, want: "can't be read as a JSON object"},
		{name: "managed settings are a JSON array", settings: Settings{Managed: [][]byte{[]byte(`[]`)}}, want: "can't be read as a JSON object"},
		{name: "one bad managed file fails a good one", settings: Settings{Managed: [][]byte{good, []byte(`not json`)}}, want: "can't be read as a JSON object"},
		{name: "hooks of the wrong shape", settings: Settings{Managed: [][]byte{[]byte(`{"hooks": {"PreToolUse": "ascc hook claude-code --agent dev-agent"}}`)}}, want: "no ascc hook for agent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolves := tc.resolves
			if resolves == nil {
				resolves = found
			}
			p := CheckPosture(tc.settings, "dev-agent", resolves)
			reasons := strings.Join(p.Reasons, "; ")
			switch {
			case p.Agent != "dev-agent":
				t.Fatalf("agent %q", p.Agent)
			case tc.want == "" && (!p.Pass || len(p.Reasons) != 0):
				t.Fatalf("want a pass, got: %s", reasons)
			case tc.want != "" && (p.Pass || !strings.Contains(reasons, tc.want)):
				t.Fatalf("want a fail with %q, got pass=%v: %s", tc.want, p.Pass, reasons)
			}
		})
	}
}

// With two managed registrations, one sound one is enough; with none sound, the nearest miss is named.
func TestCheckPostureSeveralRegistrations(t *testing.T) {
	two := []byte(`{"hooks": {"PreToolUse": [
		{"matcher": "Bash", "hooks": [{` + execForm + `}]},
		{"matcher": "*", "hooks": [{` + execForm + `, "onFailure": "block"}]}]}}`)
	if p := CheckPosture(Settings{Managed: [][]byte{two}}, "dev-agent", func(string) bool { return true }); !p.Pass {
		t.Fatalf("want a pass: %v", p.Reasons)
	}
	near := []byte(`{"hooks": {"PreToolUse": [
		{"matcher": "Bash", "hooks": [{` + execForm + `}]},
		{"matcher": "Bash", "hooks": [{` + execForm + `, "onFailure": "block"}]}]}}`)
	p := CheckPosture(Settings{Managed: [][]byte{near}}, "dev-agent", func(string) bool { return true })
	if p.Pass || len(p.Reasons) != 1 || !strings.Contains(p.Reasons[0], "matcher") {
		t.Fatalf("pass=%v reasons %v", p.Pass, p.Reasons)
	}
}

// A settings file can hold tokens in its env block. No reason may repeat anything but the matcher.
func TestCheckPostureEchoesNoSettingsValue(t *testing.T) {
	const secret = "sk-planted-not-a-real-key"
	file := []byte(`{"env": {"API_TOKEN": "` + secret + `"}, "disableAllHooks": "` + secret + `", "hooks": {"PreToolUse": [{"matcher": "Bash",
		"hooks": [{"type": "` + secret + `", "command": "ascc", "args": ["hook", "claude-code", "--agent", "dev-agent", "--audit", "` + secret + `"],
		"onFailure": "` + secret + `", "if": "` + secret + `"}]}]}}`)
	p := CheckPosture(Settings{Managed: [][]byte{file}}, "dev-agent", func(string) bool { return false })
	if p.Pass || len(p.Reasons) < 5 {
		t.Fatalf("pass=%v reasons %v", p.Pass, p.Reasons)
	}
	if strings.Contains(strings.Join(p.Reasons, "\n"), secret) {
		t.Fatalf("a reason repeats a settings value: %v", p.Reasons)
	}
}
