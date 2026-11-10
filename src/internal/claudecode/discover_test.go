package claudecode

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

const discoverRegistry = `{"agents": [{"id": "dev-agent", "owner": "platform-team", "runtime": "claude-code", "risk_tier": 1, "status": "active"}],
	"tools": {"claude-code/Read": [], "claude-code/mcp__tickets__search": [], "claude-code/mcp__my_docs__get": []}}`

// Every value below is planted: none may reach the report.
var discoverSecrets = []string{"sk-planted-not-a-real-key", "hunter2-planted", "planted-url-token", "planted-arg-secret", "/home/dev/private-project"}

func hookFor(agent string) string {
	return `{"hooks": {"PreToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "ascc", "args": ["hook", "claude-code", "--agent", "` + agent + `"], "onFailure": "block"}]}]}}`
}

func discoverTree() map[string][]byte {
	return map[string][]byte{
		// Governed: a hook for a registered agent, and a server whose tool the registry lists.
		"governed/.claude/settings.json": []byte(hookFor("dev-agent")),
		"governed/.mcp.json":             []byte(`{"mcpServers": {"tickets": {"command": "npx", "args": ["tickets-mcp"], "env": {"LOG_LEVEL": "debug"}}}}`),
		// A hook for an agent nobody registered.
		"ghost/.claude/settings.local.json": []byte(hookFor("ghost-agent")),
		// No hook at all, and an unregistered server holding credentials.
		"shadow/.claude/settings.json": []byte(`{"permissions": {"allow": ["Bash"]}}`),
		"shadow/.mcp.json": []byte(`{"mcpServers": {"paste": {"command": "npx", "args": ["paste-mcp", "--key=planted-arg-secret"],
			"env": {"PASTE_API_KEY": "sk-planted-not-a-real-key", "dbPassword": "hunter2-planted", "PATH": "/usr/bin", "LOG_LEVEL": "debug"}}}}`),
		// A user file: a global server, and one under a project path that must stay out of the report.
		".claude.json": []byte(`{"mcpServers": {"my docs": {"type": "http", "url": "https://docs.example.com/mcp?t=planted-url-token"}},
			"projects": {"/home/dev/private-project": {"mcpServers": {"paste": {"type": "http", "url": "https://paste.example/mcp",
			"headers": {"Authorization": "Bearer sk-planted-not-a-real-key", "X-Trace": "1"}}}}}}`),
		"etc/managed-mcp.json":      []byte(`{"mcpServers": {"wiki": {"command": "wiki-mcp"}}}`),
		"etc/managed-settings.json": []byte(hookFor("fleet-agent")),
		// A hook the real hook would refuse speaks for nobody.
		"broken-hook/.claude/settings.json": []byte(hookFor(`dev-agent", "stray`)),
		// Hooks for a registered agent that can never block don't govern a setup.
		"dead-matcher/.claude/settings.json":   []byte(strings.Replace(hookFor("dev-agent"), `"matcher": "*"`, `"matcher": "NoSuchTool"`, 1)),
		"dead-async/.claude/settings.json":     []byte(strings.Replace(hookFor("dev-agent"), `"onFailure": "block"`, `"onFailure": "block", "async": true`, 1)),
		"fail-open/.claude/settings.json":      []byte(strings.Replace(hookFor("dev-agent"), `, "onFailure": "block"`, ``, 1)),
		"disabled/.claude/settings.json":       []byte(hookFor("dev-agent")),
		"disabled/.claude/settings.local.json": []byte(`{"disableAllHooks": true}`),
		// A renamed file is still live on a case-insensitive filesystem.
		"renamed/.MCP.json":           []byte(`{"mcpServers": {"wiki": {}}}`),
		"unreadable/.mcp.json":        []byte(`{"mcpServers": {"hidden": {"env": {"API_KEY": "sk-planted-not-a-real-key`),
		"array/.claude/settings.json": []byte(`[]`),
	}
}

