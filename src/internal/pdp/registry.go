package pdp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Agent statuses. Only an active agent reaches policy; the other two are denied everything.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusRetired   = "retired"
)

// Agent is one entry in ascc.registry.json.
type Agent struct {
	ID       string   `json:"id"`
	Owner    string   `json:"owner"` // empty is allowed and reported by inventory as a finding
	Runtime  string   `json:"runtime"`
	RiskTier *int64   `json:"risk_tier"`
	Status   string   `json:"status"`
	Groups   []string `json:"groups,omitempty"`
}

// Registry is ascc.registry.json: the agents ascc knows and the tools they may ask for.
// Tools maps a tool id ("claude-code/Bash") to the tool groups it belongs to.
type Registry struct {
	Agents []Agent             `json:"agents"`
	Tools  map[string][]string `json:"tools"`
}

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidName reports whether s could be an agent or group id in a registry.
func ValidName(s string) bool { return nameRE.MatchString(s) }

// ParseRegistry reads and checks a registry. Every error is one line naming the entry at fault.
func ParseRegistry(data []byte) (*Registry, error) {
	var r Registry
	if err := strictUnmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("registry: %s", oneLine(err))
	}
	if len(r.Agents) == 0 {
		return nil, errors.New(`registry: "agents" is empty`)
	}
	seen := map[string]bool{}
	for i, a := range r.Agents {
		where := fmt.Sprintf("registry: agent %q", a.ID)
		switch {
		case !nameRE.MatchString(a.ID):
			return nil, fmt.Errorf("registry: agents[%d]: id %q must be 1 to 64 letters, digits, dots, dashes or underscores", i, a.ID)
		case seen[a.ID]:
			return nil, fmt.Errorf("%s is listed twice", where)
		case a.Runtime == "":
			return nil, fmt.Errorf("%s has no runtime", where)
		case a.RiskTier == nil || *a.RiskTier < 0:
			return nil, fmt.Errorf("%s needs a risk_tier of 0 or more", where)
		case !slices.Contains([]string{StatusActive, StatusSuspended, StatusRetired}, a.Status):
			return nil, fmt.Errorf("%s has status %q; use active, suspended or retired", where, a.Status)
		}
		seen[a.ID] = true
		if err := checkGroups(a.Groups); err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
	}
	for id, groups := range r.Tools {
		if id == "" || strings.ContainsAny(id, " \t\r\n") {
			return nil, fmt.Errorf("registry: tool id %q must be non-empty with no spaces", id)
		}
		if err := checkGroups(groups); err != nil {
			return nil, fmt.Errorf("registry: tool %q: %v", id, err)
		}
	}
	return &r, nil
}

func checkGroups(groups []string) error {
	for _, g := range groups {
		if !nameRE.MatchString(g) {
			return fmt.Errorf("group %q must be 1 to 64 letters, digits, dots, dashes or underscores", g)
		}
	}
	return nil
}

// strictUnmarshal refuses unknown fields and trailing data, so a typo like "stauts" is an error
// instead of a silently missing value.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}
