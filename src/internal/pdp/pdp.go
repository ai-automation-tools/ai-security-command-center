// Package pdp is the ascc policy decision point: an agent registry and Cedar policies in, an allow or a
// deny with the determining policy ids and a plain-English reason out.
//
// It does no file, network or clock access. Callers hand it bytes, so the WebAssembly demo imports
// exactly the code the CLI runs.
package pdp

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"
)

// ActionToolUse is the only action the MVP decides on.
const ActionToolUse = "runtime.tool_use"

//go:embed schema.cedar
var schemaSrc []byte

// Request is what `ascc decide` reads: bare ids, not Cedar entity syntax.
type Request struct {
	Principal string         `json:"principal"` // agent id, e.g. "dev-agent"
	Action    string         `json:"action"`    // "runtime.tool_use"
	Resource  string         `json:"resource"`  // tool id, e.g. "claude-code/Bash"
	Context   map[string]any `json:"context"`
}

// Decision is the answer. PolicyIDs holds the Cedar policies that determined it: the forbids on a
// policy deny, the permits on an allow, the erroring policies on an evaluation error, and nothing when
// ascc denied before or without a policy (unknown agent, suspended agent, default deny).
type Decision struct {
	Decision  string   `json:"decision"` // "allow" or "deny"
	PolicyIDs []string `json:"policy_ids"`
	Reason    string   `json:"reason"`
}

// Deny builds a deny that no policy determined. Callers use it for their own fail-closed paths.
func Deny(reason string) Decision {
	return Decision{Decision: "deny", PolicyIDs: []string{}, Reason: reason}
}

// ParseRequest reads a request document. Unknown fields are an error.
func ParseRequest(data []byte) (Request, error) {
	var r Request
	if err := strictUnmarshal(data, &r); err != nil {
		return Request{}, fmt.Errorf("request: %s", oneLine(err))
	}
	return r, nil
}

// PDP holds a validated registry and policy set.
type PDP struct {
	registry  *Registry
	agents    map[string]Agent
	policies  *cedar.PolicySet
	reasons   map[string]string // policy id -> its @reason annotation, if any
	entities  types.EntityMap
	validator *validate.Validator
}

// New loads a registry and policy files (name -> source). It refuses anything that could make a
// decision wrong later: an invalid registry, a policy with no @id or a duplicate one, a policy that
// doesn't type-check against the schema, or no policy at all.
func New(registryJSON []byte, policyFiles map[string][]byte) (*PDP, error) {
	v, err := newValidator()
	if err != nil {
		return nil, err
	}
	reg, err := ParseRegistry(registryJSON)
	if err != nil {
		return nil, err
	}
	p := &PDP{
		registry:  reg,
		agents:    map[string]Agent{},
		policies:  cedar.NewPolicySet(),
		reasons:   map[string]string{},
		entities:  types.EntityMap{},
		validator: v,
	}

	for _, a := range reg.Agents {
		p.agents[a.ID] = a
		p.addEntity("Agent", a.ID, "AgentGroup", a.Groups, types.NewRecord(types.RecordMap{
			"owner":     types.String(a.Owner),
			"runtime":   types.String(a.Runtime),
			"risk_tier": types.Long(*a.RiskTier),
			"status":    types.String(a.Status),
		}))
	}
	for id, groups := range reg.Tools {
		p.addEntity("Tool", id, "ToolGroup", groups, types.Record{})
	}
	for uid, e := range p.entities {
		if err := v.Entity(e); err != nil {
			return nil, fmt.Errorf("registry: %s doesn't fit the schema: %s", uid, oneLine(err))
		}
	}

	names := make([]string, 0, len(policyFiles))
	for name := range policyFiles {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		list, err := cedar.NewPolicyListFromBytes(name, policyFiles[name])
		if err != nil {
			return nil, fmt.Errorf("policies: %s: %s", name, oneLine(err))
		}
		// A file emptied by a half-finished save would otherwise drop its forbids in silence.
		if len(list) == 0 {
			return nil, fmt.Errorf("policies: %s: no policy in this file; remove the file if that is meant", name)
		}
		for _, pol := range list {
			id := string(pol.Annotations()["id"])
			if id == "" {
				return nil, fmt.Errorf(`policies: %s: the policy at line %d has no @id("...")`, name, pol.Position().Line)
			}
			if p.policies.Get(cedar.PolicyID(id)) != nil {
				return nil, fmt.Errorf("policies: %s: @id %q is used twice", name, id)
			}
			if err := checkPolicy(v, id, pol); err != nil {
				return nil, fmt.Errorf("policies: %s: policy %q: %s", name, id, oneLine(err))
			}
			p.policies.Add(cedar.PolicyID(id), pol)
			p.reasons[id] = string(pol.Annotations()["reason"])
		}
	}
	if len(p.reasons) == 0 {
		return nil, fmt.Errorf("policies: none loaded; with no policy every request is denied")
	}
	return p, nil
}

