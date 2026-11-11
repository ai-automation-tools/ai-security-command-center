// ascc-pdp serves the policy decision point over HTTP: the same pdp.Decide the Claude Code hook calls,
// behind POST /pdp/v1/decide (F §6). It is the first platform service, a binary of its own; the ascc
// CLI never imports it and never opens a socket.
//
// Every answer on any path but /healthz is a decision. Anything it can't evaluate (a bad request, a
// registry or policy that doesn't load, a panic, a decision it can't log) is a deny in the body, whatever
// the status code, so an enforcement point that
// reads only the body still fails closed.
//
// It does not authenticate its callers and it writes no audit chain: both arrive with the token service
// and the event pipeline. Until then it listens on loopback unless told otherwise.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

const maxBody = 1 << 20 // 1 MiB: a decision request is a few hundred bytes

// entityRef is how F §6 names a principal or a resource.
type entityRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// request is the F §6 decision request. TenantID, TokenJTI and TraceID are accepted and not yet used:
// this version has one tenant and no token service.
type request struct {
	TenantID  string         `json:"tenant_id"`
	Principal entityRef      `json:"principal"`
	Action    string         `json:"action"`
	Resource  entityRef      `json:"resource"`
	Context   map[string]any `json:"context"`
	TokenJTI  string         `json:"token_jti"`
	TraceID   string         `json:"trace_id"`
}

// response is the F §6 decision response. Obligations is always empty and Degraded always false here:
// this PDP has no obligations yet and never answers from a cache.
type response struct {
	Decision            string   `json:"decision"`
	PolicyVersion       string   `json:"policy_version"`
	DeterminingPolicies []string `json:"determining_policies"`
	Reasons             []string `json:"reasons"`
	Obligations         []string `json:"obligations"`
	Degraded            bool     `json:"degraded"`
}

// noPolicy is the policy version of an answer given without a loaded policy set.
const noPolicy = "none"

func deny(version, reason string) response {
	return response{Decision: "deny", PolicyVersion: version, DeterminingPolicies: []string{}, Reasons: []string{reason}, Obligations: []string{}}
}

// server answers decisions from the registry and policy files on disk. It reads them on every request
// and rebuilds the PDP only when their bytes changed, so `ascc kill`, a registry edit or a policy edit
// takes effect on the next request with no restart, and a file that stops loading or is emptied is a
// deny at once. A policy file that is deleted, or cut off exactly between two policies, still loads
// without the rules it lost: replace these files whole, by rename, and keep them where agents can't write.
type server struct {
	registry, policies string
	log                io.Writer

	mu      sync.Mutex
	version string // hash of the files the cached PDP was built from
	cached  *pdp.PDP
	failed  error // why that version didn't load, if it didn't
}

// current returns the PDP for what is on disk now, and its version.
//
// ponytail: every request reads and hashes every file, about 1 ms at 500 policies and 5,000 entities.
// If that ever matters, watch the files and keep the hash, but then a missed change is a stale allow.
func (s *server) current() (*pdp.PDP, string, error) {
	reg, err := os.ReadFile(s.registry)
	if err != nil {
		return nil, noPolicy, errors.New("the registry can't be read")
	}
	paths, err := filepath.Glob(filepath.Join(s.policies, "*.cedar"))
	if err != nil || len(paths) == 0 {
		return nil, noPolicy, errors.New("there is no policy file to read")
	}
	sort.Strings(paths)
	h := sha256.New()
	fmt.Fprintf(h, "registry %d\n", len(reg))
	h.Write(reg)
	files := map[string][]byte{}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, noPolicy, errors.New("a policy file can't be read")
		}
		name := filepath.Base(p)
		files[name] = src
		fmt.Fprintf(h, "%s %d\n", name, len(src))
		h.Write(src)
	}
	version := "sha256:" + hex.EncodeToString(h.Sum(nil))[:16]

	s.mu.Lock()
	defer s.mu.Unlock()
	if version != s.version {
		s.cached, s.failed = pdp.New(reg, files)
		s.version = version
	}
	if s.failed != nil {
		return nil, version, s.failed
	}
	return s.cached, version, nil
}

