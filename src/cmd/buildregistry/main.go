// Command buildregistry writes ascc's Cronsole template catalog (Registry v1) to registry/, the
// unattended ascc runs a user would schedule. The publish workflow mirrors that folder into the
// ai-security-command-center-site repo, which serves it at
// https://ai-security-command-center.ai-automation-tools.dev/registry/, where the Cronsole gallery
// (https://cronsole.ai-automation-tools.dev/) reads it.
//
//	registry/templates/<id>.json   one template per file
//	registry/packs/<id>.json       an importable bundle of all of them
//	registry/index.json            every template and pack with the sha256 of its exact bytes
//
// The bytes are deterministic and LF, with no timestamp: Cronsole checks each file's sha256. Edit the
// templates here, never the output. Modelled on edge-spectrum's site/scripts/build-registry.mjs.
//
//	go run ./src/cmd/buildregistry              write registry/
//	go run ./src/cmd/buildregistry -check       exit 1 if registry/ or a README link is stale
//	go run ./src/cmd/buildregistry -published   exit 1 if the hosted catalog isn't registry/ (network)
//
// This is a build tool, not part of the ascc binary: -published is the one place it reaches the network.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	hosted  = "https://ai-security-command-center.ai-automation-tools.dev/registry/"
	gallery = "https://cronsole.ai-automation-tools.dev/"
	prefix  = "ai-security-command-center"
)

type param struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required"`
	Help     string `json:"help"`
}

type trigger struct {
	Kind string `json:"kind"`
	Cron string `json:"cron"`
}

type execution struct {
	TimeoutSec int `json:"timeoutSec"`
	Retries    int `json:"retries"`
}

type template struct {
	SchemaVersion     string    `json:"schemaVersion"`
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	Runtime           string    `json:"runtime"`
	OS                string    `json:"os"`
	Category          string    `json:"category"`
	Tags              []string  `json:"tags"`
	Icon              string    `json:"icon"`
	Trigger           trigger   `json:"trigger"`
	CommandTemplate   string    `json:"commandTemplate"`
	Parameters        []param   `json:"parameters"`
	CompatibleTargets []string  `json:"compatibleTargets"`
	Execution         execution `json:"execution"`
	Author            string    `json:"author"`
	Version           string    `json:"version"`
}

type pack struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TemplateIDs []string `json:"-"`
}

var (
	asccPath = param{Key: "asccPath", Label: "ascc binary", Type: "path", Default: "ascc", Required: true,
		Help: "The ascc binary (`go install github.com/ai-automation-tools/ai-security-command-center/src/cmd/ascc@latest`). `ascc` works when Go's bin folder is on the scheduler's PATH; otherwise give its full path."}
	asccDir = param{Key: "asccDir", Label: "ascc folder", Type: "path", Required: true,
		Help: "The folder you run ascc in: it holds ascc.registry.json, policies/ and .ascc/audit.jsonl."}
	projectDir = param{Key: "projectDir", Label: "Project folder", Type: "path", Required: true,
		Help: "The project whose .claude/settings.json is read, as Claude Code reads it there. Managed and user settings are read from their usual places, for the account the task runs as."}
	outputFile = param{Key: "outputFile", Label: "Output file", Type: "path", Required: true,
		Help: "Where the JSON document goes. It is overwritten on every run, so copy it elsewhere if you want history. It can't be ascc's own registry, policies or audit log."}

	// Every template's command is the ascc binary, then one subcommand the README documents, then --flag
	// "{{param}}" pairs. These run on other people's machines: nothing else may get in.
	commandShape = regexp.MustCompile(`^"\{\{asccPath\}\}" (audit verify|posture claude-code|discover|inventory)( --[a-z-]+ "\{\{\w+\}\}[\w./-]*")*$`)
	targets      = map[string]bool{"windows": true, "cronsole-native": true, "macos": true, "linux": true}

	// The files under asccDir, passed explicitly because ascc's defaults are relative to the current folder,
	// which a scheduler doesn't set.
	stateFlags = `--registry "{{asccDir}}/ascc.registry.json" --policies "{{asccDir}}/policies" --audit "{{asccDir}}/.ascc/audit.jsonl"`
	needs      = " Makes no network call. Needs the ascc binary, built with Go 1.26 or newer, and a folder set up as the README's quick start shows."
)

