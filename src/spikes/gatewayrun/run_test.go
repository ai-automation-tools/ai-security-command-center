package gatewayrun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	api "github.com/ai-automation-tools/ai-security-command-center/src/spikes/gatewayrun/extmcp"
)

// gatewayEnv names the agentgateway binary to run. Without it the run is skipped: the binary is a
// download, and this repo doesn't fetch one behind a developer's back.
const gatewayEnv = "ASCC_AGENTGATEWAY"

// TestGatewayRun runs agentgateway between the stub upstream and the stub policy service and checks the
// six results the gateway report asks for. Each one is pinned to what v1.7.0 does, so a later release
// that behaves differently fails here and the report gets reread.
func TestGatewayRun(t *testing.T) {
	binary := os.Getenv(gatewayEnv)
	if binary == "" {
		t.Skipf("%s is not set; see README.md for the release to download", gatewayEnv)
	}

	upstream := NewUpstream()
	upstreamAddr := serveHTTP(t, upstream)
	stub := NewStub()
	stubAddr, stopStub := serveStub(t, stub)

	guarded := startGateway(t, binary, upstreamAddr, stubAddr)
	bare := startGateway(t, binary, upstreamAddr, "")

	t.Run("1 a deny reaches the client with its reason and not the upstream", func(t *testing.T) {
		c := connect(t, guarded, "agent-dev")
		if _, rpcErr := c.call(t, "tools/call", map[string]any{"name": "read_file"}); rpcErr != nil {
			t.Fatalf("read_file should pass: %+v", rpcErr)
		}
		result, rpcErr := c.call(t, "tools/call", map[string]any{"name": "delete_repo"})
		if reason := refusal(result, rpcErr); !strings.Contains(reason, "forbid-delete-repo") {
			t.Fatalf("delete_repo: want a deny carrying the policy's reason, got result %s, error %+v", result, rpcErr)
		}
		if n := upstream.Calls("delete_repo"); n != 0 {
			t.Fatalf("the denied call reached the upstream %d times", n)
		}
		// What v1.7.0 does: a deny on the way in isn't a JSON-RPC error. It is a tool result marked
		// isError with the reason as its text, so the model reads the reason like any tool failure.
		if rpcErr != nil || !strings.Contains(string(result), `"isError":true`) {
			t.Errorf("want the deny as an isError tool result, got result %s, error %+v", result, rpcErr)
		}
		t.Logf("deny as the client sees it: %s", result)
	})

	t.Run("2 tools/list is filtered per identity", func(t *testing.T) {
		dev := toolNames(t, connect(t, guarded, "agent-dev"))
		intern := toolNames(t, connect(t, guarded, "agent-intern"))
		if !slices.Contains(dev, "delete_repo") || slices.Contains(intern, "delete_repo") {
			t.Fatalf("want delete_repo for agent-dev only; dev %v, intern %v", dev, intern)
		}
		t.Logf("agent-dev sees %v, agent-intern sees %v", dev, intern)
	})

	t.Run("2b an identity the stub doesn't know gets nothing", func(t *testing.T) {
		for _, agent := range []string{"", "agent-nobody"} {
			status, rpcErr := tryConnect(t, guarded, agent)
			t.Logf("agent %q: HTTP %d, %+v", agent, status, rpcErr)
			if status == http.StatusOK && rpcErr == nil {
				t.Fatalf("agent %q got a session", agent)
			}
		}
	})

	t.Run("5 a changed description is rejected against the pin", func(t *testing.T) {
		c := connect(t, guarded, "agent-dev")
		upstream.SetRugPull(true)
		defer upstream.SetRugPull(false)
		_, rpcErr := c.call(t, "tools/list", nil)
		if rpcErr == nil || !strings.Contains(rpcErr.Message, "pinned description") {
			t.Fatalf("want the list rejected for the pin, got %+v", rpcErr)
		}
		if names := toolNames(t, connect(t, bare, "agent-dev")); !slices.Contains(names, "read_file") {
			t.Fatalf("the bare gateway should pass the poisoned list through, got %v", names)
		}
	})

	t.Run("6 added latency", func(t *testing.T) {
		const n = 2000
		direct := timeCalls(t, &client{url: "http://" + upstreamAddr + "/mcp", agent: "agent-dev"}, n)
		through := timeCalls(t, connect(t, bare, "agent-dev"), n)
		callout := timeCalls(t, connect(t, guarded, "agent-dev"), n)
		t.Logf("%d sequential tools/call, loopback, one machine", n)
		for _, row := range []struct {
			name string
			d    []time.Duration
		}{{"direct to the upstream", direct}, {"gateway, no callout", through}, {"gateway with the callout", callout}} {
			t.Logf("%-26s mean %-10v median %-10v p95 %-10v p99 %v", row.name, mean(row.d), pct(row.d, 50), pct(row.d, 95), pct(row.d, 99))
		}
		t.Logf("added by the gateway and callout: mean %v, median %v, p95 %v, p99 %v (NFR-02: median 5 ms, p95 25 ms, p99 50 ms)",
			mean(callout)-mean(direct), pct(callout, 50)-pct(direct, 50), pct(callout, 95)-pct(direct, 95), pct(callout, 99)-pct(direct, 99))
		t.Logf("of which the callout: mean %v", mean(callout)-mean(through))
		if added := pct(callout, 99) - pct(direct, 99); added > 50*time.Millisecond {
			t.Errorf("p99 added latency %v is over NFR-02's 50 ms", added)
		}
	})

	t.Run("4 a deny while a stream is open", func(t *testing.T) {
		c := connect(t, guarded, "agent-stream")
		began := time.Now()
		ticks := 0
		result, rpcErr, err := c.stream(t, "tools/call", map[string]any{"name": "slow_count"}, func(rpcMessage) {
			if ticks++; ticks == 3 {
				stub.Suspend("agent-stream")
			}
		})
		held := time.Since(began).Round(100 * time.Millisecond)
		t.Logf("suspended after 3 of %d ticks; the stream then carried %d more and ended after %v", slowTicks, ticks-3, held)
		t.Logf("stream ended with: result %s, error %+v, transport %v; upstream saw %d cancelled",
			result, rpcErr, err, upstream.SlowCancelled.Load())

		// What v1.7.0 does: the stream is not cut. Every tick still arrives; only the final result is
		// swapped for the deny, because the callout runs per request and per response and at no other time.
		if ticks != slowTicks {
			t.Errorf("want all %d ticks delivered after the deny (the stream isn't cut), got %d", slowTicks, ticks)
		}
		if rpcErr == nil || !strings.Contains(rpcErr.Message, "suspended") {
			t.Errorf("want the final result replaced by the deny, got result %s, error %+v", result, rpcErr)
		}
		// And the next call from the same agent is refused before it reaches the upstream.
		before := upstream.Calls("read_file")
		result, rpcErr = c.call(t, "tools/call", map[string]any{"name": "read_file"})
		if refusal(result, rpcErr) == "" || upstream.Calls("read_file") != before {
			t.Errorf("a call after the suspension went through: result %s, error %+v", result, rpcErr)
		}
	})

	// Last, because it takes the policy service away for good.
	t.Run("3 with the stub down a call is refused, not passed", func(t *testing.T) {
		c := connect(t, guarded, "agent-dev")
		before := upstream.Calls("read_file")
		stopStub()
		result, rpcErr := c.call(t, "tools/call", map[string]any{"name": "read_file"})
		if refusal(result, rpcErr) == "" {
			t.Fatalf("the call passed with the policy service down: %s", result)
		}
		if n := upstream.Calls("read_file") - before; n != 0 {
			t.Fatalf("the upstream was called %d times with the policy service down", n)
		}
		t.Logf("refused with: result %s, error %+v", result, rpcErr)
	})

	if stub.Requests.Load() == 0 || stub.Responses.Load() == 0 {
		t.Fatalf("the gateway never called the stub (%d requests, %d responses)", stub.Requests.Load(), stub.Responses.Load())
	}
}

