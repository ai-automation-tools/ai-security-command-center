// Command ascc is the AI Security Command Center CLI: `decide` answers one request against the agent
// registry and the Cedar policies, and `inventory` writes the registry as an Agent Security Suite
// inventory document. The Claude Code hook, the audit log and the kill switch arrive in later slices.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/version"
)

const usage = `ascc decides what an AI agent may do.

Usage:
  ascc decide    --request <file|-> [--registry <file>] [--policies <dir>]
  ascc inventory [--out <file>]     [--registry <file>] [--policies <dir>]
  ascc version

--registry defaults to ascc.registry.json and --policies to policies (every *.cedar file in it).
Exit codes: 0 a decision was made (allow or deny) or the inventory was written; 2 ascc couldn't run.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("ascc "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	registry := fs.String("registry", "ascc.registry.json", "agent registry file")
	policies := fs.String("policies", "policies", "directory of *.cedar policy files")

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "ascc "+version.Version)
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	case "decide":
		request := fs.String("request", "", "request JSON file, or - for stdin")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		d, err := decide(*registry, *policies, *request, stdin)
		writeJSON(stdout, d)
		return fail(stderr, err)
	case "inventory":
		out := fs.String("out", "", "write the document here instead of stdout")
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		return fail(stderr, inventory(*registry, *policies, *out, stdout))
	default:
		fmt.Fprintf(stderr, "ascc: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func fail(stderr io.Writer, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, "ascc: "+err.Error())
		return 2
	}
	return 0
}

// decide always returns a decision to print. When ascc can't load its inputs the decision is a deny
// carrying the error, and the error comes back too so the exit code says ascc couldn't run.
func decide(registry, policies, request string, stdin io.Reader) (pdp.Decision, error) {
	deny := func(err error) (pdp.Decision, error) { return pdp.Deny(err.Error()), err }
	p, err := load(registry, policies)
	if err != nil {
		return deny(err)
	}
	var data []byte
	switch request {
	case "":
		return deny(errors.New("decide needs --request <file|->"))
	case "-":
		data, err = io.ReadAll(stdin)
	default:
		data, err = os.ReadFile(request)
	}
	if err != nil {
		return deny(fmt.Errorf("request: %v", err))
	}
	req, err := pdp.ParseRequest(data)
	if err != nil {
		return deny(err)
	}
	return p.Decide(req), nil
}

func inventory(registry, policies, out string, stdout io.Writer) error {
	p, err := load(registry, policies)
	if err != nil {
		return err
	}
	doc := p.Inventory(version.Version, time.Now())
	if out == "" {
		return writeJSON(stdout, doc)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := writeJSON(f, doc); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func load(registry, policies string) (*pdp.PDP, error) {
	reg, err := os.ReadFile(registry)
	if err != nil {
		return nil, fmt.Errorf("registry: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(policies, "*.cedar"))
	if err != nil {
		return nil, fmt.Errorf("policies: %v", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("policies: no .cedar file in %s", policies)
	}
	files := map[string][]byte{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("policies: %v", err)
		}
		files[filepath.Base(path)] = src
	}
	return pdp.New(reg, files)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