func templates() []template {
	with := func(t template) template {
		t.SchemaVersion, t.Runtime, t.OS, t.Author = "1.0", "executable", "cross-platform", "ai-automation-tools"
		t.CompatibleTargets = []string{"windows", "cronsole-native", "linux", "macos"}
		t.Tags = append([]string{prefix, "ai-agents", "claude-code"}, t.Tags...)
		return t
	}
	return []template{
		with(template{
			ID:   prefix + "-audit-verify",
			Name: "AI Security Command Center: Audit Log Check",
			Description: "Runs `ascc audit verify` on the hash-chained log the Claude Code hook writes every decision to, and fails " +
				"the run when a line was edited, removed or reordered, naming the first one. Reads the log, writes nothing. " +
				"It checks the chain, not the head: a log cut short, or emptied, still passes, and only a missing file " +
				"fails. The head moves with every hook decision, and a scheduled run has nowhere safe to keep it." + needs,
			Category: "monitoring", Tags: []string{"audit", "tamper-check"}, Icon: "ShieldCheck",
			Trigger:         trigger{"schedule", "17 * * * *"},
			CommandTemplate: `"{{asccPath}}" audit verify --audit "{{asccDir}}/.ascc/audit.jsonl"`,
			Parameters:      []param{asccPath, asccDir},
			// One retry: a read that lands mid-append sees a half-written last line.
			Execution: execution{TimeoutSec: 60, Retries: 1},
			Version:   "0.1.0",
		}),
		with(template{
			ID:   prefix + "-posture",
			Name: "AI Security Command Center: Hook Posture Check",
			Description: "Runs `ascc posture claude-code` and fails the run when a Claude Code agent in the ascc registry has a " +
				"hook that doesn't enforce: not in managed settings, a matcher other than `*`, no `\"onFailure\": \"block\"`, " +
				"not exec form, something switching it off, or the binary missing. It prints the reason for each agent. " +
				"Reads the settings files and never writes them." + needs,
			Category: "monitoring", Tags: []string{"posture", "hooks"}, Icon: "ShieldAlert",
			Trigger:         trigger{"schedule", "0 7 * * *"},
			CommandTemplate: `"{{asccPath}}" posture claude-code --registry "{{asccDir}}/ascc.registry.json" --project "{{projectDir}}"`,
			Parameters:      []param{asccPath, asccDir, projectDir},
			Execution:       execution{TimeoutSec: 60},
			Version:         "0.1.0",
		}),
		with(template{
			ID:   prefix + "-discover",
			Name: "AI Security Command Center: Unregistered Agent Discovery",
			Description: "Runs `ascc discover` over a folder of projects and writes an Agent Security Suite inventory document: " +
				"every Claude Code setup, hook agent and MCP server the ascc registry doesn't list, as an `unmonitored` " +
				"identity with an `ascc/unregistered` finding, and credential-looking variable names as candidate identities. " +
				"Names only, never a command, URL or value. Fails the run when a file under the folder couldn't be read, " +
				"and names it." + needs,
			Category: "monitoring", Tags: []string{"discovery", "mcp", "inventory"}, Icon: "Radar",
			Trigger:         trigger{"schedule", "0 8 * * 1"},
			CommandTemplate: `"{{asccPath}}" discover ` + stateFlags + ` --root "{{projectsDir}}" --out "{{outputFile}}"`,
			Parameters: []param{asccPath, asccDir,
				{Key: "projectsDir", Label: "Projects folder", Type: "path", Required: true,
					Help: "The folder searched for agent config files, every folder below it included."},
				outputFile},
			Execution: execution{TimeoutSec: 300},
			Version:   "0.1.0",
		}),
		with(template{
			ID:   prefix + "-inventory",
			Name: "AI Security Command Center: Agent Inventory",
			Description: "Runs `ascc inventory` and writes the registry as an Agent Security Suite inventory document: each agent " +
				"labelled `enforced`, `observed` or `unmonitored` from the audit log and the posture check, never from a " +
				"declaration, then the non-human identities the agents use, with `ascc/*` findings such as an agent with " +
				"no owner or an overdue credential." + needs,
			Category: "monitoring", Tags: []string{"inventory", "nhi"}, Icon: "ClipboardList",
			Trigger:         trigger{"schedule", "30 7 * * *"},
			CommandTemplate: `"{{asccPath}}" inventory ` + stateFlags + ` --project "{{projectDir}}" --out "{{outputFile}}"`,
			Parameters:      []param{asccPath, asccDir, projectDir, outputFile},
			Execution:       execution{TimeoutSec: 60},
			Version:         "0.1.0",
		}),
	}
}