// --- the three processes ---

func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func serveHTTP(t *testing.T, h http.Handler) string {
	l := listen(t)
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return l.Addr().String()
}

func serveStub(t *testing.T, s *Stub) (addr string, stop func()) {
	l := listen(t)
	srv := grpc.NewServer()
	api.RegisterExtMcpServer(srv, s)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	return l.Addr().String(), srv.Stop
}

// freePort asks the kernel for a port and gives it back. Another process could take it before the
// gateway does; the gateway then fails to start and the test says so.
func freePort(t *testing.T) int {
	l := listen(t)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startGateway runs agentgateway in front of the upstream and returns its MCP URL. With stubAddr set,
// every tools/call and tools/list goes to the policy service on the way in and on the way out, and the
// failure mode is left at its default so the run tests that default. The listener is pinned to loopback:
// left alone, agentgateway listens on every interface.
func startGateway(t *testing.T, binary, upstreamAddr, stubAddr string) string {
	t.Helper()
	port, ready := freePort(t), freePort(t)
	guardrails := ""
	if stubAddr != "" {
		guardrails = fmt.Sprintf(`  policies:
    mcpGuardrails:
      processors:
      - kind: remote
        host: %s
        methods: {"*": full}
        metadata:
          agent: 'default(request.headers["x-agent-id"], "")'
`, stubAddr)
	}
	config := fmt.Sprintf(`config:
  adminAddr: "off"
  statsAddr: "off"
  readinessAddr: 127.0.0.1:%d
gateways:
  local:
    port: %d
    bindAddress: 127.0.0.1
mcp:
  gateways: local
%s  targets:
  - name: stub
    mcp:
      host: http://%s/mcp
`, ready, port, guardrails, upstreamAddr)
	file := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(file, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	cmd := exec.Command(binary, "-f", file)
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("gateway log:\n%s", logs.String())
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz/ready", ready)); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("the gateway wasn't ready in 20 s:\n%s", logs.String())
	return ""
}

// --- a minimal MCP client over Streamable HTTP ---

type client struct {
	url, agent, session string
	nextID              int
}

// tryConnect sends initialize and reports how it went, without failing the test.
func tryConnect(t *testing.T, url, agent string) (status int, rpcErr *rpcError) {
	t.Helper()
	c := &client{url: url, agent: agent}
	_, rpcErr, status, err := c.do(t, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "gatewayrun", "version": "0"},
	}, nil)
	if err != nil {
		t.Fatalf("initialize as %q: %v", agent, err)
	}
	return status, rpcErr
}