func (s *server) decide(w http.ResponseWriter, r *http.Request) {
	status, res := http.StatusOK, response{}
	var sum [sha256.Size]byte
	var req request
	// Whatever goes wrong below, the caller gets a deny.
	defer func() {
		if recover() != nil {
			status, res = http.StatusInternalServerError, deny(noPolicy, "the decision point failed; denied")
		}
		// A decision that can't be written down doesn't go out as an allow.
		if !s.record(req, res, sum) {
			status, res = http.StatusInternalServerError, deny(noPolicy, "the decision couldn't be recorded; denied")
		}
		write(w, status, res)
	}()

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		status, res = http.StatusMethodNotAllowed, deny(noPolicy, "a decision is asked for with POST")
		return
	}
	// A web page can't send this content type to another origin without the browser asking first, so
	// a page open in someone's browser can't use them to question the service.
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		status, res = http.StatusUnsupportedMediaType, deny(noPolicy, "a decision request is sent as application/json")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		status, res = http.StatusRequestEntityTooLarge, deny(noPolicy, "the request couldn't be read or is over 1 MiB")
		return
	}
	sum = sha256.Sum256(body)
	if req, err = parse(body); err != nil {
		req = request{}
		status, res = http.StatusBadRequest, deny(noPolicy, "the request isn't the decision request this service reads")
		return
	}
	p, version, err := s.current()
	if err != nil {
		// The cause can quote a policy or registry file, so it goes to nobody over the wire.
		status, res = http.StatusServiceUnavailable, deny(version, "the registry or the policies can't be loaded; everything is denied until they can")
		return
	}
	// This PDP knows agents and tools. Any other kind of principal or resource has no policy that
	// could permit it, so it is denied here with a reason that says so.
	if req.Principal.Type != "Agent" || req.Resource.Type != "Tool" {
		res = deny(version, "this decision point only knows Agent principals and Tool resources")
		return
	}
	d := p.Decide(pdp.Request{Principal: req.Principal.ID, Action: req.Action, Resource: req.Resource.ID, Context: req.Context})
	ids := d.PolicyIDs
	if ids == nil {
		ids = []string{}
	}
	res = response{Decision: d.Decision, PolicyVersion: version, DeterminingPolicies: ids, Reasons: []string{d.Reason}, Obligations: []string{}}
	if res.Decision != "allow" {
		res.Decision = "deny" // nothing but the engine's own "allow" is an allow
	}
}