func packs() []pack {
	return []pack{{
		ID:   prefix,
		Name: "AI Security Command Center: Unattended Checks",
		Description: "The ascc runs worth a schedule: the audit log's hash chain, the hook's posture, agents the registry " +
			"doesn't know, and the coverage inventory.",
		TemplateIDs: []string{prefix + "-audit-verify", prefix + "-posture", prefix + "-discover", prefix + "-inventory"},
	}}
}

var (
	placeholder = regexp.MustCompile(`\{\{(\w+)\}\}`)
	flagName    = regexp.MustCompile(` (--[a-z-]+) `)
	idShape     = regexp.MustCompile(`^` + prefix + `(-[a-z0-9]+)+$`)
)

// validate returns every rule t breaks. readme is the README's text: a template may only run a command,
// with flags, that the README documents, and the README must link the template's gallery page.
func validate(t template, readme string) []string {
	var errs []string
	req := func(ok bool, msg string) {
		if !ok {
			errs = append(errs, t.ID+": "+msg)
		}
	}
	req(idShape.MatchString(t.ID), "id must be "+prefix+"-<name>, so it can't collide with Cronsole's ids")
	req(t.SchemaVersion == "1.0" && t.Name != "" && t.Version != "", "schemaVersion 1.0, a name and a version are required")
	req(t.Trigger.Kind == "schedule" && len(strings.Fields(t.Trigger.Cron)) == 5, "trigger must be a 5-field schedule")
	ok := len(t.CompatibleTargets) > 0
	for _, x := range t.CompatibleTargets {
		ok = ok && targets[x]
	}
	req(ok, "compatibleTargets must be windows, cronsole-native, linux or macos")
	keys := map[string]bool{}
	for _, p := range t.Parameters {
		keys[p.Key] = true
	}
	used := map[string]bool{}
	for _, m := range placeholder.FindAllStringSubmatch(t.CommandTemplate, -1) {
		used[m[1]] = true
		req(keys[m[1]], "placeholder {{"+m[1]+"}} has no parameter")
	}
	for k := range keys {
		req(used[k], "parameter "+k+" is never used")
	}
	m := commandShape.FindStringSubmatch(t.CommandTemplate)
	req(m != nil, `commandTemplate must be "{{asccPath}}", an ascc subcommand, then --flag "{{param}}" pairs`)
	if m != nil {
		req(strings.Contains(readme, "`ascc "+m[1]+"`"), "the README doesn't document `ascc "+m[1]+"`")
	}
	for _, f := range flagName.FindAllStringSubmatch(t.CommandTemplate, -1) {
		req(strings.Contains(readme, "`"+f[1]+" <"), "the README doesn't document "+f[1])
	}
	req(strings.Contains(readme, gallery+"#/t/"+t.ID), "README.md doesn't link "+gallery+"#/t/"+t.ID)
	return errs
}

