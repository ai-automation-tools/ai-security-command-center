package pdp

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const examples = "../../../examples"

func read(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// samplePDP loads the committed examples/, the same files the README runs.
func samplePDP(t *testing.T) *PDP {
	t.Helper()
	p, err := New(read(t, examples, "ascc.registry.json"), map[string][]byte{
		"allow.cedar": read(t, examples, "policies", "allow.cedar"),
		"deny.cedar":  read(t, examples, "policies", "deny.cedar"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func ctx(inRepo, network, secret bool) map[string]any {
	return map[string]any{"path_in_repo": inRepo, "network": network, "touches_secret_path": secret, "touches_ascc_state": false}
}

// R1 to R9 are the requests from docs/reports/2026-cedar-go-check.md. Spec criteria 1, 2, 12 and 13.
func TestDecide(t *testing.T) {
	p := samplePDP(t)
	pushMain := ctx(true, true, false)
	pushMain["git_push_target"] = "main"
	pushBranch := ctx(true, true, false)
	pushBranch["git_push_target"] = "feature/x"
	pushAll := ctx(true, true, false)
	pushAll["git_push_target"] = "*"
	stateChange := ctx(true, false, false)
	stateChange["touches_ascc_state"] = true

	tests := []struct {
		name, agent, action, tool string
		context                   map[string]any
		want                      string
		ids                       []string
		reason                    string // substring
	}{
		{"R1 read in repo", "dev-agent", ActionToolUse, "claude-code/Read", ctx(true, false, false), "allow", []string{"allow-repo-reads"}, "may read inside its repo"},
		{"R2 grep through the group", "dev-agent", ActionToolUse, "claude-code/Grep", ctx(true, false, false), "allow", []string{"allow-repo-reads"}, ""},
		{"R3 curl a secret file out", "dev-agent", ActionToolUse, "claude-code/Bash", ctx(true, true, true), "deny", []string{"deny-network-with-secret-path"}, "may not send a secret file over the network"},
		{"R3 same through PowerShell", "dev-agent", ActionToolUse, "claude-code/PowerShell", ctx(true, true, true), "deny", []string{"deny-network-with-secret-path"}, ""},
		{"R4 push to main", "dev-agent", ActionToolUse, "claude-code/Bash", pushMain, "deny", []string{"deny-push-to-main"}, "open a pull request"},
		{"push to every branch", "dev-agent", ActionToolUse, "claude-code/Bash", pushAll, "deny", []string{"deny-push-to-main"}, ""},
		{"change ascc state", "dev-agent", ActionToolUse, "claude-code/Write", stateChange, "deny", []string{"deny-ascc-state-changes"}, "may not change ascc"},
		{"push to a branch", "dev-agent", ActionToolUse, "claude-code/Bash", pushBranch, "allow", []string{"allow-shell-low-risk"}, ""},
		{"R5 plain shell command", "dev-agent", ActionToolUse, "claude-code/Bash", ctx(true, false, false), "allow", []string{"allow-shell-low-risk"}, ""},
		{"R6 suspended agent", "old-agent", ActionToolUse, "claude-code/Read", ctx(true, false, false), "deny", []string{}, `"old-agent" is suspended`},
		{"R7 no permit for the tool", "dev-agent", ActionToolUse, "claude-code/WebFetch", ctx(false, true, false), "deny", []string{}, "default deny"},
		{"R8 agent not in the registry", "ghost-agent", ActionToolUse, "claude-code/Read", ctx(true, false, false), "deny", []string{}, `"ghost-agent" is not in the registry`},
		{"read outside the repo", "dev-agent", ActionToolUse, "claude-code/Read", ctx(false, false, false), "deny", []string{}, "default deny"},
		{"unknown tool", "dev-agent", ActionToolUse, "claude-code/Teleport", ctx(true, false, false), "deny", []string{}, `"claude-code/Teleport" is not in the registry`},
		{"unknown action", "dev-agent", "runtime.anything", "claude-code/Read", ctx(true, false, false), "deny", []string{}, "unknown action"},
		{"internal tool", "ci-agent", ActionToolUse, "claude-code/TodoWrite", ctx(false, false, false), "allow", []string{"allow-internal-tools"}, ""},
		{"shell above the risk tier", "ci-agent", ActionToolUse, "claude-code/Bash", ctx(true, false, false), "deny", []string{}, "default deny"},
		{"group permit applies to a member", "dev-agent", ActionToolUse, "claude-code/Write", ctx(true, false, false), "allow", []string{"allow-repo-writes-for-developers"}, ""},
		{"group permit skips a non-member", "ci-agent", ActionToolUse, "claude-code/Write", ctx(true, false, false), "deny", []string{}, "default deny"},
		{"context attribute missing", "dev-agent", ActionToolUse, "claude-code/Bash", map[string]any{"path_in_repo": true}, "deny", []string{}, "doesn't fit the schema"},
		{"context attribute unknown", "dev-agent", ActionToolUse, "claude-code/Read", map[string]any{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false, "sudo": true}, "deny", []string{}, "doesn't fit the schema"},
		{"context attribute of the wrong type", "dev-agent", ActionToolUse, "claude-code/Bash", map[string]any{"path_in_repo": true, "network": "no", "touches_secret_path": false, "touches_ascc_state": false}, "deny", []string{}, "doesn't fit the schema"},
		{"context value that isn't a bool or string", "dev-agent", ActionToolUse, "claude-code/Bash", map[string]any{"path_in_repo": 1.0}, "deny", []string{}, "every context attribute must be"},
		{"no context at all", "dev-agent", ActionToolUse, "claude-code/Read", nil, "deny", []string{}, "doesn't fit the schema"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Decide(Request{Principal: tt.agent, Action: tt.action, Resource: tt.tool, Context: tt.context})
			if got.Decision != tt.want || !slices.Equal(got.PolicyIDs, tt.ids) || !strings.Contains(got.Reason, tt.reason) || got.Reason == "" {
				t.Fatalf("got %+v\nwant %s %v with reason containing %q", got, tt.want, tt.ids, tt.reason)
			}
		})
	}
}

const (
	okRegistry = `{"agents":[{"id":"a","owner":"o","runtime":"claude-code","risk_tier":1,"status":"active"}],"tools":{"claude-code/Read":[]}}`
	okPolicy   = `@id("p") permit (principal, action, resource);`
)

// A context can hold anything the caller put there. The reason is shown to the agent and kept in the
// audit log, so it never repeats a context value.
func TestReasonNeverRepeatsAContextValue(t *testing.T) {
	p := samplePDP(t)
	const marker = "MARKER-not-a-real-secret"
	for _, ctx := range []map[string]any{
		{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false, "note": marker},
		{"path_in_repo": marker},
		{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false, "git_push_target": marker, "extra": true},
	} {
		d := p.Decide(Request{Principal: "dev-agent", Action: ActionToolUse, Resource: "claude-code/Bash", Context: ctx})
		if d.Decision != "deny" || strings.Contains(d.Reason, marker) {
			t.Errorf("%+v", d)
		}
	}
}

func TestRetiredAgentIsDenied(t *testing.T) {
	reg := strings.Replace(okRegistry, `"active"`, `"retired"`, 1)
	p, err := New([]byte(reg), map[string][]byte{"p.cedar": []byte(okPolicy)})
	if err != nil {
		t.Fatal(err)
	}
	got := p.Decide(Request{Principal: "a", Action: ActionToolUse, Resource: "claude-code/Read", Context: ctx(true, false, false)})
	if got.Decision != "deny" || !strings.Contains(got.Reason, "retired") {
		t.Fatalf("got %+v", got)
	}
}

// A policy that errors is skipped by Cedar, so the permit-everything policy next to it would allow
// the call. The PDP must turn any evaluation error into a deny naming the policy.
func TestEvaluationErrorIsDeny(t *testing.T) {
	overflow := okPolicy + `
@id("overflows") forbid (principal is Agent, action, resource) when { principal.risk_tier + 9223372036854775807 > 0 };`
	p, err := New([]byte(okRegistry), map[string][]byte{"p.cedar": []byte(overflow)})
	if err != nil {
		t.Fatal(err)
	}
	got := p.Decide(Request{Principal: "a", Action: ActionToolUse, Resource: "claude-code/Read", Context: ctx(true, false, false)})
	if got.Decision != "deny" || !slices.Equal(got.PolicyIDs, []string{"overflows"}) || !strings.Contains(got.Reason, "failed to evaluate") {
		t.Fatalf("got %+v", got)
	}
}

// Every one of these must stop ascc loading, with one line naming the fault.
func TestLoadFailsClosed(t *testing.T) {
	agent := func(fields string) string { return `{"agents":[{` + fields + `}],"tools":{}}` }
	tests := []struct{ name, registry, policy, want string }{
		{"registry isn't JSON", `{`, okPolicy, "registry:"},
		{"registry field typo", agent(`"id":"a","owner":"o","runtime":"r","risk_tier":1,"stauts":"active"`), okPolicy, `unknown field "stauts"`},
		{"R9 agent with no status", agent(`"id":"a","owner":"o","runtime":"r","risk_tier":1`), okPolicy, `agent "a" has status ""`},
		{"agent with a made-up status", agent(`"id":"a","owner":"o","runtime":"r","risk_tier":1,"status":"paused"`), okPolicy, "use active, suspended or retired"},
		{"agent with no risk tier", agent(`"id":"a","owner":"o","runtime":"r","status":"active"`), okPolicy, "needs a risk_tier"},
		{"agent with no runtime", agent(`"id":"a","owner":"o","risk_tier":1,"status":"active"`), okPolicy, "has no runtime"},
		{"agent with a bad id", agent(`"id":"a b","owner":"o","runtime":"r","risk_tier":1,"status":"active"`), okPolicy, `id "a b" must be`},
		{"agent listed twice", `{"agents":[{"id":"a","runtime":"r","risk_tier":1,"status":"active"},{"id":"a","runtime":"r","risk_tier":1,"status":"active"}]}`, okPolicy, "listed twice"},
		{"no agents", `{"agents":[]}`, okPolicy, `"agents" is empty`},
		{"bad group name", agent(`"id":"a","runtime":"r","risk_tier":1,"status":"active","groups":["x y"]`), okPolicy, `group "x y"`},
		{"no policy", okRegistry, "", "no policy in this file"},
		{"policy syntax error", okRegistry, `permit (principal`, "policies: p.cedar:"},
		{"policy with no @id", okRegistry, `permit (principal, action, resource);`, "has no @id"},
		{"policy @id used twice", okRegistry, okPolicy + okPolicy, `@id "p" is used twice`},
		{"policy reads a misspelt attribute", okRegistry, `@id("typo") forbid (principal is Agent, action, resource) when { principal.stauts == "suspended" };`, `policy "typo"`},
		{"policy reads an optional attribute without has", okRegistry, `@id("nohas") forbid (principal, action == Action::"runtime.tool_use", resource) when { context.git_push_target == "main" };`, `policy "nohas"`},
		{"policy names an unknown entity type", okRegistry, `@id("who") permit (principal is Robot, action, resource);`, `policy "who"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New([]byte(tt.registry), map[string][]byte{"p.cedar": []byte(tt.policy)})
			if err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "\n") {
				t.Fatalf("got error %v, want one line containing %q", err, tt.want)
			}
		})
	}
}

func TestParseRequestRejectsUnknownFields(t *testing.T) {
	if _, err := ParseRequest([]byte(`{"principal":"a","acton":"x"}`)); err == nil {
		t.Fatal("want an error for the misspelt field")
	}
	for _, name := range []string{"read-repo-file.json", "exfil-curl.json", "push-to-main.json"} {
		if _, err := ParseRequest(read(t, examples, "requests", name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// Spec criterion 7: the inventory validates against the suite schema, and the planted problems in
// the sample registry come out as findings.
func TestInventory(t *testing.T) {
	inv := samplePDP(t).Inventory("0.0.0", time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC), Evidence{})

	validates(t, inv)

	if len(inv.Identities) != 8 { // four agents, then the four identities they use
		t.Fatalf("got %d identities, want 8", len(inv.Identities))
	}
	for _, id := range inv.Identities[:4] {
		// No hook evidence exists yet, so claiming anything above unmonitored would be a lie.
		if id.Kind != "ai-agent" || id.Tags[0] != "coverage:"+CoverageUnmonitored {
			t.Errorf("%s: kind %s, coverage tag %q", id.ID, id.Kind, id.Tags[0])
		}
	}
	want := []string{"ascc/suspended local:agent/old-agent", "ascc/no-owner local:agent/ownerless-agent"}
	if got := findings(inv)[:2]; !slices.Equal(got, want) {
		t.Fatalf("findings %v, want %v", got, want)
	}
}

func TestInventoryNoPolicyFinding(t *testing.T) {
	reg := `{"agents":[
		{"id":"in-group","owner":"o","runtime":"claude-code","risk_tier":1,"status":"active","groups":["ci"]},
		{"id":"named","owner":"o","runtime":"claude-code","risk_tier":1,"status":"active"},
		{"id":"left-out","owner":"o","runtime":"claude-code","risk_tier":1,"status":"active","groups":["other"]}],
		"tools":{"claude-code/Read":[]}}`
	pol := `@id("ci") permit (principal in AgentGroup::"ci", action, resource);
@id("one") permit (principal == Agent::"named", action, resource);
@id("forbid-all") forbid (principal, action, resource) when { context.network };`
	p, err := New([]byte(reg), map[string][]byte{"p.cedar": []byte(pol)})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range p.Inventory("0.0.0", time.Unix(0, 0), Evidence{}).Findings {
		got = append(got, f.RuleID+" "+f.IdentityID)
	}
	if !slices.Equal(got, []string{"ascc/no-policy local:agent/left-out"}) {
		t.Fatalf("findings %v", got)
	}
}

// Spec criterion 14, the label half: unmonitored with no audit lines, observed with audit lines but
// no passing posture check, enforced only with both. Nothing in the registry can raise a label.
func TestCoverageIsComputedFromEvidence(t *testing.T) {
	ev := Evidence{
		Audited:     map[string]bool{"dev-agent": true, "ci-agent": true},
		PosturePass: map[string]bool{"dev-agent": true, "old-agent": true},
	}
	want := map[string]string{
		"dev-agent":       CoverageEnforced,
		"ci-agent":        CoverageObserved,
		"old-agent":       CoverageUnmonitored, // a passing posture check, but the hook has recorded nothing
		"ownerless-agent": CoverageUnmonitored,
	}
	for _, id := range samplePDP(t).Inventory("0.0.0", time.Unix(0, 0), ev).Identities[:4] {
		if got := strings.TrimPrefix(id.Tags[0], "coverage:"); got != want[id.DisplayName] {
			t.Errorf("%s: %s, want %s", id.DisplayName, got, want[id.DisplayName])
		}
	}
}

// The WebAssembly demo imports this package, so it can't touch files, the network or the clock.
func TestPackageDoesNoIO(t *testing.T) {
	banned := []string{"os", "os/exec", "net", "net/http", "io/ioutil", "syscall"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			if path, _ := strconv.Unquote(imp.Path.Value); slices.Contains(banned, path) {
				t.Errorf("%s imports %q", f, path)
			}
		}
	}
}
