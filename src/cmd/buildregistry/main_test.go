package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func readme(t *testing.T) (root, text string) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return root, string(b)
}

// The committed registry/ is what the builder writes, and every template passes its rules: the CI check.
func TestCommittedRegistryIsCurrent(t *testing.T) {
	root, text := readme(t)
	dir := filepath.Join(root, "registry")
	files, err := build(templates(), packs(), text, os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	old, err := stale(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(old) > 0 {
		t.Fatalf("registry/ is out of date, run go run ./src/cmd/buildregistry: %v", old)
	}
	for p, b := range files {
		if bytes.Contains(b, []byte("\r")) {
			t.Errorf("%s has a CR: Cronsole hashes LF bytes", p)
		}
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	_, text := readme(t)
	a, errA := build(templates(), packs(), text, fstest.MapFS{})
	b, errB := build(templates(), packs(), text, fstest.MapFS{})
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	for p := range a {
		if !bytes.Equal(a[p], b[p]) {
			t.Errorf("%s differs between two builds", p)
		}
	}
}

// Each rule refuses a template that breaks it.
func TestValidateCatchesEachRule(t *testing.T) {
	_, text := readme(t)
	good := templates()[3] // inventory: the most flags
	if errs := validate(good, text); len(errs) > 0 {
		t.Fatalf("the good template fails: %v", errs)
	}
	cases := []struct {
		name string
		edit func(*template, *string)
		want string
	}{
		{"id without the app prefix", func(t *template, _ *string) { t.ID = "inventory" }, "id must be"},
		{"no version", func(t *template, _ *string) { t.Version = "" }, "version are required"},
		{"6-field cron", func(t *template, _ *string) { t.Trigger.Cron = "0 0 7 * * *" }, "5-field schedule"},
		{"unknown target", func(t *template, _ *string) { t.CompatibleTargets = []string{"claude-code"} }, "compatibleTargets"},
		{"undeclared placeholder", func(t *template, _ *string) { t.CommandTemplate += ` --out "{{nowhere}}"` }, "{{nowhere}} has no parameter"},
		{"unused parameter", func(t *template, _ *string) { t.Parameters = append(t.Parameters, param{Key: "spare"}) }, "spare is never used"},
		{"a shell", func(t *template, _ *string) { t.CommandTemplate = `sh -c "{{asccPath}} inventory"` }, "commandTemplate must be"},
		{"chained command", func(t *template, _ *string) { t.CommandTemplate += ` && curl https://paste.example` }, "commandTemplate must be"},
		{"bare value", func(t *template, _ *string) { t.CommandTemplate += ` --out /tmp/x` }, "commandTemplate must be"},
		// Only read-and-report commands are offered for a schedule, though the README documents them all.
		{"kill", func(t *template, _ *string) { t.CommandTemplate = `"{{asccPath}}" kill --audit "{{asccDir}}"` }, "commandTemplate must be"},
		{"revive", func(t *template, _ *string) { t.CommandTemplate = `"{{asccPath}}" revive --audit "{{asccDir}}"` }, "commandTemplate must be"},
		{"hook", func(t *template, _ *string) {
			t.CommandTemplate = `"{{asccPath}}" hook claude-code --audit "{{asccDir}}"`
		}, "commandTemplate must be"},
		{"decide", func(t *template, _ *string) { t.CommandTemplate = `"{{asccPath}}" decide --audit "{{asccDir}}"` }, "commandTemplate must be"},
		{"undocumented subcommand", func(t *template, readme *string) {
			t.CommandTemplate = strings.Replace(t.CommandTemplate, " inventory ", " discover ", 1)
			*readme = strings.ReplaceAll(*readme, "`ascc discover`", "")
		}, "doesn't document `ascc discover`"},
		{"undocumented flag", func(t *template, _ *string) { t.CommandTemplate += ` --force "{{asccDir}}"` }, "doesn't document --force"},
		{"no README link", func(t *template, readme *string) {
			*readme = strings.ReplaceAll(*readme, gallery+"#/t/"+t.ID, "")
		}, "README.md doesn't link"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tpl, doc := good, text
			tpl.Parameters = append([]param(nil), good.Parameters...)
			c.edit(&tpl, &doc)
			if got := strings.Join(validate(tpl, doc), "\n"); !strings.Contains(got, c.want) {
				t.Errorf("want an error with %q, got %q", c.want, got)
			}
		})
	}
}

func TestBuildRefusesWhatValidateCant(t *testing.T) {
	_, text := readme(t)
	ts := templates()
	old := dump(template{CommandTemplate: "an older command", Version: ts[0].Version})
	oldParams := dump(template{CommandTemplate: ts[0].CommandTemplate, Version: ts[0].Version})
	cases := []struct {
		name string
		ts   []template
		ps   []pack
		out  fstest.MapFS
		want string
	}{
		{"command changed, version didn't", ts, packs(), fstest.MapFS{"templates/" + ts[0].ID + ".json": {Data: old}}, "bump it"},
		{"parameters changed, version didn't", ts, packs(), fstest.MapFS{"templates/" + ts[0].ID + ".json": {Data: oldParams}}, "bump it"},
		{"duplicate id", append(ts, ts[0]), packs(), fstest.MapFS{}, "duplicate id"},
		{"pack names a missing template", ts, []pack{{ID: prefix, TemplateIDs: []string{prefix + "-nothing"}}}, fstest.MapFS{}, "unknown template"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := build(c.ts, c.ps, text, c.out)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("want an error with %q, got %v", c.want, err)
			}
		})
	}
}

