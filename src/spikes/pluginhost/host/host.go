// Package host is the TKT-031 prototype of the plugin host in ADR-15: it runs a plugin as a separate
// process, gives it a capability token, and is the only thing the plugin can talk to. Every call the
// plugin makes is checked against the token, and the host writes every event, with the plugin's id.
//
// It is a spike. It doesn't confine the plugin's network or resources, restart it, or read a manifest.
package host

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/ai-automation-tools/ai-security-command-center/src/spikes/pluginhost/pluginv1alpha1"
)

// ContractVersion is the only contract this host runs.
const ContractVersion = "v1alpha1"

const (
	scopeNHIWrite     = "nhi:write@self"
	scopeFindingWrite = "findings:write@self"
)

// methodScope is every call a plugin may make and the scope it needs. A method that isn't here is
// refused, so a new RPC is closed until someone decides what it needs. PlatformCall needs no scope:
// its handler checks the approved hosts and credential kinds.
var methodScope = map[string]string{
	pb.Host_UpsertNhi_FullMethodName:     scopeNHIWrite,
	pb.Host_ReportFinding_FullMethodName: scopeFindingWrite,
	pb.Host_PlatformCall_FullMethodName:  "",
}

// Grant is what was approved for one plugin: the copy of its manifest's capabilities that the install
// gate (TKT-033) will produce.
type Grant struct {
	PluginID        string
	ConnectorID     string
	Scopes          []string
	NetworkHosts    []string
	CredentialKinds []string
}

// Event is what the host records. A plugin never writes one, so it can't forge who did what.
type Event struct {
	Type            string
	PluginID        string
	ConnectorID     string
	ContractVersion string
	Detail          string // a method, a scope or a reason; never a value a plugin sent as content
	At              time.Time
}

// Broker makes a platform call with a credential the plugin never sees. The real one is Slice 7.
type Broker func(ctx context.Context, credentialKind, method, rawURL string) (status int, body []byte, err error)

// Host runs plugins.
type Host struct {
	events func(Event)
	broker Broker
	now    func() time.Time
	key    ed25519.PrivateKey
}

// New makes a host. events and broker must be safe to call from several goroutines.
func New(events func(Event), broker Broker) (*Host, error) {
	if events == nil || broker == nil {
		return nil, errors.New("plugin host: it needs somewhere to record events and a broker")
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("plugin host: no signing key: %w", err)
	}
	return &Host{events: events, broker: broker, now: time.Now, key: key}, nil
}

// Plugin is one running plugin.
type Plugin struct {
	host  *Host
	grant Grant
	token string
	jti   string

	mu   sync.Mutex
	live bool // the token works only while this is true: after Describe passed, before stop or exit
	dead bool // stopped or exited; never live again

	dir  string
	cmd  *exec.Cmd
	srv  *grpc.Server
	conn *grpc.ClientConn
	stop sync.Once
	done chan struct{} // closed when the process has exited
}

// newPlugin mints the plugin's token. The token is refused until activate: a process holding it can't
// use it before it has described itself as the plugin that was approved.
func (h *Host) newPlugin(g Grant) (*Plugin, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("plugin host: no token id: %w", err)
	}
	p := &Plugin{host: h, grant: g, jti: "cap_" + hex.EncodeToString(id)}
	now := h.now()
	p.token = mint(h.key, claims{
		Iss: "ascc-plugin-host", Aud: tokenAud, Sub: g.PluginID, Jti: p.jti,
		Iat: now.Unix(), Exp: now.Add(tokenTTL).Unix(),
		ASCC: asccClaims{
			ConnectorID: g.ConnectorID, ContractVersion: ContractVersion,
			Scopes: g.Scopes, NetworkHosts: g.NetworkHosts, CredentialKinds: g.CredentialKinds,
		},
	})
	return p, nil
}

