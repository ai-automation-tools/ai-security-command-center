package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var (
	schemas  = filepath.Join("..", "..", "..", "schemas", "platform", "v1alpha1")
	fixtures = filepath.Join(schemas, "fixtures")
	// The contracts TKT-005 names. A schema added later is picked up from the folder; one removed fails here.
	wanted = []string{"edge", "event", "pdp-request", "pdp-response", "plugin-capability-token", "plugin-manifest",
		"registry-entity", "transaction-token"}
)

const (
	idBase = "https://ai-automation-tools.dev/schemas/ascc/v1alpha1/"
	// A PDP response with every required field, to build test documents on.
	response = `"decision": "deny", "policy_version": "p1", "determining_policies": [], "reasons": [], "obligations": []`
)

func cli(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

// check validates a document from stdin against one of the repo's schemas.
func check(t *testing.T, schema, doc string) (code int, stdout, stderr string) {
	t.Helper()
	return cli(t, doc, "validate", "--schemas", schemas, "--schema", schema, "-")
}

func names(t *testing.T) []string {
	t.Helper()
	code, out, errOut := cli(t, "", "list", "--schemas", schemas)
	if code != 0 {
		t.Fatalf("list: exit %d: %s", code, errOut)
	}
	return strings.Fields(out)
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEverySchemaCompilesAndIsNamed(t *testing.T) {
	got := names(t)
	for _, w := range wanted {
		if i := sort.SearchStrings(got, w); i == len(got) || got[i] != w {
			t.Errorf("schema %s is missing; list printed %v", w, got)
		}
	}
	for _, name := range got {
		t.Run(name, func(t *testing.T) {
			if _, err := compile(schemas, name); err != nil {
				t.Fatalf("doesn't compile: %v", err)
			}
			var doc map[string]any
			readJSON(t, filepath.Join(schemas, name+suffix), &doc)
			if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" || doc["$id"] != idBase+name+".json" {
				t.Errorf("$schema %v, $id %v", doc["$schema"], doc["$id"])
			}
			for _, k := range []string{"title", "description"} {
				if s, _ := doc[k].(string); s == "" {
					t.Errorf("no %s", k)
				}
			}
		})
	}
}

// Every file under fixtures/<schema>/valid validates. Every one under invalid fails with one line, at
// the pointer and on the keyword that expect.json gives for it, so a fixture that starts failing for
// another reason fails here. Adding a fixture needs no change to this test; a schema with too few fails.
func TestFixtures(t *testing.T) {
	dirs, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, n := range names(t) {
		known[n] = true
	}
	for _, d := range dirs {
		if !known[d.Name()] {
			t.Errorf("fixtures/%s has no schema", d.Name())
		}
	}
	for name := range known {
		t.Run(name, func(t *testing.T) {
			valid, _ := filepath.Glob(filepath.Join(fixtures, name, "valid", "*.json"))
			invalid, _ := filepath.Glob(filepath.Join(fixtures, name, "invalid", "*.json"))
			if len(valid) < 2 || len(invalid) < 4 {
				t.Errorf("%d valid and %d invalid fixtures, want at least 2 and 4", len(valid), len(invalid))
			}
			for _, f := range valid {
				if code, _, errOut := cli(t, "", "validate", "--schemas", schemas, "--schema", name, f); code != 0 {
					t.Errorf("%s: exit %d: %s", filepath.Base(f), code, errOut)
				}
			}
			var expect map[string]struct{ Pointer, Rule string }
			readJSON(t, filepath.Join(fixtures, name, "expect.json"), &expect)
			if len(expect) != len(invalid) {
				t.Errorf("expect.json has %d entries for %d invalid fixtures", len(expect), len(invalid))
			}
			for _, f := range invalid {
				want, ok := expect[filepath.Base(f)]
				if !ok || want.Rule == "" {
					t.Errorf("%s: expect.json doesn't say where it should fail", filepath.Base(f))
					continue
				}
				code, _, errOut := cli(t, "", "validate", "--schemas", schemas, "--schema", name, f)
				line := strings.TrimSuffix(errOut, "\n")
				at := fmt.Sprintf(": %q: ", want.Pointer)
				if code != 1 || strings.Contains(line, "\n") || !strings.Contains(line, at) || !strings.HasSuffix(line, "/"+want.Rule+")") {
					t.Errorf("%s: exit %d, want 1 and one line at %q on %s, got:\n%s", filepath.Base(f), code, want.Pointer, want.Rule, errOut)
				}
			}
		})
	}
}

func TestRegistryHasAValidFixturePerEntityType(t *testing.T) {
	var sch struct {
		Properties struct {
			Type struct{ Enum []string }
		}
	}
	readJSON(t, filepath.Join(schemas, "registry-entity"+suffix), &sch)
	have := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(fixtures, "registry-entity", "valid", "*.json"))
	for _, f := range files {
		var doc struct{ Type string }
		readJSON(t, f, &doc)
		have[doc.Type] = true
	}
	types := sch.Properties.Type.Enum
	if len(types) < 15 {
		t.Errorf("the schema knows %d entity types, want the 15 the data contracts define", len(types))
	}
	for _, typ := range types {
		if !have[typ] {
			t.Errorf("no valid fixture for entity type %s", typ)
		}
	}
}