func connect(t *testing.T, url, agent string) *client {
	t.Helper()
	c := &client{url: url, agent: agent}
	_, rpcErr, status, err := c.do(t, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "gatewayrun", "version": "0"},
	}, nil)
	if err != nil || rpcErr != nil || status != http.StatusOK {
		t.Fatalf("initialize as %q: HTTP %d, %+v, %v", agent, status, rpcErr, err)
	}
	c.notify(t, "notifications/initialized")
	return c
}

func (c *client) call(t *testing.T, method string, params any) (json.RawMessage, *rpcError) {
	t.Helper()
	result, rpcErr, _, err := c.do(t, method, params, nil)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return result, rpcErr
}

func (c *client) stream(t *testing.T, method string, params any, onNote func(rpcMessage)) (json.RawMessage, *rpcError, error) {
	t.Helper()
	result, rpcErr, _, err := c.do(t, method, params, onNote)
	return result, rpcErr, err
}

func (c *client) notify(t *testing.T, method string) {
	t.Helper()
	resp := c.post(t, rpcMessage{JSONRPC: "2.0", Method: method})
	resp.Body.Close()
}

func (c *client) post(t *testing.T, msg rpcMessage) *http.Response {
	t.Helper()
	body, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	if c.agent != "" {
		req.Header.Set("X-Agent-Id", c.agent)
	}
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", msg.Method, err)
	}
	if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
		c.session = id
	}
	return resp
}

