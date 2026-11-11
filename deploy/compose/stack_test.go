// Package compose_test checks the development stack in this folder: the rules compose.yaml keeps, on every
// run, and the running stack itself when ASCC_COMPOSE_SMOKE=1.
package compose_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func needCompose(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		if os.Getenv("CI") != "" && runtime.GOOS == "linux" {
			t.Fatal("docker compose isn't available, and CI on Linux must run this test")
		}
		t.Skip("docker compose isn't available")
	}
}

var (
	secretName = regexp.MustCompile(`(?i)(PASSWORD|PASSWD|PGPASS|SECRET|TOKEN|KEY|CREDENTIAL)`)
	// A URL with a password in it, as in postgres://user:pw@host.
	urlSecret = regexp.MustCompile(`://[^/\s:@"]+:[^@\s"]+@`)
	copyFrom  = regexp.MustCompile(`(?i)--from=(\S+)`)
)

// unpinned returns the images a Dockerfile takes from a registry without a digest.
func unpinned(dockerfile string) []string {
	var bad []string
	stages := map[string]bool{"scratch": true}
	check := func(image string) {
		if _, err := fmt.Sscanf(image, "%d", new(int)); err == nil || stages[strings.ToLower(image)] {
			return // an earlier stage, by number or by name
		}
		if !strings.Contains(image, "@sha256:") {
			bad = append(bad, image)
		}
	}
	for _, line := range strings.Split(dockerfile, "\n") {
		f := strings.Fields(line)
		if len(f) > 1 && strings.EqualFold(f[0], "FROM") {
			f = f[1:]
			for len(f) > 1 && strings.HasPrefix(f[0], "--") {
				f = f[1:]
			}
			check(f[0])
			if len(f) == 3 && strings.EqualFold(f[1], "AS") {
				stages[strings.ToLower(f[2])] = true
			}
			continue
		}
		for _, m := range copyFrom.FindAllStringSubmatch(line, -1) {
			check(m[1])
		}
	}
	return bad
}