// A schema name is never joined to a path unless it is a plain name. Each row would reach a real
// schema file if it were, on Linux and on Windows, and each must get the name error, not a file error.
func TestSchemaNameCantLeaveTheFolder(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a", "b")
	writeFile(t, filepath.Join(root, "x"+suffix), `{}`)
	writeFile(t, filepath.Join(root, "a", "x"+suffix), `{}`)
	writeFile(t, filepath.Join(inside, "ok"+suffix), `{}`)
	writeFile(t, filepath.Join(inside, "c", "d"+suffix), `{}`)
	for _, name := range []string{
		"../x", "../../x", `..\x`, `..\..\x`, "c/d", `c\d`, "c/../ok", "./ok", "ok/", "ok" + suffix, "ok.",
		filepath.Join(root, "x"), filepath.ToSlash(filepath.Join(root, "x")), "/" + filepath.ToSlash(filepath.Join(root, "x")),
		"", ".", "..", "OK", "ok\x00", "ok ", "-ok",
	} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := cli(t, `{}`, "validate", "--schemas", inside, "--schema", name, "-")
			if code != 2 || out != "" || errOut != "ascc-schema: "+notAName+"\n" {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
	// The control: the same folder and document, asked for by a plain name.
	if code, _, errOut := cli(t, `{}`, "validate", "--schemas", inside, "--schema", "ok", "-"); code != 0 {
		t.Fatalf("control: exit %d: %s", code, errOut)
	}
}

func TestCantRunIsExit2(t *testing.T) {
	root := t.TempDir()
	notJSON := writeFile(t, filepath.Join(root, "broken.json"), `{"decision": `)
	writeFile(t, filepath.Join(root, "broken"+suffix), `{"type": 7}`)
	writeFile(t, filepath.Join(root, "twice"+suffix), `{"type": "object", "type": "string"}`)
	writeFile(t, filepath.Join(root, "huge"+suffix), `{"description": "`+strings.Repeat("a", maxBytes)+`"}`)
	big := writeFile(t, filepath.Join(root, "big.json"), `{`+response+`, "reasons2": "`+strings.Repeat("a", maxBytes)+`"}`)
	good := filepath.Join(fixtures, "pdp-response", "valid", "deny-with-remedy.json")
	on := func(args ...string) []string {
		return append([]string{"validate", "--schemas", schemas, "--schema", "pdp-response"}, args...)
	}
	tests := map[string]struct {
		args  []string
		stdin string
		want  string // in the one stderr line
	}{
		"no command":              {args: []string{}, want: "Usage:"},
		"unknown command":         {args: []string{"check"}, want: "unknown command"},
		"unknown flag":            {args: []string{"validate", "--strict", "--schema", "event", good}, want: "strict"},
		"no --schema":             {args: []string{"validate", "--schemas", schemas, good}, want: notAName},
		"unknown schema":          {args: []string{"validate", "--schemas", schemas, "--schema", "nope", good}, want: "no such schema"},
		"no file":                 {args: on(), want: "one file"},
		"two files":               {args: on(good, good), want: "one file"},
		"missing file":            {args: on(filepath.Join(root, "none.json")), want: "none.json"},
		"file isn't JSON":         {args: on(notJSON), want: "isn't valid JSON"},
		"stdin isn't JSON":        {args: on("-"), stdin: "not json", want: "isn't valid JSON"},
		"stdin is empty":          {args: on("-"), want: "isn't valid JSON"},
		"text after the document": {args: on("-"), stdin: `{` + response + `} {}`, want: "isn't valid JSON"},
		"file over 1 MiB":         {args: on(big), want: "larger than 1 MiB"},
		"stdin over 1 MiB":        {args: on("-"), stdin: strings.Repeat(" ", maxBytes) + `{` + response + `}`, want: "larger than 1 MiB"},
		"nested too deeply":       {args: on("-"), stdin: strings.Repeat("[", maxDepth+2), want: "nested too deeply"},
		"key twice, top level":    {args: on("-"), stdin: `{"decision": "deny", "decision": "allow"}`, want: `twice in the object at ""`},
		"key twice, nested":       {args: on("-"), stdin: `{` + response + `, "remedy": {"kind": "a", "kind": "b"}}`, want: `twice in the object at "/remedy"`},
		"key twice, in an array":  {args: on("-"), stdin: `{"reasons": ["a", {"b": {"c": 1, "c": 1}}]}`, want: `twice in the object at "/reasons/1/b"`},
		"list, no schemas":        {args: []string{"list", "--schemas", t.TempDir()}, want: "found 0 schemas"},
		"list, missing folder":    {args: []string{"list", "--schemas", filepath.Join(root, "none")}, want: "found 0 schemas"},
		"list, stray argument":    {args: []string{"list", "--schemas", schemas, "event"}, want: "takes no file"},
		"schema file isn't one":   {args: []string{"validate", "--schemas", root, "--schema", "broken", good}, want: "schema broken"},
		"schema with a key twice": {args: []string{"validate", "--schemas", root, "--schema", "twice", good}, want: `twice in the object at ""`},
		"schema over 1 MiB":       {args: []string{"validate", "--schemas", root, "--schema", "huge", good}, want: "larger than 1 MiB"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := cli(t, tc.stdin, tc.args...)
			oneLine := strings.Count(errOut, "\n") == 1 || name == "no command"
			if code != 2 || out != "" || !oneLine || !strings.Contains(errOut, tc.want) {
				t.Fatalf("exit %d, stdout %q, stderr %q, want exit 2 and one line with %q", code, out, errOut, tc.want)
			}
		})
	}
	// The control: a document exactly at the limit is read.
	pad := maxBytes - len(`{`+response+`}`)
	if code, _, errOut := check(t, "pdp-response", strings.Repeat(" ", pad)+`{`+response+`}`); code != 0 {
		t.Fatalf("a 1 MiB document: exit %d: %s", code, errOut)
	}
}