// Start runs the plugin binary and returns once the plugin has described itself as the plugin that was
// approved. Anything else stops it.
func (h *Host) Start(ctx context.Context, binary string, g Grant) (*Plugin, error) {
	p, err := h.newPlugin(g)
	if err != nil {
		return nil, err
	}
	// Two sockets in a folder only this user can open: one the plugin calls, one it serves.
	if p.dir, err = os.MkdirTemp("", "ascc-ph"); err != nil {
		return nil, fmt.Errorf("plugin host: %w", err)
	}
	hostSock, pluginSock := filepath.Join(p.dir, "h.sock"), filepath.Join(p.dir, "p.sock")
	lis, err := net.Listen("unix", hostSock)
	if err != nil {
		os.RemoveAll(p.dir)
		return nil, fmt.Errorf("plugin host: %w", err)
	}
	p.srv = grpc.NewServer(grpc.UnaryInterceptor(p.intercept), grpc.StreamInterceptor(refuseStream))
	pb.RegisterHostServer(p.srv, &hostAPI{p: p})
	go p.srv.Serve(lis)

	p.cmd = exec.Command(binary)
	p.cmd.Env = childEnv(hostSock, pluginSock)
	p.cmd.Stdin = strings.NewReader(p.token + "\n") // not in the environment, where other processes can read it
	if err := p.cmd.Start(); err != nil {
		p.kill()
		p.shutdown()
		return nil, fmt.Errorf("plugin host: can't start %s: %w", g.PluginID, err)
	}
	p.done = make(chan struct{})
	go func() {
		p.cmd.Wait()
		close(p.done)
		if p.kill() {
			p.emit("plugin.exited", "")
		}
	}()

	p.conn, err = grpc.NewClient("passthrough:///plugin",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", pluginSock)
		}))
	if err != nil {
		p.Stop()
		return nil, fmt.Errorf("plugin host: %w", err)
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	d, err := pb.NewPluginClient(p.conn).Describe(dctx, &pb.DescribeRequest{}, grpc.WaitForReady(true))
	if reason := describeProblem(d, err, g); reason != "" {
		p.emit("plugin.describe_rejected", reason)
		p.Stop()
		return nil, fmt.Errorf("plugin host: %s was stopped: %s", g.PluginID, reason)
	}
	if !p.activate() {
		p.Stop()
		return nil, fmt.Errorf("plugin host: %s exited as it started", g.PluginID)
	}
	p.emit("plugin.started", "")
	return p, nil
}

func describeProblem(d *pb.DescribeResponse, err error, g Grant) string {
	switch {
	case err != nil:
		return "it didn't describe itself"
	case d.GetContractVersion() != ContractVersion:
		return "it was built for another contract version"
	case d.GetPluginId() != g.PluginID:
		return "it isn't the plugin that was approved"
	case d.GetType() != "connector":
		return "it isn't a connector"
	}
	return ""
}

// childEnv is the plugin's whole environment: the two socket paths and, on Windows, the one variable
// sockets need. Nothing of the host's own environment reaches a plugin.
func childEnv(hostSock, pluginSock string) []string {
	env := []string{"ASCC_HOST_SOCKET=" + hostSock, "ASCC_PLUGIN_SOCKET=" + pluginSock}
	if runtime.GOOS == "windows" {
		env = append(env, "SystemRoot="+os.Getenv("SystemRoot"))
	}
	return env
}

// activate makes the token work, unless the plugin is already stopped or gone.
func (p *Plugin) activate() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.live = !p.dead
	return p.live
}

// kill makes the token dead for good. It reports whether this call was the one that did it.
func (p *Plugin) kill() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	was := !p.dead
	p.live, p.dead = false, true
	return was
}

func (p *Plugin) shutdown() {
	p.stop.Do(func() {
		if p.conn != nil {
			p.conn.Close()
		}
		if p.done != nil {
			p.cmd.Process.Kill()
			<-p.done // so the binary and the socket folder aren't still in use when they are removed
		}
		if p.srv != nil {
			p.srv.Stop()
		}
		os.RemoveAll(p.dir)
	})
}

// Stop revokes the plugin's token, then ends its process (the start of disable_plugin, FR-PLG-07).
func (p *Plugin) Stop() {
	if p.kill() {
		p.emit("plugin.stopped", "")
	}
	p.shutdown()
}

// Health asks the plugin whether it can work.
func (p *Plugin) Health(ctx context.Context) (*pb.HealthResponse, error) {
	return pb.NewPluginClient(p.conn).Health(ctx, &pb.HealthRequest{})
}

// Sync asks the connector to read its platform from the cursor on.
func (p *Plugin) Sync(ctx context.Context, cursor string) (*pb.SyncResponse, error) {
	return pb.NewConnectorClient(p.conn).Sync(ctx, &pb.SyncRequest{Cursor: cursor})
}

func (p *Plugin) emit(typ, detail string) {
	p.host.events(Event{
		Type: typ, PluginID: p.grant.PluginID, ConnectorID: p.grant.ConnectorID,
		ContractVersion: ContractVersion, Detail: detail, At: p.host.now(),
	})
}

type claimsKey struct{}

// refuseStream closes every streaming call: v1alpha1 has none, and one added later must not arrive
// unchecked.
func refuseStream(any, grpc.ServerStream, *grpc.StreamServerInfo, grpc.StreamHandler) error {
	return status.Error(codes.PermissionDenied, "not a method a plugin may call")
}