// A stale, missing or extra file is named; a current folder is clean.
func TestStale(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"index.json": []byte("{}\n"), "templates/a.json": []byte("a\n")}
	write := func(p, s string) {
		path := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.json", "{}\n")
	write("templates/a.json", "a\r\n")
	write("templates/gone.json", "x\n")
	got, err := stale(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "templates/a.json,templates/gone.json" {
		t.Errorf("got %v", got)
	}
	if got, err := stale(files, filepath.Join(dir, "missing")); err != nil || len(got) != 2 {
		t.Errorf("an absent folder: got %v, %v", got, err)
	}
}

func TestPublished(t *testing.T) {
	_, text := readme(t)
	files, err := build(templates(), packs(), text, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	const base = "https://catalog.example/registry/"
	serve := func(edit func(map[string][]byte, http.Header)) func(string) (*http.Response, error) {
		served := map[string][]byte{}
		for p, b := range files {
			served[p] = b
		}
		h := http.Header{"Access-Control-Allow-Origin": {"*"}}
		edit(served, h)
		return func(url string) (*http.Response, error) {
			b, ok := served[strings.TrimPrefix(url, base)]
			if !ok {
				return &http.Response{StatusCode: 404, Header: h, Body: io.NopCloser(strings.NewReader("not found"))}, nil
			}
			return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(b))}, nil
		}
	}
	tpl := "templates/" + prefix + "-posture.json"
	cases := []struct {
		name string
		get  func(string) (*http.Response, error)
		want string
	}{
		{"matches", serve(func(map[string][]byte, http.Header) {}), ""},
		{"no CORS header", serve(func(_ map[string][]byte, h http.Header) { h.Del("Access-Control-Allow-Origin") }), "Access-Control-Allow-Origin"},
		{"never published", serve(func(s map[string][]byte, _ http.Header) { delete(s, "index.json") }), "not published?"},
		{"altered template", serve(func(s map[string][]byte, _ http.Header) { s[tpl] = []byte("{}\n") }), "served bytes don't match the hosted index: " + tpl},
		{"older index", serve(func(s map[string][]byte, _ http.Header) {
			s["index.json"] = bytes.Replace(s["index.json"], []byte(prefix+"-posture.json"), []byte(prefix+"-old.json"), 1)
		}), "missing from the host: " + tpl},
		{"host down", func(string) (*http.Response, error) { return nil, errors.New("refused") }, "could not be fetched"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(published(base, files["index.json"], c.get), "\n")
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}