func TestValidateReadsStdin(t *testing.T) {
	code, out, errOut := check(t, "pdp-response", `{`+response+`}`)
	if code != 0 || !strings.Contains(out, "valid") || errOut != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, errOut = check(t, "pdp-response", strings.Replace(`{`+response+`}`, "deny", "ask", 1))
	if code != 1 || out != "" || !strings.HasPrefix(errOut, `-: "/decision": breaks enum`) || strings.Count(errOut, "\n") != 1 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	// A response that lost its obligations is not a valid response.
	code, _, errOut = check(t, "pdp-response", `{"decision": "allow", "policy_version": "p1", "determining_policies": [], "reasons": []}`)
	if code != 1 || !strings.Contains(errOut, "missing obligations") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

// A document can hold a secret in the very field that is wrong. No rule's failure, and no parse
// error, may repeat it. The library's own messages do quote values and numbers, so this is the test
// that keeps ascc-schema from ever printing one of them.
func TestOutputNeverRepeatsAValue(t *testing.T) {
	const marker = "MARKER-s3cr3t-do-not-print"
	const number = "4242424242424242"
	tests := map[string]struct{ schema, doc string }{
		"enum":            {"pdp-response", `{"decision": "` + marker + `"}`},
		"pattern":         {"pdp-response", `{"obligations": ["` + marker + `"]}`},
		"type":            {"pdp-response", `{"degraded": "` + marker + `"}`},
		"unknown field":   {"pdp-response", `{"note": "` + marker + `"}`},
		"const":           {"event", `{"schema": "` + marker + `"}`},
		"thumbprint":      {"transaction-token", `{"cnf": {"jkt": "` + marker + `"}}`},
		"both bindings":   {"transaction-token", `{"cnf": {"jkt": "` + marker + `", "x5t#S256": "` + marker + `"}}`},
		"minimum":         {"transaction-token", `{"iat": -` + number + `}`},
		"maximum":         {"transaction-token", `{"exp": ` + number + `}`},
		"not an integer":  {"transaction-token", `{"exp": ` + number + `.5}`},
		"maxLength":       {"plugin-manifest", `{"capabilities": {"network_hosts": ["` + marker + strings.Repeat("a", 300) + `"]}}`},
		"uniqueItems":     {"plugin-manifest", `{"capabilities": {"network_hosts": ["` + marker + `", "` + marker + `"]}}`},
		"maxItems":        {"plugin-manifest", `{"capabilities": {"events": [` + strings.Repeat(`"`+marker+`",`, 65) + `"x"]}}`},
		"array for text":  {"pdp-request", `{"action": ["` + marker + `"]}`},
		"key twice":       {"pdp-response", `{"decision": "` + marker + `", "decision": "` + marker + `"}`},
		"not JSON":        {"pdp-response", `{"decision": ` + marker},
		"trailing text":   {"pdp-response", `{` + response + `} ` + marker},
		"unclosed string": {"pdp-response", `{"decision": "` + marker},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := check(t, tc.schema, tc.doc)
			if code == 0 || errOut == "" {
				t.Fatalf("exit %d, stderr %q: the document should have failed", code, errOut)
			}
			if strings.Contains(out+errOut, marker) || strings.Contains(out+errOut, number) {
				t.Fatalf("the value is in the output:\n%s%s", out, errOut)
			}
		})
	}
}

