package demo

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	examples = "../../../examples"
	now      = "2026-11-10T12:00:00Z"
)

func read(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{examples}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sample(t *testing.T) (registry string, policies map[string]string) {
	t.Helper()
	return read(t, "ascc.registry.json"), map[string]string{
		"allow.cedar": read(t, "policies", "allow.cedar"), "deny.cedar": read(t, "policies", "deny.cedar"),
	}
}

// The planted calls get the answers the page promises, each recorded on one chain.
func TestHookOnThePlantedCalls(t *testing.T) {
	registry, policies := sample(t)
	log := ""
	for _, tt := range []struct{ payload, decision, policy string }{
		{"exfil-curl", "deny", "deny-network-with-secret-path"},
		{"read-repo-file", "allow", "allow-repo-reads"},
		{"run-tests", "allow", "allow-shell-low-risk"},
		{"push-to-main", "deny", "deny-push-to-main"},
		{"edit-settings", "deny", "deny-ascc-state-changes"},
	} {
		a := Hook(registry, policies, read(t, "hook-payloads", tt.payload+".json"), "dev-agent", log, now)
		if a.Decision != tt.decision || !slices.Contains(a.PolicyIDs, tt.policy) || a.Reason == "" {
			t.Errorf("%s: %s %v %q, want %s by %s", tt.payload, a.Decision, a.PolicyIDs, a.Reason, tt.decision, tt.policy)
		}
		log = a.Log
	}
	if ok, report := Verify(log); !ok || !strings.HasPrefix(report, "audit: ok, 5 entries") {
		t.Fatalf("verify: %s", report)
	}
	// No secret at rest: the log holds a hash of the payload, never the command.
	if strings.Contains(log, "paste.example") || strings.Contains(log, "curl") {
		t.Error("the audit log holds raw tool input")
	}
}

// Everything that goes wrong is a deny with a reason, and it is still recorded when it can be.
func TestHookFailsClosed(t *testing.T) {
	registry, policies := sample(t)
	read := read(t, "hook-payloads", "read-repo-file.json")
	good := Hook(registry, policies, read, "dev-agent", "", now).Log
	tests := []struct {
		name, registry, payload, agent, log string
		policies                            map[string]string
		want                                string
		recorded                            bool
	}{
		{"registry isn't JSON", "{", read, "dev-agent", good, policies, "registry:", true},
		{"no policies", registry, read, "dev-agent", good, nil, "policies:", true},
		{"a policy that doesn't parse", registry, read, "dev-agent", good, map[string]string{"p.cedar": "permit (principal"}, "policies:", true},
		{"payload isn't JSON", registry, "{", "dev-agent", good, policies, "payload", true},
		{"unknown agent", registry, read, "ghost-agent", good, policies, "not in the registry", true},
		{"suspended agent", registry, read, "old-agent", good, policies, "suspended", true},
		{"a log whose last line was edited", registry, read, "dev-agent", strings.Replace(good, `"hash":"`, `"hash":"0`, 1), policies, "blocked: audit:", false},
		{"a log cut mid-line", registry, read, "dev-agent", good[:len(good)-5], policies, "blocked: audit:", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Hook(tt.registry, tt.policies, tt.payload, tt.agent, tt.log, now)
			if a.Decision != "deny" || !strings.Contains(a.Reason, tt.want) {
				t.Fatalf("%s %q, want a deny containing %q", a.Decision, a.Reason, tt.want)
			}
			if recorded := a.Log != tt.log; recorded != tt.recorded {
				t.Fatalf("recorded %v, want %v", recorded, tt.recorded)
			}
		})
	}
}

func TestKillSwitchAndVerify(t *testing.T) {
	registry, policies := sample(t)
	payload := read(t, "hook-payloads", "read-repo-file.json")

	killed, log, err := SetStatus(registry, "dev-agent", "kill", "", now)
	if err != nil {
		t.Fatal(err)
	}
	a := Hook(killed, policies, payload, "dev-agent", log, now)
	if a.Decision != "deny" || !strings.Contains(a.Reason, "suspended") {
		t.Fatalf("a killed agent got %s %q", a.Decision, a.Reason)
	}
	// Deleting every forbid doesn't bring it back: the kill switch isn't a policy.
	if a := Hook(killed, map[string]string{"allow.cedar": policies["allow.cedar"]}, payload, "dev-agent", log, now); a.Decision != "deny" {
		t.Fatal("removing the deny policies revived a killed agent")
	}
	revived, log, err := SetStatus(killed, "dev-agent", "revive", a.Log, now)
	if err != nil {
		t.Fatal(err)
	}
	if a = Hook(revived, policies, payload, "dev-agent", log, now); a.Decision != "allow" {
		t.Fatalf("a revived agent got %s %q", a.Decision, a.Reason)
	}
	if ok, report := Verify(a.Log); !ok || !strings.Contains(report, "4 entries") {
		t.Fatalf("verify: %s", report)
	}
	edited := strings.Replace(a.Log, `"event":"kill"`, `"event":"revive"`, 1)
	if ok, report := Verify(edited); ok || !strings.Contains(report, "FAILED") {
		t.Fatalf("an edited log verified: %s", report)
	}
	// Lines added after the edit don't mend the chain.
	if ok, _ := Verify(Hook(revived, policies, payload, "dev-agent", edited, now).Log); ok {
		t.Fatal("appending to an edited log made it verify")
	}

	// The CLI flips the registry before it records, so a kill that can't be recorded still stops the agent.
	reg, sameLog, err := SetStatus(registry, "dev-agent", "kill", "not a log\n", now)
	if err == nil || !strings.Contains(err.Error(), "couldn't be recorded") || sameLog != "not a log\n" ||
		Hook(reg, policies, payload, "dev-agent", "", now).Decision != "deny" {
		t.Errorf("a kill that can't be recorded: err %v", err)
	}
	// An ok report carries the CLI's caveat about lines cut off the end.
	if _, report := Verify(a.Log); !strings.Contains(report, "the head wasn't checked") {
		t.Errorf("verify report: %s", report)
	}

	for name, args := range map[string][4]string{
		"unknown agent": {registry, "ghost-agent", "kill", ""},
		"unknown event": {registry, "dev-agent", "delete", ""},
		"bad registry":  {"{", "dev-agent", "kill", ""},
	} {
		reg, log, err := SetStatus(args[0], args[1], args[2], args[3], now)
		if err == nil || reg != args[0] || log != args[3] {
			t.Errorf("%s: err %v, and the registry or log changed", name, err)
		}
	}
}

// The WebAssembly demo is this package, so it can't touch files, the network or the clock.
func TestPackageDoesNoIO(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "demo.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if slices.Contains([]string{"os", "os/exec", "net", "net/http", "io/ioutil", "syscall", "time"}, path) {
			t.Errorf("demo.go imports %s", path)
		}
	}
}