// record writes one line per decision: who asked for what and what was answered, with a hash of the
// request and never the request. It is a log, not the audit chain. It reports whether the line was
// written.
func (s *server) record(req request, res response, sum [sha256.Size]byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	line, _ := json.Marshal(map[string]any{
		"at": time.Now().UTC().Format(time.RFC3339Nano), "decision": res.Decision, "policy_version": res.PolicyVersion,
		"principal": clip(req.Principal.ID), "resource": clip(req.Resource.ID), "action": clip(req.Action),
		"determining_policies": res.DeterminingPolicies, "trace_id": clip(req.TraceID),
		"input_sha256": hex.EncodeToString(sum[:]),
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := fmt.Fprintf(s.log, "%s\n", line)
	return err == nil
}

// parse reads a decision request and nothing else. encoding/json would take the last of two keys with
// the same name, merge two "context" objects and match "CONTEXT" to context; an enforcement point or
// a proxy reading the same bytes another way would then see a different request. So a repeated key at
// any depth, a key that isn't spelled exactly, an unknown key and anything after the document are
// all errors.
func parse(body []byte) (request, error) {
	if err := noRepeatedKeys(body); err != nil {
		return request{}, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil || top == nil {
		return request{}, errors.New("not a JSON object")
	}
	var req request
	fields := map[string]any{
		"tenant_id": &req.TenantID, "action": &req.Action, "context": &req.Context,
		"token_jti": &req.TokenJTI, "trace_id": &req.TraceID,
	}
	refs := map[string]*entityRef{"principal": &req.Principal, "resource": &req.Resource}
	for key, raw := range top {
		if ref, ok := refs[key]; ok {
			var parts map[string]json.RawMessage
			if err := json.Unmarshal(raw, &parts); err != nil || parts == nil {
				return request{}, errors.New("a principal and a resource are objects")
			}
			for k, v := range parts {
				dst, ok := map[string]*string{"type": &ref.Type, "id": &ref.ID}[k]
				if !ok || json.Unmarshal(v, dst) != nil {
					return request{}, errors.New("a principal and a resource have a type and an id")
				}
			}
			continue
		}
		dst, ok := fields[key]
		if !ok || json.Unmarshal(raw, dst) != nil {
			return request{}, errors.New("a field this service doesn't read, or one of the wrong type")
		}
	}
	return req, nil
}

// noRepeatedKeys walks one JSON document and refuses a key used twice in the same object, and anything
// but white space after the document.
func noRepeatedKeys(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	var seen []map[string]bool // one entry per open object or array; nil for an array
	key := false               // the next string token is a key
	for {
		tok, err := dec.Token()
		if err == io.EOF && len(seen) == 0 {
			return nil
		}
		if err != nil {
			return errors.New("not one JSON document")
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				seen, key = append(seen, map[string]bool{}), true
				continue
			case '[':
				seen, key = append(seen, nil), false
				continue
			}
			seen = seen[:len(seen)-1]
		case string:
			if key {
				if seen[len(seen)-1][t] {
					return errors.New("a key is used twice")
				}
				seen[len(seen)-1][t], key = true, false
				continue
			}
		}
		// A value just ended. A second top-level value is as bad as a repeated key.
		if len(seen) == 0 {
			if _, err := dec.Token(); err != io.EOF {
				return errors.New("not one JSON document")
			}
			return nil
		}
		key = seen[len(seen)-1] != nil
	}
}

// clip keeps a caller-supplied name from filling the log.
func clip(v string) string {
	if len(v) > 200 {
		return v[:200]
	}
	return v
}

// health is 200 only while the registry and policies load: the same condition under which a decision
// can be anything but a deny.
func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if _, _, err := s.current(); err != nil {
		http.Error(w, "not ready: the registry or the policies can't be loaded", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

func write(w http.ResponseWriter, status int, res response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // a decision is for this request only
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(res)
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/pdp/v1/decide", s.decide)
	mux.HandleFunc("/healthz", s.health)
	// Any other path, a near miss on the decide path included, gets a deny and not a bare 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusNotFound, deny(noPolicy, "decisions are asked for at /pdp/v1/decide"))
	})
	return mux
}

// checkListen refuses an address other processes on the network could reach, unless asked: the
// service has no caller authentication yet.
func checkListen(addr string, allowRemote bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("--listen %q isn't host:port", addr)
	}
	// A literal address only: "localhost" is whatever the hosts file says it is.
	if ip := net.ParseIP(host); (ip != nil && ip.IsLoopback()) || allowRemote {
		return nil
	}
	return fmt.Errorf("--listen %s is reachable from other machines and this service doesn't authenticate callers; use a loopback address such as 127.0.0.1, or pass --allow-remote on a private network", addr)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ascc-pdp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", "127.0.0.1:8181", "address to listen on")
	allowRemote := fs.Bool("allow-remote", false, "allow a --listen address other machines can reach")
	registry := fs.String("registry", "ascc.registry.json", "agent registry file")
	policies := fs.String("policies", "policies", "directory of *.cedar policy files")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: ascc-pdp [--listen host:port] [--allow-remote] [--registry file] [--policies dir]")
		return 2
	}
	if err := checkListen(*listen, *allowRemote); err != nil {
		fmt.Fprintln(stderr, "ascc-pdp:", err)
		return 2
	}
	s := &server{registry: *registry, policies: *policies, log: stdout}
	// Start even if the files don't load: the service then denies everything and says why on /healthz,
	// which is safer than leaving enforcement points with nothing to ask.
	if _, _, err := s.current(); err != nil {
		fmt.Fprintf(stderr, "ascc-pdp: denying everything until this is fixed: %v\n", err)
	}
	lis, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(stderr, "ascc-pdp: can't listen on %s\n", *listen)
		return 2
	}
	srv := &http.Server{
		Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shut)
	}()
	fmt.Fprintf(stderr, "ascc-pdp: listening on %s\n", lis.Addr())
	if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "ascc-pdp: stopped:", err)
		return 2
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
