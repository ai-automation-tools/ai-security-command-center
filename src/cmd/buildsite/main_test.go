package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Spec criterion 8, and the demo's zero-network test: the built page renders the planted deny on
// first load with fetch, XMLHttpRequest and every other way to the network stubbed to fail, and the
// edit, replay, tamper and kill steps work. testdata/render.js is the harness.
func TestBuiltPageRendersWithNoNetwork(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("node isn't on PATH, and CI must run this test")
		}
		t.Skip("node isn't on PATH; the page test needs it")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := build(root, out); err != nil {
		t.Fatal(err)
	}
	// Building twice into the same folder works: a rebuild is how the site is updated.
	if err := build(root, out); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	for _, want := range []string{
		`<meta name="build" content="__BUILD_SHA__">`, // the Pages workflow stamps it
		`class="src-bar"`, `View source`, // the org source bar
		`<link rel="icon" href="favicon.svg"`,
		// The browser refuses a network call, an inline script and a script from another host too.
		`content="default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'unsafe-inline'; img-src 'self'; connect-src 'none'; base-uri 'none'; form-action 'none'"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html lacks %s", want)
		}
	}
	for _, name := range []string{"favicon.svg", "theme.js", "demo.js", "wasm_exec.js", "ascc.wasm.js", "fixtures.js"} {
		if st, err := os.Stat(filepath.Join(out, name)); err != nil || st.Size() == 0 {
			t.Errorf("the site lacks %s", name)
		}
	}
	// Nothing the page loads comes from another host: links to click are fine, loads are not.
	if m := regexp.MustCompile(`(?i)<(script|link|img|iframe|source|video|audio)\b[^>]*\b(src|href)\s*=\s*["']?(https?:)?//`).FindString(page); m != "" {
		t.Errorf("index.html loads from another host: %s", m)
	}
	if regexp.MustCompile(`(?i)@import|url\(\s*['"]?(https?:)?//`).MatchString(page) {
		t.Error("the page's styles load from another host")
	}
	// The published binary names no folder on the machine that built it.
	script, _ := os.ReadFile(filepath.Join(out, "ascc.wasm.js"))
	b64 := bytes.TrimSuffix(bytes.TrimPrefix(bytes.TrimSpace(script), []byte(`globalThis.ASCC_WASM="`)), []byte(`";`))
	wasm, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil || len(wasm) < 1<<20 {
		t.Fatalf("ascc.wasm.js doesn't hold the binary: %v", err)
	}
	home, _ := os.UserHomeDir()
	for _, local := range []string{root, filepath.ToSlash(root), home, filepath.ToSlash(home)} {
		if local != "" && bytes.Contains(wasm, []byte(local)) {
			t.Error("the WebAssembly binary holds a path from the machine that built it")
		}
	}

	// The twin of TestBinaryCantOpenASocket: the browser build links nothing that can open one either,
	// whichever buttons a test happens to click.
	list := exec.Command("go", "list", "-deps", "./src/cmd/ascc-wasm")
	list.Dir = root
	list.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	deps, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, pkg := range strings.Fields(string(deps)) {
		if pkg == "net" || pkg == "net/http" || pkg == "crypto/tls" || pkg == "os/exec" {
			t.Errorf("the browser build links %s", pkg)
		}
	}

	cmd := exec.Command(node, filepath.Join("testdata", "render.js"), out)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the page failed in Node: %v\n%s", err, msg)
	}
}