func dump(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		panic(err) // only plain structs go in
	}
	return b.Bytes()
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// build returns every file of the catalog by its path under registry/, or every rule a template breaks.
// out is the current registry/ folder: a template whose command changed must have a new version, since
// a user who imported it compares versions, not commands. The baseline is the committed copy: a change
// that also deletes registry/ has none.
func build(ts []template, ps []pack, readme string, out fs.FS) (map[string][]byte, error) {
	var errs []string
	byID := map[string]template{}
	for _, t := range ts {
		errs = append(errs, validate(t, readme)...)
		if _, dup := byID[t.ID]; dup {
			errs = append(errs, t.ID+": duplicate id")
		}
		byID[t.ID] = t
		if old, err := fs.ReadFile(out, "templates/"+t.ID+".json"); err == nil {
			var prev template
			if json.Unmarshal(old, &prev) == nil && prev.Version == t.Version &&
				(prev.CommandTemplate != t.CommandTemplate || !reflect.DeepEqual(prev.Parameters, t.Parameters)) {
				errs = append(errs, t.ID+": commandTemplate or parameters changed but version is still "+t.Version+"; bump it")
			}
		}
	}
	for _, p := range ps {
		for _, id := range p.TemplateIDs {
			if _, ok := byID[id]; !ok {
				errs = append(errs, "pack "+p.ID+": unknown template "+id)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.New("registry validation failed:\n  " + strings.Join(errs, "\n  "))
	}

	files := map[string][]byte{}
	type entry struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		Description       string   `json:"description"`
		Category          string   `json:"category"`
		Tags              []string `json:"tags"`
		Runtime           string   `json:"runtime"`
		OS                string   `json:"os"`
		CompatibleTargets []string `json:"compatibleTargets"`
		Path              string   `json:"path"`
		SHA256            string   `json:"sha256"`
	}
	type packEntry struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		TemplateIDs []string `json:"templateIds"`
		Path        string   `json:"path"`
		SHA256      string   `json:"sha256"`
	}
	sorted := append([]template(nil), ts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	index := struct {
		RegistryVersion string      `json:"registryVersion"`
		Templates       []entry     `json:"templates"`
		Packs           []packEntry `json:"packs"`
	}{RegistryVersion: "1.0"}
	for _, t := range sorted {
		path, body := "templates/"+t.ID+".json", dump(t)
		files[path] = body
		index.Templates = append(index.Templates, entry{t.ID, t.Name, t.Description, t.Category, t.Tags, t.Runtime, t.OS, t.CompatibleTargets, path, sum(body)})
	}
	for _, p := range ps {
		var members []template
		for _, id := range p.TemplateIDs {
			members = append(members, byID[id])
		}
		path := "packs/" + p.ID + ".json"
		body := dump(struct {
			CronsoleCatalogVersion string     `json:"cronsoleCatalogVersion"`
			Pack                   pack       `json:"pack"`
			Templates              []template `json:"templates"`
		}{"1.0", p, members})
		files[path] = body
		index.Packs = append(index.Packs, packEntry{p.ID, p.Name, p.Description, p.TemplateIDs, path, sum(body)})
	}
	files["index.json"] = dump(index)
	return files, nil
}

// stale lists the paths under dir that differ from files, are missing, or shouldn't be there.
func stale(files map[string][]byte, dir string) ([]string, error) {
	var out []string
	for p, b := range files {
		if have, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))); err != nil || !bytes.Equal(have, b) {
			out = append(out, p)
		}
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if _, ok := files[filepath.ToSlash(rel)]; !ok {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	sort.Strings(out)
	return out, err
}

