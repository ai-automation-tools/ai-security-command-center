package host

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/ai-automation-tools/ai-security-command-center/src/spikes/pluginhost/pluginv1alpha1"
)

// The fictional platform's answer: two apps, one with no owner.
const apps = `[{"ID":"app-1","Name":"build-bot","Owner":"team-platform"},{"ID":"app-2","Name":"old-importer","Owner":""}]`

var full = Grant{
	PluginID: "hello-connector", ConnectorID: "con_example",
	Scopes:          []string{scopeNHIWrite, scopeFindingWrite},
	NetworkHosts:    []string{"api.example.com"},
	CredentialKinds: []string{"example_app"},
}

// recorder is the event sink and the broker of a test host.
type recorder struct {
	mu     sync.Mutex
	events []Event
	calls  int
}

func (r *recorder) event(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) broker(context.Context, string, string, string) (int, []byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return 200, []byte(apps), nil
}

func (r *recorder) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

func (r *recorder) count(typ string) (n int) {
	for _, t := range r.types() {
		if t == typ {
			n++
		}
	}
	return n
}

func newHost(t *testing.T) (*Host, *recorder) {
	t.Helper()
	r := &recorder{}
	h, err := New(r.event, r.broker)
	if err != nil {
		t.Fatal(err)
	}
	return h, r
}

var (
	helloOnce sync.Once
	helloBin  string
	helloErr  error
)

// hello builds the hello connector once for the whole test run.
func hello(t *testing.T) string {
	t.Helper()
	helloOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ascc-hello")
		if err != nil {
			helloErr = err
			return
		}
		helloBin = filepath.Join(dir, "hello-connector")
		if runtime.GOOS == "windows" {
			helloBin += ".exe"
		}
		if out, err := exec.Command("go", "build", "-o", helloBin, "../cmd/hello-connector").CombinedOutput(); err != nil {
			helloErr = errors.New(string(out))
		}
	})
	if helloErr != nil {
		t.Fatalf("can't build the hello connector: %v", helloErr)
	}
	return helloBin
}

func TestMain(m *testing.M) {
	code := m.Run()
	if helloBin != "" {
		os.RemoveAll(filepath.Dir(helloBin))
	}
	os.Exit(code)
}

func start(t *testing.T, h *Host, g Grant) *Plugin {
	t.Helper()
	p, err := h.Start(context.Background(), hello(t), g)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Stop)
	return p
}

