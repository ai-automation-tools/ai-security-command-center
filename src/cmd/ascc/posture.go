package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/audit"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/claudecode"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// postureNote goes next to every pass: a pass is only as wide as what ascc could read.
const postureNote = "posture: checked the managed settings files only. A server-managed or MDM policy replaces them, and ascc can't see one."

// settingsPaths says where the Claude Code settings layers are. ascc only ever reads them.
type settingsPaths struct{ managed, user, project *string }

func settingsFlags(fs *flag.FlagSet) settingsPaths {
	user := ""
	if home, err := os.UserHomeDir(); err == nil {
		user = filepath.Join(home, ".claude", "settings.json")
	}
	return settingsPaths{
		managed: fs.String("managed", managedSettingsPath(), "Claude Code managed settings file; managed-settings.d beside it is read too"),
		user:    fs.String("user", user, "Claude Code user settings file"),
		project: fs.String("project", ".", "project root holding .claude/settings.json"),
	}
}

// managedSettingsPath is where Claude Code reads file-based managed settings, per its documentation.
func managedSettingsPath() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\Program Files\ClaudeCode\managed-settings.json`
	case "darwin":
		return "/Library/Application Support/ClaudeCode/managed-settings.json"
	}
	return "/etc/claude-code/managed-settings.json"
}

// read loads every layer. A file that isn't there is nil. A managed file that is there but can't be
// read comes back as empty bytes, which the posture check counts against the agent.
func (sp settingsPaths) read() claudecode.Settings {
	file := func(path string) []byte {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			return data
		case errors.Is(err, fs.ErrNotExist):
			return nil
		}
		return []byte{}
	}
	var s claudecode.Settings
	if main := file(*sp.managed); main != nil {
		s.Managed = append(s.Managed, main)
	}
	// Claude Code merges managed-settings.json, then each drop-in in name order, skipping hidden files.
	dropIns := filepath.Join(filepath.Dir(*sp.managed), "managed-settings.d")
	entries, err := os.ReadDir(dropIns) // sorted by name
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.Managed = append(s.Managed, []byte{})
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), ".") {
			s.Managed = append(s.Managed, file(filepath.Join(dropIns, e.Name())))
		}
	}
	if *sp.user != "" {
		s.User = file(*sp.user)
	}
	s.Project = file(filepath.Join(*sp.project, ".claude", "settings.json"))
	s.Local = file(filepath.Join(*sp.project, ".claude", "settings.local.json"))
	return s
}

// resolves reports whether a hook's command is an executable file: at that path when it names one,
// otherwise on PATH. Claude Code starts an exec-form hook with no shell, so on Windows only an .exe counts.
// ponytail: it checks the PATH of whoever runs ascc, not of the agent's session. Hand-rolled because
// exec.LookPath lives in os/exec, which the binary may not link.
func resolves(command string) bool {
	if command == "" {
		return false
	}
	if runtime.GOOS == "windows" && filepath.Ext(command) == "" {
		command += ".exe"
	}
	executable := func(path string) bool {
		st, err := os.Stat(path)
		return err == nil && st.Mode().IsRegular() && (runtime.GOOS == "windows" || st.Mode()&0o111 != 0)
	}
	if strings.ContainsAny(command, `/\`) {
		return filepath.IsAbs(command) && executable(command)
	}
	return slices.ContainsFunc(filepath.SplitList(os.Getenv("PATH")), func(dir string) bool {
		return filepath.IsAbs(dir) && executable(filepath.Join(dir, command))
	})
}

// posture checks the hook registration of every Claude Code agent in the registry, or of one.
// It exits 0 when all pass, 1 when any fails, and 2 when ascc couldn't run.
func posture(registryPath, agent string, sp settingsPaths, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return fail(stderr, fmt.Errorf("registry: %v", err))
	}
	reg, err := pdp.ParseRegistry(data)
	if err != nil {
		return fail(stderr, err)
	}
	var agents []string
	for _, a := range reg.Agents {
		if (agent == "" && a.Runtime == "claude-code") || a.ID == agent {
			agents = append(agents, a.ID)
		}
	}
	if len(agents) == 0 {
		if agent != "" {
			return fail(stderr, fmt.Errorf("agent %q is not in the registry", agent))
		}
		return fail(stderr, errors.New("the registry has no agent with runtime claude-code"))
	}
	slices.Sort(agents)

	settings, code := sp.read(), 0
	for _, id := range agents {
		p := claudecode.CheckPosture(settings, id, resolves)
		if p.Pass {
			fmt.Fprintf(stdout, "posture: %s: PASS\n", id)
			continue
		}
		code = 1
		fmt.Fprintf(stdout, "posture: %s: FAIL\n", id)
		for _, reason := range p.Reasons {
			fmt.Fprintln(stdout, "  - "+reason)
		}
	}
	fmt.Fprintln(stdout, postureNote)
	return code
}

// evidence gathers what ascc can show about each agent for the coverage label: which agents the hook
// has recorded decisions for, and for those, whether the posture check passes right now. Anything it
// can't read is no evidence, said once on stderr; it never raises a label. The log is read without
// the append lock, as `audit verify` reads it: a line caught half-written fails to verify, which
// only lowers the labels for that run.
func evidence(auditPath string, sp settingsPaths, stderr io.Writer) pdp.Evidence {
	ev := pdp.Evidence{PosturePass: map[string]bool{}}
	log, err := os.ReadFile(auditPath)
	if errors.Is(err, fs.ErrNotExist) {
		return ev
	}
	if err == nil {
		ev.Audited, err = audit.DecisionAgents(log)
	}
	if err != nil {
		fmt.Fprintf(stderr, "ascc: the audit log isn't evidence, so every agent is unmonitored: %v\n", err)
		return ev
	}
	settings := sp.read()
	for id := range ev.Audited {
		ev.PosturePass[id] = claudecode.CheckPosture(settings, id, resolves).Pass
	}
	return ev
}