// published compares the catalog served at base with registry/index.json: every path and sha256 in
// both indexes, and the sha256 of every served file. A merge that never got mirrored to the site repo
// looks fine from inside this repo; this is the check that can see it. It also wants the CORS header
// the Cronsole gallery needs to read the catalog from another origin.
func published(base string, local []byte, get func(string) (*http.Response, error)) []string {
	type idx struct {
		Templates []struct{ Path, SHA256 string } `json:"templates"`
		Packs     []struct{ Path, SHA256 string } `json:"packs"`
	}
	entries := func(b []byte) (map[string]string, error) {
		var i idx
		if err := json.Unmarshal(b, &i); err != nil {
			return nil, err
		}
		m := map[string]string{}
		for _, e := range append(i.Templates, i.Packs...) {
			m[e.Path] = e.SHA256
		}
		return m, nil
	}
	fetch := func(path string) (*http.Response, []byte, error) {
		res, err := get(base + path)
		if err != nil {
			return nil, nil, err
		}
		defer res.Body.Close()
		b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return res, b, err
	}
	var problems []string
	want, err := entries(local)
	if err != nil {
		return []string{"registry/index.json is not JSON: run go run ./src/cmd/buildregistry"}
	}
	res, body, err := fetch("index.json")
	if err != nil {
		return []string{"index.json could not be fetched: " + err.Error()}
	}
	if res.Header.Get("Access-Control-Allow-Origin") != "*" {
		problems = append(problems, "index.json is served without Access-Control-Allow-Origin: *")
	}
	live, err := entries(body)
	if err != nil || res.StatusCode != http.StatusOK {
		return append(problems, fmt.Sprintf("index.json is not the catalog (HTTP %d): not published?", res.StatusCode))
	}
	for p, h := range want {
		if lh, ok := live[p]; !ok {
			problems = append(problems, "missing from the host: "+p)
		} else if lh != h {
			problems = append(problems, "different sha256 in the hosted index: "+p)
		}
	}
	for p, h := range live {
		if _, ok := want[p]; !ok {
			problems = append(problems, "on the host but not in registry/: "+p)
		}
		if _, b, err := fetch(p); err != nil || sum(b) != h {
			problems = append(problems, "served bytes don't match the hosted index: "+p)
		}
	}
	sort.Strings(problems)
	return problems
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

func main() {
	check := flag.Bool("check", false, "exit 1 if registry/ or a README link is stale; write nothing")
	pub := flag.Bool("published", false, "exit 1 if the catalog at the hosted URL isn't registry/ (reaches the network)")
	flag.Parse()
	if err := run(*check, *pub, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "buildregistry: "+err.Error())
		os.Exit(1)
	}
}

func run(check, pub bool, base string) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "registry")
	if pub {
		if base == "" {
			base = hosted
		}
		base = strings.TrimRight(base, "/") + "/"
		local, err := os.ReadFile(filepath.Join(dir, "index.json"))
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 30 * time.Second}
		problems := published(base, local, client.Get)
		for _, p := range problems {
			fmt.Println("DRIFT " + p)
		}
		if len(problems) > 0 {
			return fmt.Errorf("%s: %d problem(s)", base, len(problems))
		}
		fmt.Println(base + ": matches registry/")
		return nil
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return err
	}
	files, err := build(templates(), packs(), string(readme), os.DirFS(dir))
	if err != nil {
		return err
	}
	old, err := stale(files, dir)
	if err != nil {
		return err
	}
	if check {
		for _, p := range old {
			fmt.Println("stale: registry/" + p)
		}
		if len(old) > 0 {
			return errors.New("registry/ is out of date: run go run ./src/cmd/buildregistry")
		}
		fmt.Printf("registry OK: %d files current\n", len(files))
		return nil
	}
	for _, p := range old {
		path := filepath.Join(dir, filepath.FromSlash(p))
		b, ok := files[p]
		if !ok {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("registry: %d template(s), %d pack(s) -> registry/\n", len(templates()), len(packs()))
	return nil
}