// The ticket's acceptance test: a connector in its own process runs under the host, and everything
// recorded about it carries its id.
func TestHelloConnectorRunsUnderTheHost(t *testing.T) {
	h, r := newHost(t)
	p := start(t, h, full)

	if hr, err := p.Health(context.Background()); err != nil || !hr.GetOk() {
		t.Fatalf("health: %v, %v", hr, err)
	}
	res, err := p.Sync(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUpserted() != 2 || res.GetRefused() != 0 || res.GetCursor() != "2" {
		t.Errorf("sync: %v, want 2 upserted, none refused, cursor 2", res)
	}
	want := []string{"plugin.started", "platform.called", "nhi.upserted", "nhi.upserted", "finding.reported"}
	if got := r.types(); !slices.Equal(got, want) {
		t.Errorf("events %v, want %v", got, want)
	}
	for _, e := range r.events {
		if e.PluginID != "hello-connector" || e.ConnectorID != "con_example" || e.ContractVersion != "v1alpha1" || e.At.IsZero() {
			t.Errorf("event without the plugin's identity: %+v", e)
		}
	}
}

// The other half: a call outside what was approved is refused and recorded, and the rest still works.
func TestOutOfScopeCallIsRefused(t *testing.T) {
	g := full
	g.Scopes = []string{scopeNHIWrite}
	h, r := newHost(t)
	p := start(t, h, g)

	res, err := p.Sync(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUpserted() != 2 || res.GetRefused() != 1 {
		t.Errorf("sync: %v, want 2 upserted and 1 refused", res)
	}
	if r.count("finding.reported") != 0 || r.count("plugin.scope_denied") != 1 {
		t.Errorf("events %v: want one scope_denied and no finding", r.types())
	}
	for _, e := range r.events {
		if e.Type == "plugin.scope_denied" && (e.PluginID != "hello-connector" || !strings.Contains(e.Detail, scopeFindingWrite)) {
			t.Errorf("denial doesn't name the plugin and the scope: %+v", e)
		}
	}
}

func TestPlatformCallOutsideTheGrantNeverReachesTheBroker(t *testing.T) {
	tests := map[string]func(*Grant){
		"host not approved":            func(g *Grant) { g.NetworkHosts = []string{"api.other.example"} },
		"no host approved":             func(g *Grant) { g.NetworkHosts = nil },
		"credential kind not approved": func(g *Grant) { g.CredentialKinds = []string{"other_kind"} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			g := full
			change(&g)
			h, r := newHost(t)
			p := start(t, h, g)
			if _, err := p.Sync(context.Background(), ""); status.Code(err) != codes.PermissionDenied {
				t.Errorf("sync error %v, want PermissionDenied", err)
			}
			if r.calls != 0 || r.count("plugin.scope_denied") != 1 || r.count("nhi.upserted") != 0 {
				t.Errorf("broker calls %d, events %v: want no call and one denial", r.calls, r.types())
			}
		})
	}
}

func TestPlatformCallProblem(t *testing.T) {
	c := asccClaims{NetworkHosts: []string{"api.example.com"}, CredentialKinds: []string{"example_app"}}
	tests := []struct {
		name, kind, method, url string
		allowed                 bool
	}{
		{"approved host", "example_app", "GET", "https://api.example.com/apps?page=2", true},
		{"plain http", "example_app", "GET", "http://api.example.com/apps", false},
		{"another host", "example_app", "GET", "https://paste.example/apps", false},
		{"approved host as a prefix", "example_app", "GET", "https://api.example.com.paste.example/", false},
		{"approved host as userinfo", "example_app", "GET", "https://api.example.com@paste.example/", false},
		{"userinfo on the approved host", "example_app", "GET", "https://someone@api.example.com/apps", false},
		{"another host after a fragment", "example_app", "GET", "https://api.example.com#@paste.example/", true},
		{"another port", "example_app", "GET", "https://api.example.com:8443/apps", false},
		{"upper case host", "example_app", "GET", "https://API.EXAMPLE.COM/apps", false},
		{"no scheme", "example_app", "GET", "api.example.com/apps", false},
		{"not a url", "example_app", "GET", "https://%zz", false},
		{"a write", "example_app", "POST", "https://api.example.com/apps", false},
		{"lower case method", "example_app", "get", "https://api.example.com/apps", false},
		{"another credential kind", "admin_token", "GET", "https://api.example.com/apps", false},
		{"no credential kind", "", "GET", "https://api.example.com/apps", false},
	}
	for _, tt := range tests {
		clean, reason := platformCallProblem(c, &pb.PlatformCallRequest{CredentialKind: tt.kind, Method: tt.method, Url: tt.url})
		if (reason == "") != tt.allowed || (clean == "") == tt.allowed {
			t.Errorf("%s: url %q, reason %q, allowed should be %v", tt.name, clean, reason, tt.allowed)
		}
	}
	// A grant that approves an empty host or an empty credential kind approves nothing.
	empty := asccClaims{NetworkHosts: []string{""}, CredentialKinds: []string{"", "example_app"}}
	for _, u := range []string{"https:///apps", "https:paste.example/x", "https:x", "https://"} {
		if _, reason := platformCallProblem(empty, &pb.PlatformCallRequest{CredentialKind: "example_app", Method: "GET", Url: u}); reason == "" {
			t.Errorf("an empty approved host allowed %s", u)
		}
	}
	if _, reason := platformCallProblem(empty, &pb.PlatformCallRequest{Method: "GET", Url: "https://api.example.com/"}); reason == "" {
		t.Error("an empty credential kind was allowed")
	}
}

func TestPluginThatIsNotTheApprovedOneIsStopped(t *testing.T) {
	g := full
	g.PluginID = "github-connector" // approved one plugin, started another
	h, r := newHost(t)
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "ascc-ph*"))
	p, err := h.Start(context.Background(), hello(t), g)
	if err == nil {
		p.Stop()
		t.Fatal("the host ran a plugin that described itself as a different one")
	}
	if after, _ := filepath.Glob(filepath.Join(os.TempDir(), "ascc-ph*")); len(after) > len(before) {
		t.Errorf("a socket folder was left behind: %v", after)
	}
	if r.count("nhi.upserted")+r.count("platform.called")+r.calls != 0 {
		t.Errorf("the rejected plugin did something first: %v", r.types())
	}
	if !strings.Contains(err.Error(), "isn't the plugin that was approved") {
		t.Errorf("error %q doesn't say why", err)
	}
	if r.count("plugin.describe_rejected") != 1 || r.count("plugin.started") != 0 {
		t.Errorf("events %v", r.types())
	}
}