func (p *Plugin) intercept(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (_ any, err error) {
	// A panic in a handler, the broker or the event sink is this call's failure, not the host's.
	defer func() {
		if recover() != nil {
			err = status.Error(codes.Internal, "the plugin host couldn't do that")
		}
	}()
	c, err := p.authorize(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}
	return next(context.WithValue(ctx, claimsKey{}, c), req)
}

// authorize is the scope check on every call a plugin makes. It refuses unless the method is known,
// the token is this plugin's live token, and the token carries the scope the method needs.
func (p *Plugin) authorize(ctx context.Context, method string) (claims, error) {
	need, known := methodScope[method]
	if !known {
		p.emit("plugin.scope_denied", "not a method a plugin may call")
		return claims{}, status.Error(codes.PermissionDenied, "not a method a plugin may call")
	}
	c, err := p.tokenClaims(ctx)
	if err != nil {
		p.emit("plugin.token_rejected", method+": "+err.Error())
		return claims{}, status.Error(codes.Unauthenticated, "capability token refused: "+err.Error())
	}
	if need != "" && !slices.Contains(c.ASCC.Scopes, need) {
		p.emit("plugin.scope_denied", method+": needs "+need)
		return claims{}, status.Error(codes.PermissionDenied, "this plugin wasn't approved for "+need)
	}
	return c, nil
}

func (p *Plugin) tokenClaims(ctx context.Context) (claims, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	auth := md.Get("authorization")
	if len(auth) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
		return claims{}, errors.New("no token")
	}
	c, err := verify(p.host.key.Public().(ed25519.PublicKey), strings.TrimPrefix(auth[0], "Bearer "), p.host.now())
	if err != nil {
		return claims{}, err
	}
	p.mu.Lock()
	live := p.live
	p.mu.Unlock()
	switch {
	case c.Sub != p.grant.PluginID || c.Jti != p.jti:
		return claims{}, errors.New("another plugin's token")
	case !live:
		return claims{}, errors.New("not in force")
	case c.ASCC.ContractVersion != ContractVersion:
		return claims{}, errors.New("wrong contract version")
	}
	return c, nil
}

// hostAPI is what one plugin can ask for. It takes who is asking from the token, never from the request.
type hostAPI struct {
	pb.UnimplementedHostServer
	p *Plugin
}

func (a *hostAPI) UpsertNhi(ctx context.Context, r *pb.UpsertNhiRequest) (*pb.UpsertNhiResponse, error) {
	if r.GetNhi().GetExternalId() == "" {
		return nil, status.Error(codes.InvalidArgument, "an identity needs an external_id")
	}
	a.p.emit("nhi.upserted", "")
	return &pb.UpsertNhiResponse{}, nil
}

func (a *hostAPI) ReportFinding(ctx context.Context, r *pb.ReportFindingRequest) (*pb.ReportFindingResponse, error) {
	if r.GetRuleId() == "" || r.GetSubject() == "" {
		return nil, status.Error(codes.InvalidArgument, "a finding needs a rule_id and a subject")
	}
	a.p.emit("finding.reported", "")
	return &pb.ReportFindingResponse{}, nil
}

func (a *hostAPI) PlatformCall(ctx context.Context, r *pb.PlatformCallRequest) (*pb.PlatformCallResponse, error) {
	c, ok := ctx.Value(claimsKey{}).(claims)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "capability token refused: no token")
	}
	clean, reason := platformCallProblem(c.ASCC, r)
	if reason != "" {
		a.p.emit("plugin.scope_denied", pb.Host_PlatformCall_FullMethodName+": "+reason)
		return nil, status.Error(codes.PermissionDenied, reason)
	}
	// The broker gets the URL as it was checked, not as it was sent.
	code, body, err := a.p.host.broker(ctx, r.GetCredentialKind(), "GET", clean)
	if err != nil {
		a.p.emit("platform.call_failed", "")
		return nil, status.Error(codes.Unavailable, "the platform call failed")
	}
	a.p.emit("platform.called", "")
	return &pb.PlatformCallResponse{Status: int32(code), Body: body}, nil
}

// platformCallProblem returns the URL to call, or why the call is outside what was approved. It checks
// the host, not the path: what a plugin may read on an approved host is the platform credential's scope.
func platformCallProblem(c asccClaims, r *pb.PlatformCallRequest) (cleanURL, reason string) {
	if r.GetMethod() != "GET" {
		return "", "only GET: connectors are read-only"
	}
	if r.GetCredentialKind() == "" || !slices.Contains(c.CredentialKinds, r.GetCredentialKind()) {
		return "", "this plugin wasn't approved for that kind of credential"
	}
	u, err := url.Parse(r.GetUrl())
	// u.Host, not u.Hostname(): a port that wasn't approved is a different place.
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Host == "" || !slices.Contains(c.NetworkHosts, u.Host) {
		return "", "this plugin wasn't approved for that host"
	}
	return u.String(), ""
}