// checkConfig returns every rule the resolved Compose document breaks. dockerfile reads a Dockerfile a
// service builds from, given its build context and name.
func checkConfig(doc []byte, dockerfile func(context, name string) ([]byte, error)) []string {
	var bad []string
	fail := func(format string, args ...any) { bad = append(bad, fmt.Sprintf(format, args...)) }
	var file struct {
		Services map[string]struct {
			Image       string   `json:"image"`
			Privileged  bool     `json:"privileged"`
			NetworkMode string   `json:"network_mode"`
			Pid         string   `json:"pid"`
			Ipc         string   `json:"ipc"`
			Userns      string   `json:"userns_mode"`
			CapAdd      []string `json:"cap_add"`
			Devices     []any    `json:"devices"`
			SecurityOpt []string `json:"security_opt"`
			Build       *struct {
				Context          string `json:"context"`
				Dockerfile       string `json:"dockerfile"`
				DockerfileInline string `json:"dockerfile_inline"`
			} `json:"build"`
			Environment map[string]*string `json:"environment"`
			EnvFile     any                `json:"env_file"`
			Ports       []struct {
				HostIP string `json:"host_ip"`
			} `json:"ports"`
			Volumes []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"volumes"`
		} `json:"services"`
		Networks map[string]struct {
			Driver   string `json:"driver"`
			Internal bool   `json:"internal"`
			External any    `json:"external"`
		} `json:"networks"`
	}
	if err := json.Unmarshal(doc, &file); err != nil {
		return []string{"not the JSON docker compose config prints: " + err.Error()}
	}
	if len(file.Services) == 0 {
		fail("no services found")
	}
	if urlSecret.Match(doc) {
		fail("a URL in the file carries a password")
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	for name, s := range file.Services {
		for _, p := range s.Ports {
			if p.HostIP != "127.0.0.1" {
				fail("%s publishes a port on %q; every port must be bound to 127.0.0.1", name, p.HostIP)
			}
		}
		switch {
		case s.Build == nil:
			if !strings.Contains(s.Image, "@sha256:") {
				fail("%s uses an image that isn't pinned by digest", name)
			}
		case s.Build.DockerfileInline != "":
			fail("%s builds from an inline Dockerfile; keep it in a file this test reads", name)
		default:
			text, err := dockerfile(s.Build.Context, s.Build.Dockerfile)
			if err != nil {
				fail("%s: its Dockerfile can't be read: %v", name, err)
			}
			for _, image := range unpinned(string(text)) {
				fail("%s builds from %s, which isn't pinned by digest", name, image)
			}
		}
		for key := range s.Environment {
			if secretName.MatchString(key) && !strings.HasSuffix(key, "_FILE") {
				fail("%s sets %s in the Compose file; a secret is made on first start, never written here", name, key)
			}
		}
		if s.EnvFile != nil {
			fail("%s reads an env_file, which this test can't see into", name)
		}
		if s.Privileged || len(s.CapAdd) > 0 || len(s.Devices) > 0 {
			fail("%s is privileged, adds a capability or gets a device", name)
		}
		if s.NetworkMode == "host" || s.Pid != "" || s.Ipc != "" || s.Userns != "" {
			fail("%s shares a namespace with the host or another container", name)
		}
		if len(s.SecurityOpt) != 1 || s.SecurityOpt[0] != "no-new-privileges:true" {
			fail("%s must set exactly no-new-privileges:true under security_opt", name)
		}
		for _, v := range s.Volumes {
			if v.Type == "volume" {
				continue
			}
			src := filepath.Clean(v.Source)
			if v.Type != "bind" || (src != root && !strings.HasPrefix(src, root+string(filepath.Separator))) {
				fail("%s mounts a %s from outside this repository; only named volumes and folders in it are allowed", name, v.Type)
			}
		}
	}
	for name, n := range file.Networks {
		if (n.Driver != "" && n.Driver != "bridge") || n.External != nil {
			fail("network %s isn't a plain bridge network of this stack", name)
		}
	}
	if !file.Networks["sandbox"].Internal {
		fail("the sandbox network must be internal")
	}
	return bad
}

// The rules in the comment at the top of compose.yaml, read from Compose's own resolved view of the file so
// a short-form port or an anchor can't hide one.
func TestComposeFileKeepsItsRules(t *testing.T) {
	needCompose(t)
	out, err := exec.Command("docker", "compose", "-f", "compose.yaml", "config", "--format", "json").Output()
	if err != nil {
		t.Fatalf("docker compose config: %v", err)
	}
	for _, problem := range checkConfig(out, func(context, name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(context, name))
	}) {
		t.Error(problem)
	}
}

// Each rule, broken one at a time in a small resolved document. Needs no Docker.
func TestRulesCatchABrokenFile(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	quote := func(path string) string {
		b, _ := json.Marshal(path)
		return string(b)
	}
	const ok = `"image":"x@sha256:00","security_opt":["no-new-privileges:true"]`
	doc := func(service, networks string) string {
		return `{"services":{"a":{` + service + `}},"networks":{"sandbox":{"internal":true}` + networks + `}}`
	}
	dockerfile := "FROM --platform=linux/amd64 golang:1@sha256:00 AS build\nFROM scratch\nCOPY --from=build /a /a\nCOPY --from=0 /a /b\n"
	read := func(string, string) ([]byte, error) { return []byte(dockerfile), nil }

	good := ok + `,"build":{"context":".","dockerfile":"d"},"ports":[{"host_ip":"127.0.0.1"}],` +
		`"environment":{"POSTGRES_PASSWORD_FILE":"/f"},` +
		`"volumes":[{"type":"volume","source":"v"},{"type":"bind","source":` + quote(filepath.Join(root, "examples")) + `}]`
	if bad := checkConfig([]byte(doc(good, "")), read); len(bad) != 0 {
		t.Fatalf("a file that keeps every rule was refused: %v", bad)
	}

	const noOpt = `"image":"x@sha256:00"`
	for name, c := range map[string]struct{ service, networks, dockerfile, want string }{
		"port on every address":    {service: ok + `,"ports":[{"host_ip":"0.0.0.0"}]`, want: "127.0.0.1"},
		"port with no address":     {service: ok + `,"ports":[{}]`, want: "127.0.0.1"},
		"image by tag":             {service: `"image":"x:latest","security_opt":["no-new-privileges:true"]`, want: "pinned"},
		"inline Dockerfile":        {service: ok + `,"build":{"dockerfile_inline":"FROM x"}`, want: "inline"},
		"Dockerfile by tag":        {service: ok + `,"build":{"dockerfile":"d"}`, dockerfile: "FROM alpine:latest\n", want: "pinned"},
		"Dockerfile in lower case": {service: ok + `,"build":{"dockerfile":"d"}`, dockerfile: "from --platform=linux/amd64 alpine:latest\n", want: "pinned"},
		"COPY from a tag":          {service: ok + `,"build":{"dockerfile":"d"}`, dockerfile: "FROM scratch\nCOPY --from=busybox:latest /a /a\n", want: "pinned"},
		"password variable":        {service: ok + `,"environment":{"POSTGRES_PASSWORD":"x"}`, want: "POSTGRES_PASSWORD"},
		"PGPASS variable":          {service: ok + `,"environment":{"PGPASS":"x"}`, want: "PGPASS"},
		"password in a URL":        {service: ok + `,"environment":{"DATABASE_URL":"postgres://ascc:x@postgres/ascc"}`, want: "URL"},
		"password in a command":    {service: ok + `,"command":["--db","postgres://ascc:x@postgres/ascc"]`, want: "URL"},
		"env_file":                 {service: ok + `,"env_file":[{"path":".env"}]`, want: "env_file"},
		"privileged":               {service: ok + `,"privileged":true`, want: "privileged"},
		"added capability":         {service: ok + `,"cap_add":["SYS_ADMIN"]`, want: "capability"},
		"host network":             {service: ok + `,"network_mode":"host"`, want: "namespace"},
		"host pid":                 {service: ok + `,"pid":"host"`, want: "namespace"},
		"no-new-privileges off":    {service: noOpt + `,"security_opt":["no-new-privileges:false"]`, want: "no-new-privileges"},
		"seccomp off":              {service: noOpt + `,"security_opt":["no-new-privileges:true","seccomp:unconfined"]`, want: "no-new-privileges"},
		"no security_opt":          {service: noOpt, want: "no-new-privileges"},
		"Docker socket":            {service: ok + `,"volumes":[{"type":"bind","source":"/var/run/docker.sock"}]`, want: "outside"},
		"repo's parent":            {service: ok + `,"volumes":[{"type":"bind","source":` + quote(filepath.Dir(root)) + `}]`, want: "outside"},
		"repo's sibling":           {service: ok + `,"volumes":[{"type":"bind","source":` + quote(root+"-other") + `}]`, want: "outside"},
		"tmpfs":                    {service: ok + `,"volumes":[{"type":"tmpfs"}]`, want: "outside"},
		"macvlan network":          {service: ok, networks: `,"lan":{"driver":"macvlan"}`, want: "bridge"},
		"external network":         {service: ok, networks: `,"lan":{"external":true}`, want: "bridge"},
		"sandbox not internal":     {service: ok, networks: `,"sandbox":{"internal":false}`, want: "sandbox"},
	} {
		dockerfile = c.dockerfile
		bad := strings.Join(checkConfig([]byte(doc(c.service, c.networks)), read), "\n")
		if !strings.Contains(bad, c.want) {
			t.Errorf("%s: got %q, want a refusal mentioning %q", name, bad, c.want)
		}
	}
}

const (
	denyBody = `{"tenant_id":"tnt_lab","principal":{"type":"Agent","id":"dev-agent"},"action":"runtime.tool_use",` +
		`"resource":{"type":"Tool","id":"claude-code/Bash"},` +
		`"context":{"path_in_repo":true,"network":true,"touches_secret_path":true,"touches_ascc_state":false}}`
	allowBody = `{"tenant_id":"tnt_lab","principal":{"type":"Agent","id":"dev-agent"},"action":"runtime.tool_use",` +
		`"resource":{"type":"Tool","id":"claude-code/Read"},` +
		`"context":{"path_in_repo":true,"network":false,"touches_secret_path":false,"touches_ascc_state":false}}`
	rootToken = `sed -n 's/^Initial Root Token: //p' /openbao/file/dev-keys`
)

// Brings the whole stack up from nothing and checks what the roadmap item says it gives. It uses the same
// host ports as the stack a developer runs, so stop that one first. CI runs it on Linux.
func TestStackComesUp(t *testing.T) {
	if os.Getenv("ASCC_COMPOSE_SMOKE") == "" {
		t.Skip("set ASCC_COMPOSE_SMOKE=1 to start the stack")
	}
	needCompose(t)
	dc := func(args ...string) ([]byte, error) {
		cmd := exec.Command("docker", append([]string{"compose", "-f", "compose.yaml", "-p", "ascc-smoke"}, args...)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return append(out, stderr.Bytes()...), err
		}
		return out, nil
	}
	t.Cleanup(func() { dc("down", "-v", "--remove-orphans") })
	if out, err := dc("up", "-d", "--build", "--wait"); err != nil {
		t.Fatalf("docker compose up: %v\n%s", err, out)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	get := func(u string) (int, string) {
		resp, err := client.Get(u)
		if err != nil {
			return 0, err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return resp.StatusCode, string(b)
	}
	// until polls for a minute: `up --wait` covers the services with a health check, not the others.
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
			if ok() {
				return
			}
		}
		t.Fatalf("%s: not true after a minute", what)
	}

	decide := func(body string) string {
		resp, err := client.Post("http://127.0.0.1:8181/pdp/v1/decide", "application/json", strings.NewReader(body))
		if err != nil {
			return err.Error()
		}
		defer resp.Body.Close()
		var d struct {
			Decision string   `json:"decision"`
			Policies []string `json:"determining_policies"`
		}
		json.NewDecoder(resp.Body).Decode(&d)
		return d.Decision + " " + strings.Join(d.Policies, ",")
	}
	until("the decision point loads the sample registry and policies", func() bool {
		code, _ := get("http://127.0.0.1:8181/healthz")
		return code == 200
	})
	if got := decide(denyBody); got != "deny deny-network-with-secret-path" {
		t.Errorf("sending a secret file over the network: got %q, want the sample deny", got)
	}
	if got := decide(allowBody); !strings.HasPrefix(got, "allow ") {
		t.Errorf("reading a file in the repo: got %q, want an allow", got)
	}

	if code, body := get("http://127.0.0.1:18080/realms/ascc-dev/.well-known/openid-configuration"); code != 200 ||
		!strings.Contains(body, "/realms/ascc-dev") {
		t.Errorf("Keycloak doesn't serve the imported ascc-dev realm (status %d)", code)
	}

	unsealed := func() bool {
		_, body := get("http://127.0.0.1:18200/v1/sys/seal-status")
		return strings.Contains(body, `"initialized":true`) && strings.Contains(body, `"sealed":false`)
	}
	if !unsealed() {
		t.Error("OpenBao isn't initialised and unsealed after up --wait")
	}
	// The token stays inside the container: the shell there reads it and hands it to bao.
	mounts, _ := dc("exec", "-T", "openbao", "sh", "-c",
		`BAO_TOKEN=$(`+rootToken+`) bao secrets list -address=http://127.0.0.1:8200`)
	if !bytes.Contains(mounts, []byte("secret/")) || !bytes.Contains(mounts, []byte("transit/")) {
		t.Error("OpenBao reports healthy without kv-v2 at secret/ and transit at transit/")
	}
	if out, err := dc("restart", "openbao"); err != nil {
		t.Fatalf("restart openbao: %v\n%s", err, out)
	}
	until("OpenBao unseals itself again after a restart", unsealed)

	until("the Jaeger UI answers", func() bool {
		code, _ := get("http://127.0.0.1:16686/")
		return code == 200
	})

	if out, err := dc("exec", "-T", "postgres", "psql", "-U", "ascc", "-d", "ascc", "-Atc", "select current_database()"); err != nil ||
		strings.TrimSpace(string(out)) != "ascc" {
		t.Errorf("Postgres has no ascc database: %v", err)
	}

	// The egress placeholder refuses everything. paste.example is never resolved: the proxy has no upstream.
	proxy, _ := url.Parse("http://127.0.0.1:15001")
	viaProxy := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	until("the egress proxy answers 403 to a plain request", func() bool {
		resp, err := viaProxy.Get("http://paste.example/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusForbidden
	})
	if resp, err := viaProxy.Get("https://paste.example/"); err == nil {
		resp.Body.Close()
		t.Error("the egress proxy opened a CONNECT tunnel; it must refuse every one")
	} else if !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("CONNECT through the egress proxy failed, but not with a 403: %v", err)
	}

	// Each service made its own secret and none of them reached a log. The values are read here only to
	// search for them, and a failure names the secret without printing it.
	logs, err := dc("logs", "--no-color")
	if err != nil {
		t.Fatalf("docker compose logs: %v", err)
	}
	for name, cmd := range map[string][]string{
		"the Postgres password":       {"postgres", "cat", "/run/ascc/password"},
		"the Keycloak admin password": {"keycloak", "cat", "/opt/keycloak/data/admin-password"},
		"the OpenBao unseal key":      {"openbao", "sed", "-n", "s/^Unseal Key 1: //p", "/openbao/file/dev-keys"},
		"the OpenBao root token":      {"openbao", "sh", "-c", rootToken},
	} {
		out, err := dc(append([]string{"exec", "-T"}, cmd...)...)
		value := bytes.TrimSpace(out)
		if err != nil || len(value) < 20 {
			t.Errorf("%s wasn't made on first start", name)
			continue
		}
		if bytes.Contains(logs, value) {
			t.Errorf("%s is in the service logs", name)
		}
	}
}
