package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

var (
	root     = filepath.Join("..", "..", "..")
	examples = filepath.Join(root, "examples")
	schemas  = filepath.Join(root, "schemas", "platform", "v1alpha1")
)

// sandbox copies the sample registry and policies somewhere a test can change them.
func sandbox(t testing.TB) (s *server, log *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	pol := filepath.Join(dir, "policies")
	if err := os.Mkdir(pol, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(examples, "ascc.registry.json"), filepath.Join(dir, "ascc.registry.json"))
	for _, name := range []string{"allow.cedar", "deny.cedar"} {
		copyFile(t, filepath.Join(examples, "policies", name), filepath.Join(pol, name))
	}
	log = &bytes.Buffer{}
	return &server{registry: filepath.Join(dir, "ascc.registry.json"), policies: pol, log: log}, log
}

func copyFile(t testing.TB, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ask builds the F §6 request for one of the sample requests in examples/requests.
func ask(t testing.TB, sample string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examples, "requests", sample))
	if err != nil {
		t.Fatal(err)
	}
	r, err := pdp.ParseRequest(data)
	if err != nil {
		t.Fatal(err)
	}
	return body(t, r.Principal, r.Resource, r.Context)
}

func body(t testing.TB, agent, tool string, ctx map[string]any) string {
	t.Helper()
	out, err := json.Marshal(request{
		TenantID: "tnt_lab", Principal: entityRef{"Agent", agent}, Action: "runtime.tool_use",
		Resource: entityRef{"Tool", tool}, Context: ctx, TokenJTI: "tok_test", TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func post(t testing.TB, s *server, reqBody string) (int, response, string) {
	t.Helper()
	return do(t, s, http.MethodPost, "/pdp/v1/decide", reqBody)
}

func do(t testing.TB, s *server, method, path, reqBody string) (int, response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	s.handler().ServeHTTP(rec, req)
	raw := rec.Body.String()
	var res response
	if path != "/healthz" {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&res); err != nil {
			t.Fatalf("the answer isn't a decision response: %v: %s", err, raw)
		}
	}
	return rec.Code, res, raw
}

func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	sch, err := jsonschema.NewCompiler().Compile(filepath.Join(schemas, name+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, doc string) {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(v); err != nil {
		t.Errorf("doesn't match the schema: %v\n%s", err, doc)
	}
}

// The service gives the answer `ascc decide` gives on the same sample files, in the F §6 shape.
func TestDecideMatchesTheCLIOnTheSamples(t *testing.T) {
	s, _ := sandbox(t)
	reqSchema, resSchema := compile(t, "pdp-request"), compile(t, "pdp-response")
	tests := []struct{ sample, want, policy string }{
		{"read-repo-file.json", "allow", "allow-repo-reads"},
		{"exfil-curl.json", "deny", "deny-network-with-secret-path"},
		{"push-to-main.json", "deny", "deny-push-to-main"},
	}
	for _, tt := range tests {
		reqBody := ask(t, tt.sample)
		validate(t, reqSchema, reqBody)
		code, res, raw := post(t, s, reqBody)
		validate(t, resSchema, raw)
		if code != 200 || res.Decision != tt.want || len(res.DeterminingPolicies) != 1 || res.DeterminingPolicies[0] != tt.policy {
			t.Errorf("%s: status %d, %+v, want %s by %s", tt.sample, code, res, tt.want, tt.policy)
		}
		if len(res.Reasons) != 1 || res.Reasons[0] == "" || !strings.HasPrefix(res.PolicyVersion, "sha256:") || res.Degraded || len(res.Obligations) != 0 {
			t.Errorf("%s: %+v", tt.sample, res)
		}
	}
}

func TestEveryOtherRequestIsADeny(t *testing.T) {
	s, _ := sandbox(t)
	resSchema := compile(t, "pdp-response")
	good := ask(t, "read-repo-file.json")
	inRepo := map[string]any{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false}
	swap := func(old, new string) string {
		if !strings.Contains(good, old) {
			t.Fatalf("the sample request has no %s", old)
		}
		return strings.Replace(good, old, new, 1)
	}
	tests := []struct {
		name, method, body string
		status             int
	}{
		{"GET", http.MethodGet, "", 405},
		{"PUT", http.MethodPut, good, 405},
		{"empty body", http.MethodPost, "", 400},
		{"not JSON", http.MethodPost, "allow", 400},
		{"an array", http.MethodPost, "[]", 400},
		{"an unknown field", http.MethodPost, swap(`"tenant_id"`, `"decision":"allow","tenant_id"`), 400},
		{"two documents", http.MethodPost, good + good, 400},
		{"a stray brace after the document", http.MethodPost, good + "}", 400},
		{"a stray bracket after the document", http.MethodPost, good + "]", 400},
		{"a second context that overrides the first", http.MethodPost, swap(`"token_jti"`, `"context":{"path_in_repo":true},"token_jti"`), 400},
		{"a second principal", http.MethodPost, swap(`"action"`, `"principal":{"type":"Agent","id":"dev-agent"},"action"`), 400},
		{"a key repeated inside the context", http.MethodPost, swap(`"network":false`, `"network":true,"network":false`), 400},
		{"a key repeated inside the principal", http.MethodPost, swap(`"id":"dev-agent"`, `"id":"ghost-agent","id":"dev-agent"`), 400},
		{"CONTEXT for context", http.MethodPost, swap(`"context"`, `"CONTEXT"`), 400},
		{"Principal for principal", http.MethodPost, swap(`"principal"`, `"Principal"`), 400},
		{"Type for type", http.MethodPost, swap(`"type":"Agent"`, `"Type":"Agent"`), 400},
		{"null", http.MethodPost, "null", 400},
		{"a principal that is a string", http.MethodPost, swap(`{"type":"Agent","id":"dev-agent"}`, `"dev-agent"`), 400},
		{"over 1 MiB", http.MethodPost, swap(`"tok_test"`, `"`+strings.Repeat("a", maxBody)+`"`), 413},
		{"a principal that isn't an Agent", http.MethodPost, swap(`"type":"Agent"`, `"type":"User"`), 200},
		{"a resource that isn't a Tool", http.MethodPost, swap(`"type":"Tool"`, `"type":"Agent"`), 200},
		{"no principal type", http.MethodPost, swap(`"type":"Agent"`, `"type":""`), 200},
		{"an agent the registry doesn't list", http.MethodPost, body(t, "ghost-agent", "claude-code/Read", inRepo), 200},
		{"a tool the registry doesn't list", http.MethodPost, body(t, "dev-agent", "claude-code/Teleport", inRepo), 200},
		{"an action there is no policy for", http.MethodPost, swap(`"runtime.tool_use"`, `"mcp.call"`), 200},
		{"a context attribute the schema lacks", http.MethodPost, body(t, "dev-agent", "claude-code/Read", map[string]any{"path_in_repo": true, "approved": true}), 200},
		{"no context", http.MethodPost, body(t, "dev-agent", "claude-code/Read", nil), 200},
	}
	for _, tt := range tests {
		code, res, raw := do(t, s, tt.method, "/pdp/v1/decide", tt.body)
		if code != tt.status || res.Decision != "deny" || len(res.Reasons) != 1 || res.Reasons[0] == "" {
			t.Errorf("%s: status %d, %+v, want a deny with a reason and status %d", tt.name, code, res, tt.status)
		}
		validate(t, resSchema, raw)
	}
	// The exfiltration sample is a deny; a second context must not turn it into an allow.
	exfil := ask(t, "exfil-curl.json")
	if _, res, _ := post(t, s, strings.Replace(exfil, `"token_jti"`, `"context":{"touches_secret_path":false},"token_jti"`, 1)); res.Decision != "deny" {
		t.Errorf("a second context overrode the first: %+v", res)
	}
	// Every other path and every other content type is a deny too, never a bare error page.
	for _, path := range []string{"/", "/pdp/v1/decide/", "/pdp/v1/Decide", "/pdp/v2/decide"} {
		if code, res, _ := do(t, s, http.MethodPost, path, good); code != 404 || res.Decision != "deny" {
			t.Errorf("%s: status %d, %+v", path, code, res)
		}
	}
	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pdp/v1/decide", strings.NewReader(good))
		req.Header.Set("Content-Type", ct)
		s.handler().ServeHTTP(rec, req)
		if rec.Code != 415 || !strings.Contains(rec.Body.String(), `"decision":"deny"`) {
			t.Errorf("content type %q: %d %s", ct, rec.Code, rec.Body)
		}
	}
	if code, _, _ := post(t, s, good); code != 200 {
		t.Fatal("the unchanged sample request stopped working")
	}
}

// The kill switch, a registry edit and a policy edit all take effect on the next request.
func TestChangesOnDiskApplyWithoutARestart(t *testing.T) {
	s, _ := sandbox(t)
	good := ask(t, "read-repo-file.json")
	allowed := func(when string) string {
		t.Helper()
		code, res, _ := post(t, s, good)
		if code != 200 || res.Decision != "allow" {
			t.Fatalf("%s: status %d, %+v, want allow", when, code, res)
		}
		return res.PolicyVersion
	}
	denied := func(when string, status int) response {
		t.Helper()
		code, res, _ := post(t, s, good)
		if code != status || res.Decision != "deny" {
			t.Fatalf("%s: status %d, %+v, want deny and status %d", when, code, res, status)
		}
		return res
	}
	rewrite := func(path string, change func(string) string) (restore func()) {
		t.Helper()
		orig, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		changed := change(string(orig))
		if changed == string(orig) {
			t.Fatalf("the change to %s changed nothing", filepath.Base(path))
		}
		if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		return func() {
			if err := os.WriteFile(path, orig, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	first := allowed("at the start")

	// What `ascc kill dev-agent` does to the registry.
	restore := rewrite(s.registry, func(reg string) string {
		i := strings.Index(reg, `"dev-agent"`)
		return reg[:i] + strings.Replace(reg[i:], `"active"`, `"suspended"`, 1)
	})
	if res := denied("after the agent is suspended", 200); res.PolicyVersion == first {
		t.Error("the policy version didn't change with the registry")
	}
	restore()
	if v := allowed("after the agent is revived"); v != first {
		t.Errorf("the same files gave another policy version: %s, then %s", first, v)
	}

	allow := filepath.Join(s.policies, "allow.cedar")
	restore = rewrite(allow, func(src string) string { return src + "\npermit (principal, action, resource" })
	res := denied("with a policy that doesn't parse", 503)
	if strings.Contains(res.Reasons[0], "permit") || strings.Contains(res.Reasons[0], "allow.cedar") {
		t.Errorf("the reason repeats the policy file: %s", res.Reasons[0])
	}
	if code, _, _ := do(t, s, http.MethodGet, "/healthz", ""); code != 503 {
		t.Errorf("healthz is %d while nothing can be allowed", code)
	}
	restore()
	allowed("after the policy is fixed")

	restore = rewrite(s.registry, func(string) string { return "{" })
	denied("with a registry that doesn't parse", 503)
	restore()

	if err := os.Rename(allow, allow+".off"); err != nil {
		t.Fatal(err)
	}
	denied("with the permits gone", 200) // deny.cedar still loads: default deny
	if err := os.Rename(filepath.Join(s.policies, "deny.cedar"), filepath.Join(s.policies, "deny.off")); err != nil {
		t.Fatal(err)
	}
	denied("with no policy file at all", 503)
	if err := os.Remove(s.registry); err != nil {
		t.Fatal(err)
	}
	denied("with no registry", 503)
}

func TestHealthIsOKOnlyWhenADecisionCouldBeAnAllow(t *testing.T) {
	s, _ := sandbox(t)
	if code, _, raw := do(t, s, http.MethodGet, "/healthz", ""); code != 200 || strings.TrimSpace(raw) != "ok" {
		t.Errorf("healthz: %d %q", code, raw)
	}
	if code, _, _ := do(t, &server{registry: "nothing-here.json", policies: "nothing-here", log: io.Discard}, http.MethodGet, "/healthz", ""); code != 503 {
		t.Errorf("healthz with nothing to load: %d", code)
	}
}

// A request can carry a secret in its context. It must not come back in the answer or reach the log.
func TestNoRequestValueIsRepeated(t *testing.T) {
	s, log := sandbox(t)
	const marker = "MARKER-not-a-real-secret"
	requests := []string{
		body(t, "dev-agent", "claude-code/Read", map[string]any{"path_in_repo": true, "note": marker}),
		body(t, "dev-agent", "claude-code/Read", map[string]any{"path_in_repo": marker}),
		body(t, "dev-agent", "claude-code/Read", map[string]any{marker: true}),
		`{"tenant_id":"tnt_lab","` + marker + `":1}`,
		`{"tenant_id":"` + marker,
	}
	for i, r := range requests {
		_, res, raw := post(t, s, r)
		if res.Decision != "deny" || strings.Contains(raw, marker) {
			t.Errorf("request %d: answer %s", i, raw)
		}
	}
	if strings.Contains(log.String(), marker) {
		t.Errorf("the log repeats a request value:\n%s", log)
	}
	if lines := strings.Count(log.String(), "\n"); lines != len(requests) || !strings.Contains(log.String(), `"input_sha256"`) {
		t.Errorf("want one log line per decision with the request's hash, got %d:\n%s", lines, log)
	}
}

type brokenLog struct{}

func (brokenLog) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// An allow that can't be written down doesn't go out, whether the log refuses the line or panics.
func TestADecisionThatCantBeRecordedIsADeny(t *testing.T) {
	good := ask(t, "read-repo-file.json")
	for name, log := range map[string]io.Writer{"a log that refuses the line": brokenLog{}, "no log at all": nil} {
		s, _ := sandbox(t)
		if _, res, _ := post(t, s, good); res.Decision != "allow" {
			t.Fatalf("the sample request isn't an allow to begin with: %+v", res)
		}
		s.log = log
		if code, res, _ := post(t, s, good); code != 500 || res.Decision != "deny" {
			t.Errorf("%s: status %d, %+v, want a deny and 500", name, code, res)
		}
	}
}

func TestAnEmptiedPolicyFileDeniesEverything(t *testing.T) {
	s, _ := sandbox(t)
	good := ask(t, "read-repo-file.json")
	if err := os.WriteFile(filepath.Join(s.policies, "deny.cedar"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, res, _ := post(t, s, good); code != 503 || res.Decision != "deny" {
		t.Errorf("with the forbids file emptied: status %d, %+v, want a deny and 503", code, res)
	}
	if err := os.WriteFile(filepath.Join(s.policies, "deny.cedar"), []byte("// nothing but a comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, res, _ := post(t, s, good); code != 503 || res.Decision != "deny" {
		t.Errorf("with only a comment in the forbids file: status %d, %+v, want a deny and 503", code, res)
	}
}

func TestListenAddress(t *testing.T) {
	tests := []struct {
		addr        string
		allowRemote bool
		ok          bool
	}{
		{"127.0.0.1:8181", false, true},
		{"[::1]:8181", false, true},
		{"localhost:8181", false, false},
		{"127.0.0.1.example.com:8181", false, false},
		{"[::ffff:192.0.2.1]:8181", false, false},
		{"0.0.0.0:8181", false, false},
		{":8181", false, false},
		{"[::]:8181", false, false},
		{"192.0.2.10:8181", false, false},
		{"pdp.example.com:8181", false, false},
		{"127.0.0.1", false, false},
		{"0.0.0.0:8181", true, true},
		{"not an address", true, false},
	}
	for _, tt := range tests {
		if err := checkListen(tt.addr, tt.allowRemote); (err == nil) != tt.ok {
			t.Errorf("%q allow-remote=%v: %v, want ok=%v", tt.addr, tt.allowRemote, err, tt.ok)
		}
	}
	var stderr bytes.Buffer
	if code := run([]string{"--listen", "0.0.0.0:0"}, io.Discard, &stderr); code != 2 || !strings.Contains(stderr.String(), "doesn't authenticate") {
		t.Errorf("exit %d: %s", code, &stderr)
	}
	if code := run([]string{"extra"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("a stray argument: exit %d", code)
	}
}

// scale writes a registry and policy set of the size ADR-03 asks to be timed: 5,000 entities
// (1,000 agents and 4,000 tools) and 500 policies.
func scale(tb testing.TB) *server {
	tb.Helper()
	dir := tb.TempDir()
	type agent struct {
		ID       string   `json:"id"`
		Owner    string   `json:"owner"`
		Runtime  string   `json:"runtime"`
		RiskTier int      `json:"risk_tier"`
		Status   string   `json:"status"`
		Groups   []string `json:"groups"`
	}
	reg := struct {
		Agents []agent             `json:"agents"`
		Tools  map[string][]string `json:"tools"`
	}{Tools: map[string][]string{}}
	for i := 0; i < 1000; i++ {
		reg.Agents = append(reg.Agents, agent{fmt.Sprintf("agent-%04d", i), "platform-team", "claude-code", 1, "active", []string{"developers"}})
	}
	for i := 0; i < 4000; i++ {
		reg.Tools[fmt.Sprintf("claude-code/Tool%04d", i)] = []string{"read-only"}
	}
	data, err := json.Marshal(reg)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ascc.registry.json"), data, 0o644); err != nil {
		tb.Fatal(err)
	}
	var pol strings.Builder
	for i := 0; i < 497; i++ {
		fmt.Fprintf(&pol, "@id(\"allow-generated-%04d\")\n@reason(\"generated for the timing\")\npermit (principal == Agent::\"agent-%04d\", action == Action::\"runtime.tool_use\", resource in ToolGroup::\"read-only\")\nwhen { context.path_in_repo };\n\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "generated.cedar"), []byte(pol.String()), 0o644); err != nil {
		tb.Fatal(err)
	}
	copyFile(tb, filepath.Join(examples, "policies", "deny.cedar"), filepath.Join(dir, "deny.cedar")) // 3 forbids: 500 in all
	return &server{registry: filepath.Join(dir, "ascc.registry.json"), policies: dir, log: io.Discard}
}

func TestScaleSetLoadsAndDecides(t *testing.T) {
	s := scale(t)
	inRepo := map[string]any{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false}
	if _, res, _ := post(t, s, body(t, "agent-0496", "claude-code/Tool3999", inRepo)); res.Decision != "allow" || res.DeterminingPolicies[0] != "allow-generated-0496" {
		t.Errorf("an agent with a permit: %+v", res)
	}
	if _, res, _ := post(t, s, body(t, "agent-0999", "claude-code/Tool3999", inRepo)); res.Decision != "deny" {
		t.Errorf("an agent with no permit: %+v", res)
	}
}

func p99(took []time.Duration) float64 {
	sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
	return float64(took[len(took)*99/100].Microseconds())
}

// BenchmarkDecideAtScale is the timing ADR-03 owed: one decision at 500 policies and 5,000 entities,
// in the engine alone and through the service (which also reads and hashes the files).
func BenchmarkDecideAtScale(b *testing.B) {
	s := scale(b)
	p, _, err := s.current()
	if err != nil {
		b.Fatal(err)
	}
	ctx := map[string]any{"path_in_repo": true, "network": false, "touches_secret_path": false, "touches_ascc_state": false}
	b.Run("engine", func(b *testing.B) {
		took := make([]time.Duration, 0, b.N)
		for i := 0; i < b.N; i++ {
			t0 := time.Now()
			if d := p.Decide(pdp.Request{Principal: "agent-0250", Action: "runtime.tool_use", Resource: "claude-code/Tool2000", Context: ctx}); d.Decision != "allow" {
				b.Fatal(d)
			}
			took = append(took, time.Since(t0))
		}
		b.ReportMetric(p99(took), "p99-µs")
	})
	b.Run("service", func(b *testing.B) {
		reqBody := body(b, "agent-0250", "claude-code/Tool2000", ctx)
		h := s.handler()
		took := make([]time.Duration, 0, b.N)
		for i := 0; i < b.N; i++ {
			t0 := time.Now()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/pdp/v1/decide", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			h.ServeHTTP(rec, req)
			if rec.Code != 200 {
				b.Fatal(rec.Body)
			}
			took = append(took, time.Since(t0))
		}
		b.ReportMetric(p99(took), "p99-µs")
	})
}