// Spec criterion 15: the unregistered MCP server and the credential-looking variable name are
// reported, and no variable value is.
func TestDiscover(t *testing.T) {
	reg, err := pdp.ParseRegistry([]byte(discoverRegistry))
	if err != nil {
		t.Fatal(err)
	}
	inv, unreadable := Discover(discoverTree(), reg, "0.0.0", time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))

	var ids, findings []string
	for _, id := range inv.Identities {
		ids = append(ids, id.ID)
		if id.Tags[0] != "coverage:unmonitored" || id.Status != "unknown" || id.Platform != "local" {
			t.Errorf("%s: tags %v, status %s, platform %s", id.ID, id.Tags, id.Status, id.Platform)
		}
	}
	for _, f := range inv.Findings {
		if f.RuleID != "ascc/unregistered" || f.Evidence == "" || f.Remediation == "" {
			t.Errorf("finding %+v", f)
		}
		findings = append(findings, f.IdentityID)
	}
	wantIDs := []string{
		"local:agent/fleet-agent", "local:agent/ghost-agent",
		"local:claude-code-setup/broken-hook", "local:claude-code-setup/dead-async", "local:claude-code-setup/dead-matcher",
		"local:claude-code-setup/disabled", "local:claude-code-setup/fail-open", "local:claude-code-setup/renamed", "local:claude-code-setup/shadow",
		"local:credential/paste/Authorization", "local:credential/paste/PASTE_API_KEY", "local:credential/paste/dbPassword",
		"local:mcp-server/paste", "local:mcp-server/wiki",
	}
	slices.Sort(ids)
	if !slices.Equal(ids, wantIDs) {
		t.Errorf("identities\n got %v\nwant %v", ids, wantIDs)
	}
	// Candidates are named, not accused: only setups, agents and servers get a finding.
	wantFindings := []string{"local:agent/fleet-agent", "local:agent/ghost-agent", "local:claude-code-setup/broken-hook",
		"local:claude-code-setup/dead-async", "local:claude-code-setup/dead-matcher", "local:claude-code-setup/disabled",
		"local:claude-code-setup/fail-open", "local:claude-code-setup/renamed", "local:claude-code-setup/shadow",
		"local:mcp-server/paste", "local:mcp-server/wiki"}
	if slices.Sort(findings); !slices.Equal(findings, wantFindings) {
		t.Errorf("findings\n got %v\nwant %v", findings, wantFindings)
	}
	// A file ascc can't read is named, never skipped in silence.
	if want := []string{"array/.claude/settings.json", "unreadable/.mcp.json"}; !slices.Equal(unreadable, want) {
		t.Errorf("unreadable %v, want %v", unreadable, want)
	}

	raw, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range discoverSecrets {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the report holds a config value: %s", secret)
		}
	}
	for _, want := range []string{`"source:shadow/.mcp.json"`, `"source:.claude.json"`, `"candidate"`, `"used-by:local:mcp-server/paste"`, `"locationsVersion":"` + LocationsVersion + `"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the report lacks %s", want)
		}
	}
	// The same files always give the same bytes.
	again, _ := Discover(discoverTree(), reg, "0.0.0", time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))
	if raw2, _ := json.Marshal(again); string(raw2) != string(raw) {
		t.Error("two runs over the same files differ")
	}
}

// A known limit, pinned so a change to it is deliberate: a server is recognised by name alone, and
// Claude Code's own tool naming can't tell server "a" from a registered server "a__b".
func TestDiscoverMatchesServersByNameOnly(t *testing.T) {
	reg, err := pdp.ParseRegistry([]byte(`{"agents": [{"id": "dev-agent", "owner": "o", "runtime": "claude-code", "risk_tier": 1, "status": "active"}],
		"tools": {"claude-code/mcp__a__b__get": []}}`))
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := Discover(map[string][]byte{"managed-mcp.json": []byte(`{"mcpServers": {"a": {}, "c": {}}}`)}, reg, "0.0.0", time.Unix(0, 0))
	if len(inv.Identities) != 1 || inv.Identities[0].ID != "local:mcp-server/c" {
		t.Fatalf("identities %+v", inv.Identities)
	}
}

func TestIsLocation(t *testing.T) {
	for rel, want := range map[string]bool{
		".mcp.json": true, "repo/.mcp.json": true, "repo/.claude/settings.json": true, ".claude/settings.local.json": true,
		".claude.json": true, "etc/managed-settings.json": true, "managed-mcp.json": true,
		"repo/.MCP.json": true, ".Claude/Settings.json": true, "x.mcp.json": false,
		"repo/settings.json": false, "repo/x.mcp.json": false, "repo/.claude/settings.json.bak": false, "repo/mcp.json": false,
	} {
		if IsLocation(rel) != want {
			t.Errorf("IsLocation(%q) = %v", rel, !want)
		}
	}
}

func TestCredentialName(t *testing.T) {
	for name, want := range map[string]bool{
		"API_KEY": true, "GITHUB_TOKEN": true, "dbPassword": true, "Authorization": true, "X-Api-Key": true, "client-secret": true,
		"GH_PAT": true, "apiKey": true, "AWS_SECRET_ACCESS_KEY": true,
		"PATH": false, "LOG_LEVEL": false, "TOKENIZER_PARALLELISM": false, "KEYBOARD": false, "X-Trace": false, "": false,
	} {
		if credentialName(name) != want {
			t.Errorf("credentialName(%q) = %v", name, !want)
		}
	}
}