// newValidator and checkPolicy are the only callers of cedar-go's experimental schema validator.
func newValidator() (*validate.Validator, error) {
	var s schema.Schema
	if err := s.UnmarshalCedar(schemaSrc); err != nil {
		return nil, fmt.Errorf("built-in schema: %s", oneLine(err))
	}
	rs, err := s.Resolve()
	if err != nil {
		return nil, fmt.Errorf("built-in schema: %s", oneLine(err))
	}
	return validate.New(rs), nil
}

func checkPolicy(v *validate.Validator, id string, pol *cedar.Policy) error {
	return v.Policy(id, (*xast.Policy)(pol.AST()))
}

func (p *PDP) addEntity(typ, id, groupType string, groups []string, attrs types.Record) {
	parents := make([]types.EntityUID, 0, len(groups))
	for _, g := range groups {
		guid := types.NewEntityUID(types.EntityType(groupType), types.String(g))
		parents = append(parents, guid)
		p.entities[guid] = types.Entity{UID: guid}
	}
	uid := types.NewEntityUID(types.EntityType(typ), types.String(id))
	p.entities[uid] = types.Entity{UID: uid, Parents: types.NewEntityUIDSet(parents...), Attributes: attrs}
}

// Decide answers one request. It never panics and never returns an error: every failure is a deny
// with the reason.
func (p *PDP) Decide(req Request) (d Decision) {
	defer func() {
		if r := recover(); r != nil {
			d = Deny("internal error while deciding")
		}
	}()

	// Cedar skips a policy that errors, so a forbid reading a missing entity would fail open.
	// Anything not in the registry is denied before the engine sees it.
	agent, ok := p.agents[req.Principal]
	if !ok {
		return Deny(fmt.Sprintf("agent %q is not in the registry", req.Principal))
	}
	// The kill switch doesn't depend on a policy someone could delete.
	if agent.Status != StatusActive {
		return Deny(fmt.Sprintf("agent %q is %s: every request is denied", agent.ID, agent.Status))
	}
	if req.Action != ActionToolUse {
		return Deny(fmt.Sprintf("unknown action %q; the only action is %q", req.Action, ActionToolUse))
	}
	if _, ok := p.registry.Tools[req.Resource]; !ok {
		return Deny(fmt.Sprintf("tool %q is not in the registry", req.Resource))
	}
	ctx, err := contextRecord(req.Context)
	if err != nil {
		return Deny(err.Error())
	}
	creq := cedar.Request{
		Principal: types.NewEntityUID("Agent", types.String(req.Principal)),
		Action:    types.NewEntityUID("Action", ActionToolUse),
		Resource:  types.NewEntityUID("Tool", types.String(req.Resource)),
		Context:   ctx,
	}
	if err := p.validator.Request(creq); err != nil {
		// The validator's own message prints the whole context, values included. A context can carry
		// anything a caller put there, and the reason goes to the audit log and back to the agent.
		return Deny("request doesn't fit the schema: the context is missing an attribute, has one the schema lacks, or has one of the wrong type")
	}

	decision, diag := cedar.Authorize(p.policies, p.entities, creq)

	if len(diag.Errors) > 0 {
		var ids, parts []string
		for _, e := range diag.Errors {
			ids = append(ids, string(e.PolicyID))
			parts = append(parts, fmt.Sprintf("policy %q failed to evaluate (%s)", e.PolicyID, e.Message))
		}
		slices.Sort(ids)
		slices.Sort(parts)
		return Decision{Decision: "deny", PolicyIDs: ids,
			Reason: strings.Join(parts, "; ") + "; a policy that errors is treated as a deny"}
	}

	ids := make([]string, 0, len(diag.Reasons))
	for _, r := range diag.Reasons {
		ids = append(ids, string(r.PolicyID))
	}
	slices.Sort(ids)
	switch {
	case decision == cedar.Allow && len(ids) > 0:
		return Decision{Decision: "allow", PolicyIDs: ids, Reason: p.explain("permitted by", ids)}
	case len(ids) > 0:
		return Decision{Decision: "deny", PolicyIDs: ids, Reason: p.explain("forbidden by", ids)}
	default:
		return Deny(fmt.Sprintf("no policy permits agent %q to use %q (default deny)", req.Principal, req.Resource))
	}
}

func (p *PDP) explain(verb string, ids []string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id
		if r := p.reasons[id]; r != "" {
			parts[i] = id + ": " + r
		}
	}
	return verb + " " + strings.Join(parts, "; ")
}

// contextRecord turns the request's context into a Cedar record. The schema only has booleans and
// strings; anything else is refused rather than guessed at.
func contextRecord(ctx map[string]any) (types.Record, error) {
	m := types.RecordMap{}
	for k, v := range ctx {
		switch x := v.(type) {
		case bool:
			m[types.String(k)] = types.Boolean(x)
		case string:
			m[types.String(k)] = types.String(x)
		default:
			return types.Record{}, fmt.Errorf("every context attribute must be true, false or a string")
		}
	}
	return types.NewRecord(m), nil
}
