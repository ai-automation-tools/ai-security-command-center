package claudecode

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
)

// LocationsVersion names the list below. It changes whenever the list does, and every discover
// report carries it, so two reports can be compared knowing what each one looked for.
const LocationsVersion = "2026-11-10"

// Locations are the agent config files discover knows, as paths relative to any folder.
var Locations = []string{
	".claude/settings.json",       // project hooks
	".claude/settings.local.json", // one user's hooks for a project
	".mcp.json",                   // a project's MCP servers
	".claude.json",                // a user's MCP servers, global and per project
	"managed-settings.json",       // an organization's hooks
	"managed-mcp.json",            // an organization's MCP servers
}

// IsLocation reports whether a slash-separated relative path is one of Locations.
func IsLocation(rel string) bool {
	return slices.ContainsFunc(Locations, func(loc string) bool { return at(rel, loc) })
}

// at ignores case: Windows and macOS open .MCP.json as .mcp.json, so a renamed file is still live there.
func at(rel, loc string) bool {
	return strings.EqualFold(rel, loc) || (len(rel) > len(loc) && strings.EqualFold(rel[len(rel)-len(loc)-1:], "/"+loc))
}

// Discover reads agent config files (slash-separated relative path to content) and reports what the
// registry doesn't know: a Claude Code setup with no ascc hook that can enforce, a hook that names an
// unregistered agent, and every MCP server the registry lists neither a tool nor an identity for. Each is an
// unmonitored identity with an ascc/unregistered finding. A variable or header name in an MCP
// server's config that looks like a credential becomes a candidate identity.
//
// Only names are read: server names, variable names, agent ids and the paths given. No command,
// argument, URL or variable value reaches the output, because any of them can hold a secret.
//
// unreadable lists the files that aren't JSON objects. What they configure is missing from the
// report, and the caller has to say so.
//
// ponytail: it knows only the files in Locations. A server passed with --mcp-config, added by a
// plugin or delivered from the console isn't found; discovery is partial by nature (T-12).
func Discover(files map[string][]byte, reg *pdp.Registry, version string, when time.Time) (inv pdp.Inventory, unreadable []string) {
	inv = pdp.NewInventory(version, when)
	inv.X = map[string]any{"discover": map[string]any{"locationsVersion": LocationsVersion, "locations": Locations}}

	registered := map[string]bool{}
	for _, a := range reg.Agents {
		registered[a.ID] = true
	}
	// Identities the registry lists under the id discover would give them. A retired or disabled
	// entry doesn't count: the config is still there, so the server is still live.
	declared := map[string]bool{}
	for _, n := range reg.Identities {
		live := n.Status == "active" || n.Status == "unknown"
		declared[n.ID] = live && (n.Kind == "mcp-server" || !strings.HasPrefix(n.ID, "local:mcp-server/"))
	}
	setups := map[string]*setup{}   // folder -> the Claude Code setup found there
	agents := map[string][]string{} // unregistered agent id named by a hook -> the files naming it
	servers := map[string]*server{}

	for _, p := range slices.Sorted(maps.Keys(files)) {
		var doc map[string]any
		if json.Unmarshal(files[p], &doc) != nil || doc == nil {
			unreadable = append(unreadable, p)
			continue
		}
		var s *setup // the setup this file belongs to, when it is a project file
		project := func(dir string) {
			if s = setups[dir]; s == nil {
				s = &setup{}
				setups[dir] = s
			}
			s.sources = append(s.sources, p)
		}
		addServers := func(v any) {
			m, _ := v.(map[string]any)
			for name, conf := range m {
				sv := servers[name]
				if sv == nil {
					sv = &server{credentials: map[string]bool{}}
					servers[name] = sv
				}
				sv.sources = append(sv.sources, p)
				c, _ := conf.(map[string]any)
				for _, block := range []string{"env", "headers"} {
					vars, _ := c[block].(map[string]any)
					for key := range vars { // the key only: the value is never touched
						if credentialName(key) {
							sv.credentials[key] = true
						}
					}
				}
			}
		}
		switch {
		case at(p, ".claude/settings.json"), at(p, ".claude/settings.local.json"):
			project(path.Dir(path.Dir(p)))
		case at(p, ".mcp.json"):
			project(path.Dir(p))
			addServers(doc["mcpServers"])
		case at(p, "managed-mcp.json"):
			addServers(doc["mcpServers"])
		case at(p, ".claude.json"):
			addServers(doc["mcpServers"])
			projects, _ := doc["projects"].(map[string]any)
			for _, proj := range projects { // keyed by absolute path, which stays out of the report
				m, _ := proj.(map[string]any)
				addServers(m["mcpServers"])
			}
		}
		if v, set := doc["disableAllHooks"]; set && v != false && s != nil {
			s.disabled = true
		}
		asccHooks(doc, func(agent string, group, handler map[string]any, command string, exec bool) {
			switch {
			case registered[agent]:
			case pdp.ValidName(agent):
				agents[agent] = append(agents[agent], p)
			default:
				return // a hook that speaks for nobody governs nothing
			}
			// A hook that can't block (a narrow matcher, no onFailure, async) doesn't make the setup
			// governed, or one dead line would take a setup off the report.
			if s != nil && len(defects(agent, group, handler, command, exec, nil)) == 0 {
				s.hooked = true
			}
		})
	}

	add := func(id pdp.Identity, title, evidence, remediation string) {
		id.Platform, id.Status = "local", "unknown"
		inv.Identities = append(inv.Identities, id)
		if title != "" {
			inv.Findings = append(inv.Findings, pdp.Finding{ID: "ascc/unregistered:" + id.ID, IdentityID: id.ID,
				RuleID: "ascc/unregistered", Severity: "medium", Title: title, Evidence: evidence, Remediation: remediation, Status: "open"})
		}
	}
	tags := func(sources []string, more ...string) []string {
		out := append([]string{"coverage:" + pdp.CoverageUnmonitored}, more...)
		for _, src := range slices.Compact(slices.Sorted(slices.Values(sources))) {
			out = append(out, "source:"+src)
		}
		return out
	}
	found := func(sources []string) string {
		return "Found in " + strings.Join(slices.Compact(slices.Sorted(slices.Values(sources))), ", ") + "."
	}

	for _, id := range slices.Sorted(maps.Keys(agents)) {
		add(pdp.Identity{ID: "local:agent/" + id, Kind: "ai-agent", DisplayName: id, Tags: tags(agents[id], "runtime:claude-code")},
			fmt.Sprintf("Agent %s has a hook but isn't in the registry", id),
			"An ascc hook is registered for this agent id, which the registry given to discover doesn't list. "+found(agents[id]),
			"Add the agent to the registry with an owner, or remove the hook registration.")
	}
	for _, dir := range slices.Sorted(maps.Keys(setups)) {
		if s := setups[dir]; !s.hooked || s.disabled {
			where := dir
			if dir == "." {
				where = "the root folder"
			}
			add(pdp.Identity{ID: "local:claude-code-setup/" + dir, Kind: "ai-agent", DisplayName: "Claude Code setup in " + where,
				Tags: tags(s.sources, "runtime:claude-code")},
				fmt.Sprintf("Claude Code is set up in %s with no ascc hook that can enforce", where),
				"This folder's Claude Code files register no ascc hook that runs on every tool call and fails closed, or they set disableAllHooks. "+found(s.sources)+
					" A hook in user or managed settings would still cover it: `ascc posture claude-code` checks those.",
				"Register the agent that works here and its hook, or confirm with `ascc posture claude-code` that a managed hook covers it.")
		}
	}
	for _, name := range slices.Sorted(maps.Keys(servers)) {
		sv := servers[name]
		id := "local:mcp-server/" + name
		// Claude Code names an MCP tool mcp__<server>__<tool>, with other characters in the server name turned to "_".
		// ponytail: a server is recognised by name alone. Another server reusing a registered name, or
		// server "a" beside a registered "a__b", counts as listed. The registry has no more to match on.
		prefix := "claude-code/mcp__" + toolNameRE.ReplaceAllString(name, "_") + "__"
		listed := slices.ContainsFunc(slices.Collect(maps.Keys(reg.Tools)), func(tool string) bool { return strings.HasPrefix(tool, prefix) })
		if !listed && !declared[id] {
			add(pdp.Identity{ID: id, Kind: "mcp-server", DisplayName: name, Tags: tags(sv.sources)},
				fmt.Sprintf("MCP server %s isn't in the registry", name),
				"The registry lists no tool of this server and no identity "+id+". "+found(sv.sources),
				"List the tools an agent may use as "+prefix+"<tool> in the registry, declare the server under identities, or remove it from the config.")
		}
		for _, key := range slices.Sorted(maps.Keys(sv.credentials)) {
			if declared["local:credential/"+name+"/"+key] {
				continue
			}
			add(pdp.Identity{ID: "local:credential/" + name + "/" + key, Kind: "other",
				DisplayName: key + " (used by MCP server " + name + ")", Tags: tags(sv.sources, "candidate", "used-by:"+id)}, "", "", "")
		}
	}
	return inv, unreadable
}

type setup struct {
	sources  []string
	hooked   bool // some ascc hook here speaks for an agent and has no defect
	disabled bool // one of its files sets disableAllHooks
}

type server struct {
	sources     []string
	credentials map[string]bool // names only
}

var (
	toolNameRE = regexp.MustCompile(`[^A-Za-z0-9_-]`)
	wordRE     = regexp.MustCompile(`[A-Z]+[a-z0-9]*|[a-z0-9]+`)
)

// credentialName reports whether a variable or header name looks like it holds a credential.
// ponytail: a word list, so MONKEY_KEY is a hit and an oddly named secret is a miss. It only
// nominates candidates for a person to confirm.
func credentialName(name string) bool {
	for _, word := range wordRE.FindAllString(name, -1) {
		w := strings.ToUpper(word)
		if slices.Contains([]string{"KEY", "APIKEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "PWD", "PAT", "AUTH",
			"AUTHORIZATION", "CREDENTIAL", "CREDENTIALS", "COOKIE", "BEARER"}, w) {
			return true
		}
	}
	return false
}
