// Command buildsite writes the browser demo to a folder, ready for a static host:
//
//	go run ./src/cmd/buildsite            # writes _site/
//	go run ./src/cmd/buildsite -out dir
//
// It copies site/, compiles src/cmd/ascc-wasm for the browser and embeds it in a script
// (ascc.wasm.js) so the page never fetches it, copies Go's own wasm_exec.js, and writes the planted
// registry, policies and tool calls from examples/ as fixtures.js. It is Go, not a shell script, so
// it runs the same in PowerShell and bash. It is a build tool and no part of the ascc binary.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The tool calls the page replays, in the order it shows them. The injected one comes first.
var calls = []string{"exfil-curl", "read-repo-file", "run-tests", "push-to-main", "edit-settings"}

func main() {
	out := flag.String("out", "_site", "folder to write the site to")
	flag.Parse()
	root, err := repoRoot()
	if err == nil {
		err = build(root, *out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "buildsite: "+err.Error())
		os.Exit(1)
	}
	fmt.Println("buildsite: wrote " + *out)
}

// repoRoot is the nearest folder at or above the current one that holds go.mod.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run it from inside the repo: no go.mod found")
		}
		dir = parent
	}
}

func build(root, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.CopyFS(out, os.DirFS(filepath.Join(root, "site"))); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	// A rebuild overwrites: CopyFS won't, so copy the page's own files by hand when they exist.
	pages, err := os.ReadDir(filepath.Join(root, "site"))
	if err != nil {
		return err
	}
	for _, f := range pages {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "site", f.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, f.Name()), data, 0o644); err != nil {
			return err
		}
	}

	tmp, err := os.MkdirTemp("", "ascc-wasm")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	wasm := filepath.Join(tmp, "ascc.wasm")
	// -trimpath keeps this machine's folder names out of the published binary.
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", wasm, "./src/cmd/ascc-wasm")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build for WebAssembly: %v\n%s", err, msg)
	}
	bin, err := os.ReadFile(wasm)
	if err != nil {
		return err
	}
	// ponytail: base64 in a script makes the download a third larger than the .wasm. It is what lets
	// the page load with fetch switched off; serve the .wasm itself if the size ever matters more.
	if err := os.WriteFile(filepath.Join(out, "ascc.wasm.js"), []byte(`globalThis.ASCC_WASM="`+base64.StdEncoding.EncodeToString(bin)+"\";\n"), 0o644); err != nil {
		return err
	}

	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("go env GOROOT: %v", err)
	}
	glue, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "wasm_exec.js"), glue, 0o644); err != nil {
		return err
	}

	fixtures, err := fixturesJS(filepath.Join(root, "examples"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "fixtures.js"), fixtures, 0o644)
}

// fixturesJS is the planted setup the page starts from: the same example files the tests and the
// README run.
func fixturesJS(examples string) ([]byte, error) {
	read := func(parts ...string) (string, error) {
		b, err := os.ReadFile(filepath.Join(append([]string{examples}, parts...)...))
		return strings.ReplaceAll(string(b), "\r\n", "\n"), err
	}
	type call struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
	}
	doc := struct {
		Registry string            `json:"registry"`
		Policies map[string]string `json:"policies"`
		Calls    []call            `json:"calls"`
	}{Policies: map[string]string{}}
	var err error
	if doc.Registry, err = read("ascc.registry.json"); err != nil {
		return nil, err
	}
	policies, err := filepath.Glob(filepath.Join(examples, "policies", "*.cedar"))
	if err != nil || len(policies) == 0 {
		return nil, errors.New("no policy in examples/policies")
	}
	for _, p := range policies {
		if doc.Policies[filepath.Base(p)], err = read("policies", filepath.Base(p)); err != nil {
			return nil, err
		}
	}
	for _, name := range calls {
		payload, err := read("hook-payloads", name+".json")
		if err != nil {
			return nil, err
		}
		doc.Calls = append(doc.Calls, call{Name: name, Payload: payload})
	}
	js, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(append([]byte("globalThis.ASCC_FIXTURES = "), js...), ";\n"...), nil
}
