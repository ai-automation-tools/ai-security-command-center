package claudecode

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
)

// Settings holds the bytes of the Claude Code settings files on one machine. A nil entry means the
// file isn't there.
type Settings struct {
	Managed              [][]byte // managed-settings.json, then managed-settings.d/*.json in name order
	User, Project, Local []byte   // ~/.claude/settings.json, .claude/settings.json, .claude/settings.local.json
}

// Posture is the answer for one agent: whether its hook is registered so that it enforces, and
// every reason it doesn't.
type Posture struct {
	Agent   string
	Pass    bool
	Reasons []string
}

// CheckPosture reports whether the ascc hook for agent is registered where the agent's user can't
// remove it and in a form that can't fail open: in managed settings, on every tool, in exec form,
// with "onFailure": "block", with nothing switching hooks off. resolves says whether a hook's
// command is an executable on this machine.
//
// Anything it can't read counts against the agent: a pass is a claim, and ascc doesn't claim coverage
// it can't show. The reasons never repeat a settings value other than a matcher, because a settings
// file can hold tokens.
//
// ponytail: it reads the managed settings files only. A server-managed or MDM policy replaces them
// and can't be seen from here, and neither can the permission mode or the Claude Code version
// (FR-RT-05). Callers say so next to a pass.
func CheckPosture(s Settings, agent string, resolves func(command string) bool) Posture {
	p := Posture{Agent: agent}
	var best []string // the defects of the managed registration that has the fewest
	found := false
	for _, raw := range s.Managed {
		var doc map[string]any
		if json.Unmarshal(raw, &doc) != nil || doc == nil {
			p.Reasons = append(p.Reasons, "a managed settings file can't be read as a JSON object, so ascc can't tell what it switches off")
			continue
		}
		// Claude Code drops a value that isn't a boolean; ascc doesn't guess, and counts it as set.
		if v, set := doc["disableAllHooks"]; set && v != false {
			p.Reasons = append(p.Reasons, "disableAllHooks is set in managed settings, which switches every hook off")
		}
		for _, defects := range registrations(doc, agent, resolves) {
			if !found || len(defects) < len(best) {
				best, found = defects, true
			}
		}
	}
	if found {
		p.Reasons = append(p.Reasons, best...)
	} else {
		var elsewhere []string
		for _, layer := range []struct {
			name string
			raw  []byte
		}{{"user", s.User}, {"project", s.Project}, {"local", s.Local}} {
			var doc map[string]any
			if json.Unmarshal(layer.raw, &doc) == nil && len(registrations(doc, agent, nil)) > 0 {
				elsewhere = append(elsewhere, layer.name)
			}
		}
		if len(elsewhere) > 0 {
			p.Reasons = append(p.Reasons, fmt.Sprintf("the hook for agent %q is registered only in %s settings, which the agent's user can edit or switch off; register it in managed settings",
				agent, strings.Join(elsewhere, " and ")))
		} else {
			p.Reasons = append(p.Reasons, fmt.Sprintf("no ascc hook for agent %q is registered in managed settings", agent))
		}
	}
	p.Pass = len(p.Reasons) == 0
	return p
}

// registrations finds every PreToolUse handler in one settings file that runs
// `ascc hook claude-code --agent <agent>`, and returns each one's defects (none when it enforces).
// Keys are matched exactly, as Claude Code matches them: "onfailure" is not "onFailure".
func registrations(doc map[string]any, agent string, resolves func(command string) bool) [][]string {
	var out [][]string
	asccHooks(doc, func(hookFor string, group, handler map[string]any, command string, exec bool) {
		if hookFor != agent {
			return
		}
		out = append(out, defects(agent, group, handler, command, exec, resolves))
	})
	return out
}

// asccHooks calls fn for every PreToolUse handler in one settings file that runs
// `ascc hook claude-code`, with the agent it speaks for ("" when the hook would refuse its arguments).
func asccHooks(doc map[string]any, fn func(agent string, group, handler map[string]any, command string, exec bool)) {
	hooks, _ := doc["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	for _, g := range groups {
		group, _ := g.(map[string]any)
		handlers, _ := group["hooks"].([]any)
		for _, h := range handlers {
			handler, _ := h.(map[string]any)
			command, _ := handler["command"].(string)
			args, exec := handler["args"].([]any)
			argv := []string{command}
			for _, a := range args {
				s, _ := a.(string)
				argv = append(argv, s)
			}
			if !exec { // shell form: the shell splits the string, and the command ends at its first operator
				argv = nil
				for _, tok := range strings.Fields(command) {
					if strings.ContainsAny(tok, "|&;<>") {
						break
					}
					argv = append(argv, strings.Trim(tok, `"'`))
				}
			}
			if len(argv) >= 3 && program(strings.ReplaceAll(argv[0], `\`, "/")) == "ascc" && argv[1] == "hook" && argv[2] == "claude-code" {
				fn(hookAgent(argv[3:]), group, handler, command, exec)
			}
		}
	}
}

// defects lists what stops one registration enforcing; none means it does.
func defects(agent string, group, handler map[string]any, command string, exec bool, resolves func(command string) bool) []string {
	out := []string{}
	add := func(format string, a ...any) {
		out = append(out, "the managed hook for agent \""+agent+"\": "+fmt.Sprintf(format, a...))
	}
	// "*", "" and no matcher at all fire on every tool; anything else is a list or a regex.
	if m, set := group["matcher"]; set && m != "*" && m != "" {
		if s, ok := m.(string); ok {
			add(`its matcher is %q, not "*", so some tools never reach it`, s)
		} else {
			add(`its matcher isn't "*", so some tools never reach it`)
		}
	}
	if handler["type"] != "command" {
		add(`its type isn't "command"`)
	}
	if !exec {
		add(`it is in shell form (no "args"), where the shell can swallow its exit code; register it with "args"`)
	}
	if handler["onFailure"] != "block" {
		add(`"onFailure" isn't "block", so a crash, a timeout or a missing binary lets the call through`)
	}
	for _, key := range []string{"async", "asyncRewake"} {
		if v, set := handler[key]; set && v != false {
			add(`"%s" runs it in the background, where it can't block a call`, key)
		}
	}
	if _, set := handler["if"]; set {
		add(`it has an "if" filter, so it doesn't run on every call`)
	}
	if exec && resolves != nil && !resolves(command) {
		add("its command doesn't resolve to an executable on this machine, so the hook can't start")
	}
	return out
}

// hookAgent reads --agent from a hook's arguments with the same parser and the same flags as
// `ascc hook claude-code` in src/cmd/ascc/main.go, so the check and the hook can't disagree about
// whom a registration speaks for. Arguments the hook would refuse speak for nobody.
func hookAgent(args []string) string {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	agent := fs.String("agent", "", "")
	for _, name := range []string{"repo", "registry", "policies", "audit"} {
		fs.String(name, "", "")
	}
	if fs.Parse(args) != nil || fs.NArg() > 0 {
		return ""
	}
	return *agent
}
