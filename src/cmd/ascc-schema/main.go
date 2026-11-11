// Command ascc-schema checks a JSON document against one of the platform contract schemas in
// schemas/platform/v1alpha1: registry entities, edges, the event envelope, the two tokens' claims, the
// plugin manifest and the PDP request and response. It is a development and CI tool, separate from
// ascc: nothing in the ascc binary imports it or the JSON Schema library.
//
// A failure is reported as the JSON pointer of the failing location and the rule it broke, never the
// value found there: the documents it checks can hold sensitive data.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

const usage = `ascc-schema checks a JSON document against a platform contract schema.

Usage:
  ascc-schema list     [--schemas <dir>]                      print the schema names
  ascc-schema validate --schema <name> [--schemas <dir>] <file|->

--schemas defaults to schemas/platform/v1alpha1. A name is the file name without .schema.json.

Exit codes: 0 the document is valid; 1 it isn't, with one line per failure on stderr (the JSON
pointer and the rule, never the value); 2 ascc-schema couldn't run, which includes a document
that isn't JSON, is larger than 1 MiB or has the same key twice in one object.
`

// maxBytes caps a document and a schema file. maxDepth caps nesting, below the JSON decoder's own limit.
const (
	maxBytes = 1 << 20
	maxDepth = 512
)

const (
	suffix   = ".schema.json"
	notAName = "--schema takes a schema name such as event; run 'ascc-schema list'"
)

// A schema name is a file name stem and nothing else, so it can't walk out of the schema folder.
var schemaName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil { // a panic's text could quote the document, so it isn't printed
			fmt.Fprintln(stderr, "ascc-schema: internal error")
			code = 2
		}
	}()
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "ascc-schema: "+format+"\n", a...)
		return 2
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("schemas", filepath.Join("schemas", "platform", "v1alpha1"), "")
	name := fs.String("schema", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return fail("%v", err)
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "list":
		files, _ := filepath.Glob(filepath.Join(*dir, "*"+suffix))
		if fs.NArg() != 0 || len(files) == 0 {
			return fail("list takes no file, and found %d schemas in %s", len(files), *dir)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintln(stdout, strings.TrimSuffix(filepath.Base(f), suffix))
		}
		return 0
	case "validate":
		if !schemaName.MatchString(*name) {
			return fail(notAName)
		}
		if fs.NArg() != 1 {
			return fail("validate takes one file, or - for stdin")
		}
		sch, err := compile(*dir, *name)
		if err != nil {
			return fail("schema %s: %s", *name, firstLine(err))
		}
		file := fs.Arg(0)
		doc, err := load(file, stdin)
		if err != nil {
			return fail("%s", firstLine(err))
		}
		err = sch.Validate(doc)
		if err == nil {
			fmt.Fprintf(stdout, "%s: valid %s\n", file, *name)
			return 0
		}
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return fail("%s couldn't be checked", file)
		}
		for _, line := range failures(ve) {
			fmt.Fprintf(stderr, "%s: %s\n", file, line)
		}
		return 1
	}
	return fail("unknown command %q; run 'ascc-schema help'", args[0])
}

// refuse is the compiler's loader. Every schema is self-contained, so a $ref that needs loading, from a
// URL or from a file, is a mistake or an attack, and the library's default loader would read any file.
type refuse struct{}

func (refuse) Load(url string) (any, error) {
	return nil, errors.New("ascc-schema loads no schema but the one named")
}

// load reads one JSON value from a file, or from stdin for "-". It refuses more than maxBytes and an
// object with a key twice: Go keeps the last of two, another parser may keep the first, and a document
// two readers disagree about can't be called valid. Its errors never quote the input.
func load(file string, stdin io.Reader) (any, error) {
	r := stdin
	if file != "-" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s couldn't be read", file)
	}
	if len(raw) > maxBytes {
		return nil, fmt.Errorf("%s is larger than 1 MiB", file)
	}
	if err := noRepeatedKey(json.NewDecoder(bytes.NewReader(raw)), nil); err != nil {
		return nil, fmt.Errorf("%s %w", file, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil { // the decoder's message can quote the input, so it isn't repeated
		return nil, fmt.Errorf("%s %w", file, errNotJSON)
	}
	return doc, nil
}

var errNotJSON = errors.New("isn't valid JSON")

// noRepeatedKey walks the next JSON value's tokens and names the first object that has a key twice,
// by its pointer and without the key.
func noRepeatedKey(dec *json.Decoder, path []string) error {
	if len(path) > maxDepth {
		return errors.New("is nested too deeply")
	}
	tok, err := dec.Token()
	if err != nil {
		return errNotJSON
	}
	delim, _ := tok.(json.Delim)
	seen := map[string]bool{}
	for i := 0; (delim == '{' || delim == '[') && dec.More(); i++ {
		key := strconv.Itoa(i)
		if delim == '{' {
			k, err := dec.Token()
			if err != nil {
				return errNotJSON
			}
			if key, _ = k.(string); seen[key] {
				return fmt.Errorf("has the same key twice in the object at %s", pointer(path))
			}
			seen[key] = true
		}
		if err := noRepeatedKey(dec, append(path[:len(path):len(path)], key)); err != nil {
			return err
		}
	}
	if delim == '{' || delim == '[' {
		if _, err := dec.Token(); err != nil { // the closing bracket
			return errNotJSON
		}
	}
	return nil
}

func compile(dir, name string) (*jsonschema.Schema, error) {
	path := filepath.Join(dir, name+suffix)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("no such schema; run 'ascc-schema list'")
	}
	doc, err := load(path, nil)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(refuse{})
	url := "ascc-schema:///" + name + suffix
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

// failures flattens a validation error into one line per failing rule: the JSON pointer of the place
// in the document, the keyword, and where that keyword sits in the schema. Nothing in a line comes from
// a value in the document. Property names do: a pointer is made of them.
func failures(root *jsonschema.ValidationError) []string {
	seen := map[string]bool{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		_, at, _ := strings.Cut(e.SchemaURL, "#")
		rule := strings.Join(e.ErrorKind.KeywordPath(), "/")
		switch k := e.ErrorKind.(type) {
		case *kind.AdditionalProperties:
			for _, p := range k.Properties {
				seen[fmt.Sprintf("%s: unknown field (schema #%s/additionalProperties)", pointer(append(e.InstanceLocation[:len(e.InstanceLocation):len(e.InstanceLocation)], p)), at)] = true
			}
			return
		case *kind.Required: // the names come from the schema
			seen[fmt.Sprintf("%s: missing %s (schema #%s/required)", pointer(e.InstanceLocation), strings.Join(k.Missing, ", "), at)] = true
			return
		case *kind.Not:
			rule = "not"
		}
		seen[fmt.Sprintf("%s: breaks %s (schema #%s/%s)", pointer(e.InstanceLocation), rule, at, rule)] = true
	}
	walk(root)
	lines := make([]string, 0, len(seen))
	for l := range seen {
		lines = append(lines, l)
	}
	sort.Strings(lines)
	return lines
}

// pointer is the RFC 6901 pointer, quoted so a property name can't break the line.
func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(t))
	}
	return fmt.Sprintf("%q", b.String())
}

func firstLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}