// do sends one request and reads its answer, as plain JSON or off an event stream. A refusal that isn't
// JSON-RPC (a bare HTTP error) comes back as an rpcError holding the status and body, so a caller can
// treat every kind of "no" alike. err is set only when a stream ends with no answer.
func (c *client) do(t *testing.T, method string, params any, onNote func(rpcMessage)) (json.RawMessage, *rpcError, int, error) {
	t.Helper()
	c.nextID++
	id, _ := json.Marshal(c.nextID)
	msg := rpcMessage{JSONRPC: "2.0", ID: id, Method: method}
	if params != nil {
		msg.Params, _ = json.Marshal(params)
	}
	resp := c.post(t, msg)
	defer resp.Body.Close()

	answer := func(raw []byte) (json.RawMessage, *rpcError, bool) {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *rpcError       `json:"error"`
		}
		if json.Unmarshal(raw, &m) != nil {
			return nil, nil, false
		}
		if m.Method != "" {
			if onNote != nil {
				onNote(rpcMessage{Method: m.Method, Params: m.Params})
			}
			return nil, nil, false
		}
		return m.Result, m.Error, m.Result != nil || m.Error != nil
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		lines := bufio.NewScanner(resp.Body)
		lines.Buffer(make([]byte, 1<<20), 1<<20)
		for lines.Scan() {
			if data, ok := strings.CutPrefix(lines.Text(), "data:"); ok {
				if result, rpcErr, done := answer([]byte(strings.TrimSpace(data))); done {
					return result, rpcErr, resp.StatusCode, nil
				}
			}
		}
		return nil, nil, resp.StatusCode, fmt.Errorf("the stream ended with no answer: %v", lines.Err())
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if result, rpcErr, done := answer(raw); done {
		return result, rpcErr, resp.StatusCode, nil
	}
	return nil, &rpcError{Code: resp.StatusCode, Message: strings.TrimSpace(string(raw))}, resp.StatusCode, nil
}

func toolNames(t *testing.T, c *client) []string {
	t.Helper()
	raw, rpcErr := c.call(t, "tools/list", nil)
	if rpcErr != nil {
		t.Fatalf("tools/list as %s: %+v", c.agent, rpcErr)
	}
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func timeCalls(t *testing.T, c *client, n int) []time.Duration {
	t.Helper()
	params := map[string]any{"name": "read_file"}
	for range 50 { // warm the connections
		c.call(t, "tools/call", params)
	}
	took := make([]time.Duration, n)
	for i := range took {
		began := time.Now()
		if _, rpcErr := c.call(t, "tools/call", params); rpcErr != nil {
			t.Fatalf("timed call %d: %+v", i, rpcErr)
		}
		took[i] = time.Since(began)
	}
	slices.Sort(took)
	return took
}

// refusal returns why a call was refused, or "" if it wasn't. agentgateway says no in two shapes: a
// JSON-RPC error, or for a tools/call stopped on the way in, a tool result marked isError.
func refusal(result json.RawMessage, rpcErr *rpcError) string {
	if rpcErr != nil {
		return rpcErr.Message
	}
	var r struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(result, &r) == nil && r.IsError && len(r.Content) > 0 {
		return r.Content[0].Text
	}
	return ""
}

// mean is the one figure a coarse clock can't blur: Windows ticks about every half millisecond, so a
// percentile of sub-millisecond calls there is a multiple of the tick.
func mean(d []time.Duration) time.Duration {
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	return (sum / time.Duration(len(d))).Round(10 * time.Microsecond)
}

// pct reads a percentile off a sorted slice.
func pct(sorted []time.Duration, p int) time.Duration {
	return sorted[min(len(sorted)-1, len(sorted)*p/100)].Round(10 * time.Microsecond)
}