func TestDescribeProblem(t *testing.T) {
	ok := func() *pb.DescribeResponse {
		return &pb.DescribeResponse{PluginId: "hello-connector", Type: "connector", ContractVersion: "v1alpha1"}
	}
	if r := describeProblem(ok(), nil, full); r != "" {
		t.Errorf("a matching plugin was refused: %s", r)
	}
	bad := map[string]func() (*pb.DescribeResponse, error){
		"no answer":        func() (*pb.DescribeResponse, error) { return nil, errors.New("gone") },
		"newer contract":   func() (*pb.DescribeResponse, error) { d := ok(); d.ContractVersion = "v1beta1"; return d, nil },
		"no contract":      func() (*pb.DescribeResponse, error) { d := ok(); d.ContractVersion = ""; return d, nil },
		"another type":     func() (*pb.DescribeResponse, error) { d := ok(); d.Type = "detector"; return d, nil },
		"empty everything": func() (*pb.DescribeResponse, error) { return &pb.DescribeResponse{}, nil },
	}
	for name, f := range bad {
		if d, err := f(); describeProblem(d, err, full) == "" {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestStartFailsClosedOnABinaryThatIsNotThere(t *testing.T) {
	h, r := newHost(t)
	if _, err := h.Start(context.Background(), filepath.Join(t.TempDir(), "nothing-here"), full); err == nil {
		t.Fatal("no error for a missing plugin binary")
	}
	if r.count("plugin.started") != 0 {
		t.Errorf("events %v", r.types())
	}
}

// ctxWith is the context of a call arriving with this authorization metadata.
func ctxWith(auth ...string) context.Context {
	md := metadata.MD{}
	for _, a := range auth {
		md.Append("authorization", a)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestAuthorizeFailsClosed(t *testing.T) {
	h, r := newHost(t)
	p, err := h.newPlugin(full)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.authorize(ctxWith("Bearer "+p.token), pb.Host_UpsertNhi_FullMethodName); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("the token worked before the plugin had described itself: %v", err)
	}
	p.activate()
	other, _ := h.newPlugin(Grant{PluginID: "other-connector", Scopes: full.Scopes})
	same, _ := h.newPlugin(full) // same plugin id, another instance
	_, strangerKey, _ := ed25519.GenerateKey(rand.Reader)
	good := claims{Aud: tokenAud, Sub: full.PluginID, Jti: p.jti, Iat: h.now().Unix(), Exp: h.now().Add(time.Minute).Unix(),
		ASCC: asccClaims{ContractVersion: ContractVersion, Scopes: full.Scopes}}
	with := func(change func(*claims)) string {
		c := good
		change(&c)
		return mint(h.key, c)
	}
	parts := strings.Split(p.token, ".")
	algNone := b64.EncodeToString([]byte(`{"alg":"none","typ":"plgcap+jwt"}`)) + "." + parts[1] + "."
	// The same claims under another header, signed with the host's own key: only the header check stops it.
	signed := b64.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`)) + "." + parts[1]
	wrongTyp := signed + "." + b64.EncodeToString(ed25519.Sign(h.key, []byte(signed)))

	upsert := pb.Host_UpsertNhi_FullMethodName
	if _, err := p.authorize(ctxWith("Bearer "+p.token), upsert); err != nil {
		t.Fatalf("the plugin's own token was refused: %v", err)
	}
	if _, err := p.authorize(ctxWith("Bearer "+mint(h.key, good)), upsert); err != nil {
		t.Fatalf("the baseline token was refused: %v", err)
	}

	tests := []struct {
		name, method string
		ctx          context.Context
		want         codes.Code
	}{
		{"no metadata", upsert, context.Background(), codes.Unauthenticated},
		{"no token", upsert, ctxWith(), codes.Unauthenticated},
		{"two tokens", upsert, ctxWith("Bearer "+p.token, "Bearer "+p.token), codes.Unauthenticated},
		{"not a bearer token", upsert, ctxWith(p.token), codes.Unauthenticated},
		{"garbage", upsert, ctxWith("Bearer a.b.c"), codes.Unauthenticated},
		{"alg none", upsert, ctxWith("Bearer " + algNone), codes.Unauthenticated},
		{"another token type, properly signed", upsert, ctxWith("Bearer " + wrongTyp), codes.Unauthenticated},
		{"signed by another key", upsert, ctxWith("Bearer " + mint(strangerKey, good)), codes.Unauthenticated},
		{"claims swapped under the signature", upsert, ctxWith("Bearer " + parts[0] + "." + strings.Split(other.token, ".")[1] + "." + parts[2]), codes.Unauthenticated},
		{"another plugin's token", upsert, ctxWith("Bearer " + other.token), codes.Unauthenticated},
		{"another instance's token", upsert, ctxWith("Bearer " + same.token), codes.Unauthenticated},
		{"expired", upsert, ctxWith("Bearer " + with(func(c *claims) { c.Exp = h.now().Unix() })), codes.Unauthenticated},
		{"not valid yet", upsert, ctxWith("Bearer " + with(func(c *claims) { c.Iat = h.now().Add(time.Hour).Unix() })), codes.Unauthenticated},
		{"for another audience", upsert, ctxWith("Bearer " + with(func(c *claims) { c.Aud = "ascc-gateway" })), codes.Unauthenticated},
		{"another contract version", upsert, ctxWith("Bearer " + with(func(c *claims) { c.ASCC.ContractVersion = "v1" })), codes.Unauthenticated},
		{"scope missing", upsert, ctxWith("Bearer " + with(func(c *claims) { c.ASCC.Scopes = []string{scopeFindingWrite} })), codes.PermissionDenied},
		{"no scopes", upsert, ctxWith("Bearer " + with(func(c *claims) { c.ASCC.Scopes = nil })), codes.PermissionDenied},
		{"a method that isn't mapped", "/ascc.plugin.v1alpha1.Host/ReadSecret", ctxWith("Bearer " + p.token), codes.PermissionDenied},
		{"a host-to-plugin method", pb.Connector_Sync_FullMethodName, ctxWith("Bearer " + p.token), codes.PermissionDenied},
	}
	for _, tt := range tests {
		before := len(r.types())
		if _, err := p.authorize(tt.ctx, tt.method); status.Code(err) != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, err, tt.want)
		}
		wantEvent := map[codes.Code]string{codes.Unauthenticated: "plugin.token_rejected", codes.PermissionDenied: "plugin.scope_denied"}[tt.want]
		if got := r.types(); len(got) != before+1 || got[before] != wantEvent {
			t.Errorf("%s: recorded %v, want one %s", tt.name, got[before:], wantEvent)
		}
	}
	if err := refuseStream(nil, nil, nil, nil); status.Code(err) != codes.PermissionDenied {
		t.Errorf("a streaming call wasn't refused: %v", err)
	}
}

func TestNewRefusesAHostWithNowhereToRecord(t *testing.T) {
	r := &recorder{}
	if _, err := New(nil, r.broker); err == nil {
		t.Error("no error without an event sink")
	}
	if _, err := New(r.event, nil); err == nil {
		t.Error("no error without a broker")
	}
}

// A panic while serving one plugin call fails that call and nothing else.
func TestPanicInACallIsThatCallsError(t *testing.T) {
	h, _ := newHost(t)
	p, _ := h.newPlugin(full)
	p.activate()
	info := &grpc.UnaryServerInfo{FullMethod: pb.Host_UpsertNhi_FullMethodName}
	_, err := p.intercept(ctxWith("Bearer "+p.token), nil, info, func(context.Context, any) (any, error) { panic("boom") })
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "boom") {
		t.Errorf("got %v, want an Internal error that doesn't repeat the panic", err)
	}
}

func TestNoEventOrErrorRepeatsTheToken(t *testing.T) {
	h, r := newHost(t)
	p, _ := h.newPlugin(full)
	p.activate()
	forged := p.token + "x"
	_, err := p.authorize(ctxWith("Bearer "+forged), pb.Host_UpsertNhi_FullMethodName)
	if err == nil || strings.Contains(err.Error(), forged) || strings.Contains(err.Error(), p.token) {
		t.Errorf("error: %v", err)
	}
	for _, e := range r.events {
		if strings.Contains(e.Detail, p.token) {
			t.Errorf("event repeats the token: %+v", e)
		}
	}
}

func TestStoppedPluginsTokenIsDead(t *testing.T) {
	h, r := newHost(t)
	p := start(t, h, full)
	ctx := ctxWith("Bearer " + p.token)
	if _, err := p.authorize(ctx, pb.Host_UpsertNhi_FullMethodName); err != nil {
		t.Fatalf("live token refused: %v", err)
	}
	p.Stop()
	p.Stop() // twice is fine
	if _, err := p.authorize(ctx, pb.Host_UpsertNhi_FullMethodName); status.Code(err) != codes.Unauthenticated {
		t.Errorf("a stopped plugin's token still works: %v", err)
	}
	if r.count("plugin.stopped") != 1 {
		t.Errorf("events %v, want one plugin.stopped", r.types())
	}
	if _, err := os.Stat(p.dir); !os.IsNotExist(err) {
		t.Errorf("the socket folder is still there: %v", err)
	}
}

// A plugin that dies takes nothing with it: its token dies too and the host runs the next one.
func TestPluginCrashLeavesTheHostRunning(t *testing.T) {
	h, r := newHost(t)
	p := start(t, h, full)
	p.cmd.Process.Kill()
	for i := 0; r.count("plugin.exited") == 0; i++ {
		if i > 500 {
			t.Fatal("no plugin.exited event after the process was killed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := p.authorize(ctxWith("Bearer "+p.token), pb.Host_UpsertNhi_FullMethodName); status.Code(err) != codes.Unauthenticated {
		t.Errorf("a dead plugin's token still works: %v", err)
	}
	if _, err := p.Sync(context.Background(), ""); err == nil {
		t.Error("sync on a dead plugin returned no error")
	}
	next := start(t, h, full)
	if res, err := next.Sync(context.Background(), ""); err != nil || res.GetUpserted() != 2 {
		t.Errorf("the host couldn't run another plugin: %v, %v", res, err)
	}
}

func TestChildEnvCarriesNothingOfTheHosts(t *testing.T) {
	t.Setenv("ASCC_TEST_SECRET", "not-for-plugins")
	h, _ := newHost(t)
	env := start(t, h, full).cmd.Env // what the process was really given
	if len(env) == 0 {
		t.Fatal("the plugin was started with the host's environment")
	}
	allowed := map[string]bool{"ASCC_HOST_SOCKET": true, "ASCC_PLUGIN_SOCKET": true, "SystemRoot": runtime.GOOS == "windows"}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !allowed[name] {
			t.Errorf("the plugin's environment has %s", name)
		}
	}
}

// BenchmarkHostRoundTrip measures what the host adds to one plugin call (NFR-20: 2 ms p95 or less):
// the socket, gRPC, the token check and the event.
func BenchmarkHostRoundTrip(b *testing.B) {
	h, err := New(func(Event) {}, func(context.Context, string, string, string) (int, []byte, error) { return 200, nil, nil })
	if err != nil {
		b.Fatal(err)
	}
	p, _ := h.newPlugin(full)
	p.activate()
	dir, err := os.MkdirTemp("", "ascc-ph")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "h.sock")
	lis, err := net.Listen("unix", sock)
	if err != nil {
		b.Fatal(err)
	}
	srv := grpc.NewServer(grpc.UnaryInterceptor(p.intercept))
	pb.RegisterHostServer(srv, &hostAPI{p: p})
	go srv.Serve(lis)
	defer srv.Stop()
	conn, err := grpc.NewClient("passthrough:///host", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		}))
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewHostClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+p.token)
	req := &pb.UpsertNhiRequest{Nhi: &pb.Nhi{ExternalId: "app-1", Kind: "oauth_app"}}
	if _, err := client.UpsertNhi(ctx, req); err != nil {
		b.Fatal(err)
	}
	took := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		if _, err := client.UpsertNhi(ctx, req); err != nil {
			b.Fatal(err)
		}
		took = append(took, time.Since(t0))
	}
	b.StopTimer()
	sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
	b.ReportMetric(float64(took[len(took)*95/100].Microseconds()), "p95-µs")
}
