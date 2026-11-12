// Package compose_test checks the development stack in this folder: the rules compose.yaml keeps, on every
// run, and the running stack itself when ASCC_COMPOSE_SMOKE=1.
package compose_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	"strconv"
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
	// --from=image on a COPY, and from=image inside a RUN --mount.
	copyFrom = regexp.MustCompile(`(?i)\bfrom=([^\s,]+)`)
)

// unpinned returns the images a Dockerfile takes from a registry without a digest.
func unpinned(dockerfile string) []string {
	var bad []string
	stages := map[string]bool{"scratch": true}
	check := func(image string) {
		if _, err := strconv.Atoi(image); err == nil || stages[strings.ToLower(image)] {
			return // an earlier stage, by number or by name
		}
		if !strings.Contains(image, "@sha256:") {
			bad = append(bad, image)
		}
	}
	for _, line := range strings.Split(dockerfile, "\n") {
		// A "# syntax=" line names an image too: the one that reads the rest of the file.
		if rest, ok := strings.CutPrefix(strings.ReplaceAll(strings.ToLower(line), " ", ""), "#syntax="); ok {
			check(rest)
			continue
		}
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
// service builds from, given its build context and name. fromEnv holds the values the env file gave, so a
// secret that came from anywhere else stands out.
func checkConfig(doc []byte, dockerfile func(context, name string) ([]byte, error), fromEnv map[string]bool) []string {
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
			Uts         string   `json:"uts"`
			Cgroup      string   `json:"cgroup"`
			Sysctls     any      `json:"sysctls"`
			APISocket   bool     `json:"use_api_socket"`
			VolumesFrom []string `json:"volumes_from"`
			Secrets     []any    `json:"secrets"`
			Configs     []any    `json:"configs"`
			CapAdd      []string `json:"cap_add"`
			Devices     []any    `json:"devices"`
			SecurityOpt []string `json:"security_opt"`
			Build       *struct {
				Context          string `json:"context"`
				Dockerfile       string `json:"dockerfile"`
				DockerfileInline string `json:"dockerfile_inline"`
				Network          string `json:"network"`
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
			Driver     string `json:"driver"`
			DriverOpts any    `json:"driver_opts"`
			Internal   bool   `json:"internal"`
			External   any    `json:"external"`
			IPv6       bool   `json:"enable_ipv6"`
			Attachable bool   `json:"attachable"`
		} `json:"networks"`
		Volumes map[string]struct {
			Driver     string `json:"driver"`
			DriverOpts any    `json:"driver_opts"`
			External   any    `json:"external"`
		} `json:"volumes"`
		Secrets map[string]any `json:"secrets"`
		Configs map[string]any `json:"configs"`
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
	// A value from .env belongs in a service's environment and nowhere else: in a command or a health
	// check it is a process argument, in a label anyone can read it, in a build argument it is in the image.
	var rest map[string]any
	json.Unmarshal(doc, &rest)
	if services, ok := rest["services"].(map[string]any); ok {
		for _, s := range services {
			if s, ok := s.(map[string]any); ok {
				delete(s, "environment")
			}
		}
	}
	outside, _ := json.Marshal(rest)
	for value := range fromEnv {
		if bytes.Contains(outside, []byte(value)) {
			fail("a value from .env is used outside a service's environment")
		}
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	inRepo := func(path string) bool {
		path = filepath.Clean(path)
		return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
	}
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
		case !inRepo(s.Build.Context) || (s.Build.Network != "" && s.Build.Network != "default"):
			fail("%s builds from a context outside this repository, or on a network of its own choosing", name)
		default:
			text, err := dockerfile(s.Build.Context, s.Build.Dockerfile)
			if err != nil {
				fail("%s: its Dockerfile can't be read: %v", name, err)
			}
			for _, image := range unpinned(string(text)) {
				fail("%s builds from %s, which isn't pinned by digest", name, image)
			}
		}
		for key, value := range s.Environment {
			if secretName.MatchString(key) && !strings.HasSuffix(key, "_FILE") && (value == nil || !fromEnv[*value]) {
				fail("%s sets %s to something that isn't a variable from .env", name, key)
			}
		}
		if s.EnvFile != nil {
			fail("%s reads an env_file, which this test can't see into", name)
		}
		if s.Privileged || len(s.CapAdd) > 0 || len(s.Devices) > 0 {
			fail("%s is privileged, adds a capability or gets a device", name)
		}
		if s.NetworkMode == "host" || strings.Contains(s.NetworkMode, ":") || s.Pid != "" || s.Ipc != "" || s.Userns != "" ||
			s.Uts != "" || s.Cgroup != "" || s.Sysctls != nil {
			fail("%s shares a namespace with the host or another container, or sets a sysctl", name)
		}
		if s.APISocket || len(s.VolumesFrom) > 0 || len(s.Secrets) > 0 || len(s.Configs) > 0 {
			fail("%s gets the Docker socket, another container's volumes, or a secret or config file from outside this repository", name)
		}
		if len(s.SecurityOpt) != 1 || s.SecurityOpt[0] != "no-new-privileges:true" {
			fail("%s must set exactly no-new-privileges:true under security_opt", name)
		}
		for _, v := range s.Volumes {
			if v.Type == "volume" {
				continue
			}
			if v.Type != "bind" || !inRepo(v.Source) {
				fail("%s mounts a %s from outside this repository; only named volumes and folders in it are allowed", name, v.Type)
			}
		}
	}
	for name, n := range file.Networks {
		if (n.Driver != "" && n.Driver != "bridge") || n.External != nil || n.DriverOpts != nil || n.IPv6 || n.Attachable {
			fail("network %s isn't a plain bridge network of this stack", name)
		}
	}
	// A named volume with driver options can be a bind mount of anything on the host.
	for name, v := range file.Volumes {
		if (v.Driver != "" && v.Driver != "local") || v.DriverOpts != nil || v.External != nil {
			fail("volume %s mounts from outside this repository; a named volume takes no driver, options or external name", name)
		}
	}
	if len(file.Secrets) > 0 || len(file.Configs) > 0 {
		fail("the file declares a secret or config, which mounts a file from outside this repository; secrets come from .env")
	}
	if !file.Networks["sandbox"].Internal {
		fail("the sandbox network must be internal")
	}
	return bad
}

// envFile writes an env file giving every name in .env.example the value made for it, and returns its path
// and the values. .env.example itself must hold names only.
func envFile(t *testing.T, value func(name string) string) (string, map[string]bool) {
	t.Helper()
	example, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	values := map[string]bool{}
	for _, line := range strings.Split(string(example), "\n") {
		name, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(name, "#") {
			continue
		}
		if v != "" {
			t.Errorf(".env.example gives %s a value; it holds names only", name)
		}
		values[value(name)] = true
		fmt.Fprintf(&out, "%s=%s\n", name, value(name))
	}
	if len(values) == 0 {
		t.Fatal(".env.example names no variable")
	}
	path := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(path, []byte(out.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, values
}

// The rules in the comment at the top of compose.yaml, read from Compose's own resolved view of the file so
// a short-form port or an anchor can't hide one.
func TestComposeFileKeepsItsRules(t *testing.T) {
	needCompose(t)
	env, fromEnv := envFile(t, func(name string) string { return "from-env-" + name })
	out, err := exec.Command("docker", "compose", "--env-file", env, "-f", "compose.yaml", "config", "--format", "json").Output()
	if err != nil {
		t.Fatalf("docker compose config: %v", err)
	}
	for _, problem := range checkConfig(out, func(context, name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(context, name))
	}, fromEnv) {
		t.Error(problem)
	}

	// Compose folds an env_file into environment and drops the key, so it has to be looked for in the file.
	if raw, err := os.ReadFile("compose.yaml"); err != nil || bytes.Contains(raw, []byte("env_file")) {
		t.Errorf("compose.yaml uses env_file, which this test can't see into (or can't be read: %v)", err)
	}

	// With any one secret missing the stack must refuse to start, not start with an empty or default one.
	example, _ := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	var names []string
	for _, line := range strings.Split(string(example), "\n") {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(name, "#") {
			names = append(names, name)
		}
	}
	for _, missing := range names {
		partial := filepath.Join(t.TempDir(), "env")
		var text string
		for _, name := range names {
			if name != missing {
				text += name + "=from-env-" + name + "\n"
			}
		}
		os.WriteFile(partial, []byte(text), 0o600)
		cmd := exec.Command("docker", "compose", "--env-file", partial, "-f", "compose.yaml", "config", "-q")
		cmd.Env = append(os.Environ(), missing+"=")
		if err := cmd.Run(); err == nil {
			t.Errorf("the Compose file resolves without %s; each secret must be required", missing)
		}
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
	doc := func(service, networks, more string) string {
		return `{"services":{"a":{` + service + `}},"networks":{"sandbox":{"internal":true}` + networks + `}` + more + `}`
	}
	dockerfile := "FROM --platform=linux/amd64 golang:1@sha256:00 AS build\nFROM scratch\nCOPY --from=build /a /a\nCOPY --from=0 /a /b\n"
	read := func(string, string) ([]byte, error) { return []byte(dockerfile), nil }

	good := ok + `,"build":{"context":` + quote(root) + `,"dockerfile":"d"},"ports":[{"host_ip":"127.0.0.1"}],` +
		`"environment":{"POSTGRES_PASSWORD_FILE":"/f","POSTGRES_PASSWORD":"from-env"},` +
		`"volumes":[{"type":"volume","source":"v"},{"type":"bind","source":` + quote(filepath.Join(root, "examples")) + `}]`
	fromEnv := map[string]bool{"from-env": true}
	if bad := checkConfig([]byte(doc(good, "", `,"volumes":{"v":{"name":"stack_v"}}`)), read, fromEnv); len(bad) != 0 {
		t.Fatalf("a file that keeps every rule was refused: %v", bad)
	}

	const noOpt = `"image":"x@sha256:00"`
	build := func(dockerfile string) struct{ service, networks, volumes, dockerfile, want string } {
		return struct{ service, networks, volumes, dockerfile, want string }{
			service: ok + `,"build":{"context":` + quote(root) + `,"dockerfile":"d"}`, dockerfile: dockerfile, want: "pinned"}
	}
	for name, c := range map[string]struct{ service, networks, volumes, dockerfile, want string }{
		"port on every address":     {service: ok + `,"ports":[{"host_ip":"0.0.0.0"}]`, want: "127.0.0.1"},
		"port with no address":      {service: ok + `,"ports":[{}]`, want: "127.0.0.1"},
		"image by tag":              {service: `"image":"x:latest","security_opt":["no-new-privileges:true"]`, want: "pinned"},
		"inline Dockerfile":         {service: ok + `,"build":{"context":` + quote(root) + `,"dockerfile_inline":"FROM x"}`, want: "inline"},
		"Dockerfile by tag":         build("FROM alpine:latest\n"),
		"Dockerfile in lower case":  build("from --platform=linux/amd64 alpine:latest\n"),
		"COPY from a tag":           build("FROM scratch\nCOPY --from=busybox:latest /a /a\n"),
		"image named like a stage":  build("FROM 1password/op:latest\n"),
		"syntax line by tag":        build("# syntax=frontend.example/dockerfile:latest\nFROM scratch\n"),
		"RUN mount from a tag":      build("FROM scratch\nRUN --mount=type=bind,from=busybox:latest,target=/b true\n"),
		"build context outside":     {service: ok + `,"build":{"context":` + quote(filepath.Dir(root)) + `,"dockerfile":"d"}`, want: "context"},
		"build on the host net":     {service: ok + `,"build":{"context":` + quote(root) + `,"dockerfile":"d","network":"host"}`, want: "context"},
		"secret in a command":       {service: ok + `,"command":["--password","from-env"]`, want: "outside a service's environment"},
		"secret in a health check":  {service: ok + `,"healthcheck":{"test":["CMD","pg_isready","from-env"]}`, want: "outside a service's environment"},
		"secret in a label":         {service: ok + `,"labels":{"pw":"from-env"}`, want: "outside a service's environment"},
		"secret as a build arg":     {service: ok + `,"build":{"context":` + quote(root) + `,"dockerfile":"d","args":{"PW":"from-env"}}`, want: "outside a service's environment"},
		"password variable":         {service: ok + `,"environment":{"POSTGRES_PASSWORD":"x"}`, want: "POSTGRES_PASSWORD"},
		"password left empty":       {service: ok + `,"environment":{"POSTGRES_PASSWORD":null}`, want: "POSTGRES_PASSWORD"},
		"PGPASS variable":           {service: ok + `,"environment":{"PGPASS":"x"}`, want: "PGPASS"},
		"password in a URL":         {service: ok + `,"environment":{"DATABASE_URL":"postgres://ascc:x@postgres/ascc"}`, want: "URL"},
		"password in a command":     {service: ok + `,"command":["--db","postgres://ascc:x@postgres/ascc"]`, want: "URL"},
		"env_file":                  {service: ok + `,"env_file":[{"path":".env"}]`, want: "env_file"},
		"privileged":                {service: ok + `,"privileged":true`, want: "privileged"},
		"added capability":          {service: ok + `,"cap_add":["SYS_ADMIN"]`, want: "capability"},
		"host network":              {service: ok + `,"network_mode":"host"`, want: "namespace"},
		"host pid":                  {service: ok + `,"pid":"host"`, want: "namespace"},
		"host ipc":                  {service: ok + `,"ipc":"host"`, want: "namespace"},
		"host user namespace":       {service: ok + `,"userns_mode":"host"`, want: "namespace"},
		"host uts":                  {service: ok + `,"uts":"host"`, want: "namespace"},
		"host cgroup":               {service: ok + `,"cgroup":"host"`, want: "namespace"},
		"another service's network": {service: ok + `,"network_mode":"service:b"`, want: "namespace"},
		"a sysctl":                  {service: ok + `,"sysctls":{"net.ipv4.ip_forward":"1"}`, want: "sysctl"},
		"a device":                  {service: ok + `,"devices":[{"source":"/dev/kmsg"}]`, want: "device"},
		"the API socket":            {service: ok + `,"use_api_socket":true`, want: "Docker socket"},
		"volumes from another":      {service: ok + `,"volumes_from":["b"]`, want: "Docker socket"},
		"a secret file":             {service: ok + `,"secrets":[{"source":"s"}]`, want: "Docker socket"},
		"a declared secret":         {service: ok, volumes: `,"secrets":{"s":{"file":"/etc/shadow"}}`, want: "declares a secret"},
		"volume bound to the host":  {service: ok, volumes: `,"volumes":{"v":{"driver_opts":{"type":"none","o":"bind","device":"/"}}}`, want: "volume v"},
		"external volume":           {service: ok, volumes: `,"volumes":{"v":{"external":true}}`, want: "volume v"},
		"network driver options":    {service: ok, networks: `,"lan":{"driver_opts":{"com.docker.network.bridge.gateway_mode_ipv4":"nat-unprotected"}}`, want: "bridge"},
		"IPv6 network":              {service: ok, networks: `,"lan":{"enable_ipv6":true}`, want: "bridge"},
		"attachable network":        {service: ok, networks: `,"lan":{"attachable":true}`, want: "bridge"},
		"no-new-privileges off":     {service: noOpt + `,"security_opt":["no-new-privileges:false"]`, want: "no-new-privileges"},
		"seccomp off":               {service: noOpt + `,"security_opt":["no-new-privileges:true","seccomp:unconfined"]`, want: "no-new-privileges"},
		"no security_opt":           {service: noOpt, want: "no-new-privileges"},
		"Docker socket":             {service: ok + `,"volumes":[{"type":"bind","source":"/var/run/docker.sock"}]`, want: "outside"},
		"repo's parent":             {service: ok + `,"volumes":[{"type":"bind","source":` + quote(filepath.Dir(root)) + `}]`, want: "outside"},
		"repo's sibling":            {service: ok + `,"volumes":[{"type":"bind","source":` + quote(root+"-other") + `}]`, want: "outside"},
		"tmpfs":                     {service: ok + `,"volumes":[{"type":"tmpfs"}]`, want: "outside"},
		"macvlan network":           {service: ok, networks: `,"lan":{"driver":"macvlan"}`, want: "bridge"},
		"external network":          {service: ok, networks: `,"lan":{"external":true}`, want: "bridge"},
		"sandbox not internal":      {service: ok, networks: `,"sandbox":{"internal":false}`, want: "sandbox"},
	} {
		dockerfile = c.dockerfile
		bad := strings.Join(checkConfig([]byte(doc(c.service, c.networks, c.volumes)), read, fromEnv), "\n")
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
)

// Brings the whole stack up from nothing and checks what the roadmap item says it gives. It uses the same
// host ports as the stack a developer runs, so stop that one first. CI runs it on Linux.
func TestStackComesUp(t *testing.T) {
	if os.Getenv("ASCC_COMPOSE_SMOKE") == "" {
		t.Skip("set ASCC_COMPOSE_SMOKE=1 to start the stack")
	}
	needCompose(t)
	// Throwaway secrets, as .env would give them. They are never printed: a failure names the variable.
	secrets := map[string]string{}
	env, _ := envFile(t, func(name string) string {
		if secrets[name] == "" {
			b := make([]byte, 32)
			rand.Read(b)
			secrets[name] = hex.EncodeToString(b)
		}
		return secrets[name]
	})
	dc := func(args ...string) ([]byte, error) {
		cmd := exec.Command("docker", append([]string{"compose", "--env-file", env, "-f", "compose.yaml", "-p", "ascc-smoke"}, args...)...)
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
	// The root token is the one from .env, and the one OpenBao generated at init is gone.
	mounts, _ := dc("exec", "-T", "openbao", "sh", "-c",
		`test ! -e /openbao/file/dev-init.tmp && BAO_TOKEN=$ASCC_OPENBAO_ROOT_TOKEN bao secrets list -address=http://127.0.0.1:8200`)
	if !bytes.Contains(mounts, []byte("secret/")) || !bytes.Contains(mounts, []byte("transit/")) {
		t.Error("OpenBao doesn't take the root token from .env, or lacks kv-v2 at secret/ and transit at transit/")
	}
	if resp, err := client.PostForm("http://127.0.0.1:18080/realms/master/protocol/openid-connect/token", url.Values{
		"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {"admin"},
		"password": {secrets["ASCC_KEYCLOAK_ADMIN_PASSWORD"]},
	}); err != nil {
		t.Errorf("Keycloak admin login: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("Keycloak refuses the admin password from .env (status %d)", resp.StatusCode)
		}
	}
	if out, err := dc("restart", "openbao"); err != nil {
		t.Fatalf("restart openbao: %v\n%s", err, out)
	}
	until("OpenBao unseals itself again after a restart", unsealed)

	until("the Jaeger UI answers", func() bool {
		code, _ := get("http://127.0.0.1:16686/")
		return code == 200
	})

	// By name, not loopback: inside its own container Postgres trusts loopback and asks for no password.
	const psql = `psql -h postgres -U ascc -d ascc -Atc "select current_database()"`
	if out, err := dc("exec", "-T", "postgres", "sh", "-c", `PGPASSWORD=$POSTGRES_PASSWORD `+psql); err != nil ||
		strings.TrimSpace(string(out)) != "ascc" {
		t.Errorf("Postgres has no ascc database behind the password from .env: %v", err)
	}
	if _, err := dc("exec", "-T", "postgres", "sh", "-c", `PGPASSWORD=wrong `+psql); err == nil {
		t.Error("Postgres let a wrong password in")
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

	// No secret from .env reached a log.
	logs, err := dc("logs", "--no-color")
	if err != nil {
		t.Fatalf("docker compose logs: %v", err)
	}
	for name, value := range secrets {
		if bytes.Contains(logs, []byte(value)) {
			t.Errorf("the value of %s is in the service logs", name)
		}
	}
}
