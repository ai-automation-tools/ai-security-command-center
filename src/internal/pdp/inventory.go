package pdp

import (
	"fmt"
	"slices"
	"strings"
	"time"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
)

// The Agent Security Suite inventory document, schema v0.1
// (https://ai-automation-tools.dev/schemas/agent-inventory/v0.1.json, vendored in schemas/).
// Only the fields ascc fills are declared.
type Inventory struct {
	SchemaVersion string     `json:"schemaVersion"`
	GeneratedAt   string     `json:"generatedAt"`
	Producer      Producer   `json:"producer"`
	Identities    []Identity `json:"identities"`
	Findings      []Finding  `json:"findings"`
}

type Producer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

type Identity struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Platform    string   `json:"platform"`
	DisplayName string   `json:"displayName"`
	Owner       *Owner   `json:"owner"` // null when the registry names none
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
}

type Owner struct {
	Name string `json:"name"`
}

type Finding struct {
	ID          string `json:"id"`
	IdentityID  string `json:"identityId"`
	RuleID      string `json:"ruleId"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
	Status      string `json:"status"`
}

// Coverage labels. The label is computed from evidence, never declared in the registry.
const (
	CoverageEnforced    = "enforced"
	CoverageObserved    = "observed"
	CoverageUnmonitored = "unmonitored"
)

// Evidence is what ascc has seen about each agent, keyed by agent id. The caller gathers it, since
// this package reads no files. The zero value is no evidence at all.
type Evidence struct {
	Audited     map[string]bool // the hook has written a decision for the agent to a log that verifies
	PosturePass map[string]bool // the posture check passes for the agent's hook registration
}

// coverage is the evidence-based label for an agent. A passing posture check alone isn't enough: a
// hook that has never recorded a call for the agent hasn't shown it runs.
func coverage(a Agent, ev Evidence) string {
	switch {
	case !ev.Audited[a.ID]:
		return CoverageUnmonitored
	case !ev.PosturePass[a.ID]:
		return CoverageObserved
	}
	return CoverageEnforced
}

// Inventory writes the registry as an inventory document. Agents are sorted by id, so the same
// registry, policies, version, time and evidence always give the same bytes.
func (p *PDP) Inventory(version string, generatedAt time.Time, ev Evidence) Inventory {
	inv := Inventory{
		SchemaVersion: "0.1",
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339),
		Producer: Producer{Name: "ascc", Version: version,
			URL: "https://github.com/ai-automation-tools/ai-security-command-center"},
		Identities: []Identity{},
		Findings:   []Finding{},
	}
	agents := slices.Clone(p.registry.Agents)
	slices.SortFunc(agents, func(a, b Agent) int { return strings.Compare(a.ID, b.ID) })

	for _, a := range agents {
		id := "local:agent/" + a.ID
		ident := Identity{
			ID: id, Kind: "ai-agent", Platform: "local", DisplayName: a.ID,
			Status: map[string]string{StatusActive: "active", StatusSuspended: "disabled", StatusRetired: "retired"}[a.Status],
			Tags:   []string{"coverage:" + coverage(a, ev), "runtime:" + a.Runtime, fmt.Sprintf("risk-tier:%d", *a.RiskTier)},
		}
		for _, g := range a.Groups {
			ident.Tags = append(ident.Tags, "group:"+g)
		}
		add := func(rule, severity, title, evidence, remediation string) {
			inv.Findings = append(inv.Findings, Finding{
				ID: "ascc/" + rule + ":" + a.ID, IdentityID: id, RuleID: "ascc/" + rule, Severity: severity,
				Title: title, Evidence: evidence, Remediation: remediation, Status: "open",
			})
		}
		if strings.TrimSpace(a.Owner) == "" {
			add("no-owner", "high", fmt.Sprintf("Agent %s has no owner", a.ID),
				"The registry entry has an empty owner.",
				"Name the accountable person or team in the agent's owner field, or retire the agent.")
		} else {
			ident.Owner = &Owner{Name: a.Owner}
		}
		if !p.hasPermit(a) {
			add("no-policy", "medium", fmt.Sprintf("No policy covers agent %s", a.ID),
				"No permit policy has a principal scope that matches this agent, so every request it makes is denied by default.",
				"Add the agent to a group a permit policy names, write a permit for it, or retire it.")
		}
		if a.Status == StatusSuspended {
			add("suspended", "info", fmt.Sprintf("Agent %s is suspended", a.ID),
				"The registry status is suspended: every request is denied.",
				"Revive the agent once the reason for suspending it is resolved, or retire it.")
		}
		inv.Identities = append(inv.Identities, ident)
	}
	return inv
}

// hasPermit reports whether any permit policy's principal scope matches the agent.
// ponytail: it reads the scope only, not `when` conditions, so a permit that can never fire still
// counts. Evaluate sample requests instead if that ever hides a real gap.
func (p *PDP) hasPermit(a Agent) bool {
	uid := types.NewEntityUID("Agent", types.String(a.ID))
	in := func(e types.EntityUID) bool {
		return e == uid || (e.Type == "AgentGroup" && slices.Contains(a.Groups, string(e.ID)))
	}
	for _, pol := range p.policies.All() {
		if pol.Effect() != cedar.Permit {
			continue
		}
		switch s := (*xast.Policy)(pol.AST()).Principal.(type) {
		case xast.ScopeTypeAll:
			return true
		case xast.ScopeTypeEq:
			if s.Entity == uid {
				return true
			}
		case xast.ScopeTypeIn:
			if in(s.Entity) {
				return true
			}
		case xast.ScopeTypeIs:
			if s.Type == "Agent" {
				return true
			}
		case xast.ScopeTypeIsIn:
			if s.Type == "Agent" && in(s.Entity) {
				return true
			}
		}
	}
	return false
}