// A property name is part of the pointer, so it is printed, but quoted: a name with a newline or an
// escape sequence in it can't start a line of its own or drive the terminal.
func TestPropertyNamesAreQuoted(t *testing.T) {
	code, _, errOut := check(t, "pdp-response", `{`+response+`, "a\nfake.json: valid\u001b[2Jb": 1, "c\rd~e/f": 2}`)
	if code != 1 || strings.Count(errOut, "\n") != 2 || strings.ContainsAny(errOut, "\x1b\r") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{`"/a\nfake.json: valid\x1b[2Jb": unknown field`, `"/c\rd~0e~1f": unknown field`} {
		if !strings.Contains(errOut, want) {
			t.Errorf("no %s in:\n%s", want, errOut)
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(errOut, "\n"), "\n") {
		if !strings.HasPrefix(line, `-: "/`) {
			t.Errorf("a line doesn't start with the file and a quoted pointer: %q", line)
		}
	}
}

// A schema that points outside itself doesn't compile: nothing is fetched and no other file is read.
func TestNoRefIsLoaded(t *testing.T) {
	dir := t.TempDir()
	other := writeFile(t, filepath.Join(dir, "other.json"), `{}`)
	for name, ref := range map[string]string{
		"https":    "https://example.com/remote.schema.json",
		"http":     "http://example.com/remote.schema.json",
		"file":     "file:///" + strings.TrimPrefix(filepath.ToSlash(other), "/"),
		"relative": "other.json",
	} {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": ref})
			writeFile(t, filepath.Join(dir, name+suffix), string(body))
			code, out, errOut := cli(t, `{}`, "validate", "--schemas", dir, "--schema", name, "-")
			if code != 2 || out != "" || !strings.Contains(errOut, "loads no schema but the one named") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
}
