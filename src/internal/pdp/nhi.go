package pdp

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Non-human identities: the service accounts, bots, CI jobs, OAuth apps and MCP servers the agents
// use. They are declared in the registry's "identities" list or imported from another suite tool's
// inventory document, and both are written exactly as the suite schema writes an identity, so one
// check serves both. ascc holds no credential for any of them: a fingerprint, never a secret.

// The suite schema's enums (schemas/agent-inventory.schema.json).
var (
	identityKinds     = []string{"ai-agent", "service-account", "bot", "ci-job", "mcp-server", "oauth-app", "workload", "other"}
	identityPlatforms = []string{"github", "aws", "azure-entra", "okta", "gcp", "slack", "scim", "mcp", "identityiq", "local", "other"}
	identityStatuses  = []string{"active", "disabled", "retired", "unknown"}
	credentialTypes   = []string{"api-key", "oauth-token", "personal-access-token", "access-key", "certificate", "ssh-key", "password", "session", "other"}
	credentialPlaces  = []string{"env-file", "os-keychain", "vault", "config-file", "ci-secret", "platform", "unknown"}
	grantAccess       = []string{"read", "write", "admin", "execute", "other"}
	toolCapabilities  = []string{"read-secrets", "network-egress", "write-code", "execute-shell", "read-private-data", "send-message", "modify-iam", "spend-money", "deploy", "other"}

	// No control or text-direction characters: an id is printed to a terminal by `ascc kill`.
	identityIDRE  = regexp.MustCompile(`^[a-z0-9-]+:[^\x00-\x1f\x7f-\x{9f}\x{202a}-\x{202e}\x{2066}-\x{2069}]+$`)
	fingerprintRE = regexp.MustCompile(`^[A-Za-z0-9:_-]{4,64}$`)
	emailRE       = regexp.MustCompile(`^[A-Za-z0-9_%+-]+(\.[A-Za-z0-9_%+-]+)*@[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)
)

const agentIDPrefix = "local:agent/"

// secretLike reports whether s starts the way a well-known token does. A fingerprint or a credential
// id is printed in the document, so a token pasted into either would be a secret at rest.
// ponytail: a prefix list. A secret with no known prefix that fits the fingerprint pattern still
// gets through; the guide's limits table says so.
func secretLike(s string) bool {
	l := strings.ToLower(s)
	return slices.ContainsFunc([]string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "glpat-", "sk-", "sk_", "xox", "eyj", "bearer"},
		func(p string) bool { return strings.HasPrefix(l, p) })
}

// checkIdentity holds one identity to the suite schema's rules, so a document ascc writes from it
// still validates, and puts its times in one form. No error repeats a value from the entry: an
// imported file is someone else's data.
func checkIdentity(n *Identity) error {
	enum := func(field, v string, allowed []string, optional bool) error {
		if (v == "" && optional) || slices.Contains(allowed, v) {
			return nil
		}
		return fmt.Errorf("%s must be one of: %s", field, strings.Join(allowed, ", "))
	}
	stamp := func(field string, s *string) error {
		if *s == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, *s)
		if t = t.UTC(); err != nil || t.Year() < 1 || t.Year() > 9999 {
			return fmt.Errorf("%s must be a time like 2026-01-31T00:00:00Z", field)
		}
		*s = t.Format(time.RFC3339Nano)
		return nil
	}
	if !identityIDRE.MatchString(n.ID) || len(n.ID) > 512 {
		return errors.New(`id must look like "<platform>:<native id>", such as "github:app/release-bot"`)
	}
	err := errors.Join(
		enum("kind", n.Kind, identityKinds, false),
		enum("platform", n.Platform, identityPlatforms, false),
		enum("status", n.Status, identityStatuses, false),
		stamp("createdAt", &n.CreatedAt), stamp("lastUsedAt", &n.LastUsedAt), stamp("retiredAt", &n.RetiredAt),
	)
	if o := n.Owner; o != nil {
		local, domain, _ := strings.Cut(o.Email, "@")
		longLabel := slices.ContainsFunc(strings.Split(domain, "."), func(l string) bool { return len(l) > 63 })
		if o.Email != "" && (len(o.Email) > 254 || len(local) > 64 || longLabel || !emailRE.MatchString(o.Email)) {
			err = errors.Join(err, errors.New("owner.email isn't an email address"))
		}
		err = errors.Join(err, stamp("owner.verifiedAt", &o.VerifiedAt))
	}
	seen := map[string]bool{}
	for i := range n.Credentials {
		c := &n.Credentials[i]
		where := fmt.Sprintf("credentials[%d].", i)
		switch {
		case c.ID == "" || seen[c.ID]:
			err = errors.Join(err, errors.New(where+"id must be set and used once"))
		case secretLike(c.ID):
			err = errors.Join(err, errors.New(where+"id looks like a token; give the credential a name, never the secret"))
		case c.Fingerprint != "" && (!fingerprintRE.MatchString(c.Fingerprint) || secretLike(c.Fingerprint)):
			// The value isn't repeated: someone may have pasted the secret here.
			err = errors.Join(err, errors.New(where+"fingerprint must be 4 to 64 letters, digits, colons, dashes or underscores: a key id or the first 12 hex characters of the secret's SHA-256, never the secret"))
		}
		seen[c.ID] = true
		err = errors.Join(err, enum(where+"type", c.Type, credentialTypes, false), enum(where+"location", c.Location, credentialPlaces, true),
			stamp(where+"createdAt", &c.CreatedAt), stamp(where+"expiresAt", &c.ExpiresAt), stamp(where+"rotatedAt", &c.RotatedAt),
			stamp(where+"rotateBy", &c.RotateBy), stamp(where+"lastUsedAt", &c.LastUsedAt), stamp(where+"revokedAt", &c.RevokedAt),
			stamp(where+"lastVerifiedDeadAt", &c.LastVerifiedDeadAt))
	}
	for i := range n.Grants {
		g := &n.Grants[i]
		where := fmt.Sprintf("grants[%d].", i)
		if g.ID == "" || g.Resource == "" || g.Permission == "" {
			err = errors.Join(err, errors.New(where+"id, resource and permission must all be set"))
		}
		if g.ObservationDays != nil && *g.ObservationDays < 0 {
			err = errors.Join(err, errors.New(where+"observationDays can't be negative"))
		}
		err = errors.Join(err, enum(where+"access", g.Access, grantAccess, true), stamp(where+"grantedAt", &g.GrantedAt), stamp(where+"lastUsedAt", &g.LastUsedAt))
	}
	for i, t := range n.Tools {
		where := fmt.Sprintf("tools[%d].", i)
		if t.Name == "" {
			err = errors.Join(err, errors.New(where+"name must be set"))
		}
		for _, c := range t.Capabilities {
			err = errors.Join(err, enum(where+"capabilities", c, toolCapabilities, false))
		}
	}
	if err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), "\n", "; "))
	}
	return nil
}

// ParseInventory reads another suite tool's inventory document for `inventory --import`. Fields the
// schema doesn't have are dropped; an identity that breaks the schema's rules is an error, since
// ascc would otherwise write a document that doesn't validate. An id repeated inside the document
// keeps its first entry.
func ParseInventory(data []byte) (Inventory, error) {
	var doc Inventory
	if json.Unmarshal(data, &doc) != nil {
		return Inventory{}, errors.New("isn't a suite inventory document")
	}
	if doc.SchemaVersion != "0.1" {
		return Inventory{}, errors.New(`schemaVersion isn't "0.1", the only suite schema version ascc reads`)
	}
	if doc.Identities == nil {
		return Inventory{}, errors.New(`has no "identities" list`)
	}
	seen := map[string]bool{}
	kept := doc.Identities[:0]
	for i := range doc.Identities {
		n := doc.Identities[i]
		if err := checkIdentity(&n); err != nil {
			return Inventory{}, fmt.Errorf("identities[%d]: %v", i, err)
		}
		if strings.HasPrefix(n.ID, agentIDPrefix) && n.Kind != "ai-agent" {
			return Inventory{}, fmt.Errorf("identities[%d]: an id under %q is an agent's, so its kind must be ai-agent", i, agentIDPrefix)
		}
		if !seen[n.ID] {
			seen[n.ID] = true
			kept = append(kept, n)
		}
	}
	doc.Identities = kept
	return doc, nil
}

// addNHIs writes the declared identities, then the imported ones the registry doesn't already
// have, with their findings. A declared identity wins over an imported one with the same id, and
// so does a registered agent.
func (p *PDP) addNHIs(inv *Inventory, now time.Time, imports []Inventory) {
	usedBy := map[string][]Agent{}
	have := map[string]bool{}
	for _, a := range p.registry.Agents {
		have[agentIDPrefix+a.ID] = true
		for _, id := range a.Uses {
			usedBy[id] = append(usedBy[id], a)
		}
	}
	var all []Identity
	take := func(n Identity, more ...string) {
		if have[n.ID] {
			return
		}
		have[n.ID] = true
		// Coverage is computed, never taken from a file: an import can't call anything enforced. It
		// can say unmonitored, which only lowers, and that is how a discover report says it.
		unmonitored := n.Kind == "ai-agent" // an agent ascc has no hook in front of
		tags := slices.DeleteFunc(slices.Clone(n.Tags), func(t string) bool {
			l := strings.ToLower(strings.TrimSpace(t))
			unmonitored = unmonitored || l == "coverage:"+CoverageUnmonitored
			return strings.HasPrefix(l, "coverage:") || strings.HasPrefix(l, "imported-from:") || strings.HasPrefix(l, "used-by:"+agentIDPrefix)
		})
		if unmonitored {
			tags = append([]string{"coverage:" + CoverageUnmonitored}, tags...)
		}
		n.Tags = append(tags, more...)
		if n.DisplayName == "" {
			n.DisplayName = n.ID
		}
		n.Credentials = slices.Clone(n.Credentials)
		all = append(all, n)
	}
	for _, n := range p.registry.Identities {
		take(n)
	}
	for _, doc := range imports {
		from := "unknown"
		if ValidName(doc.Producer.Name) {
			from = doc.Producer.Name
		}
		for _, n := range doc.Identities {
			take(n, "imported-from:"+from)
		}
	}
	slices.SortFunc(all, func(a, b Identity) int { return strings.Compare(a.ID, b.ID) })

	for _, n := range all {
		add := func(rule, key, severity, title, evidence, remediation string) {
			inv.Findings = append(inv.Findings, Finding{
				ID: "ascc/" + rule + ":" + n.ID + key, IdentityID: n.ID, RuleID: "ascc/" + rule, Severity: severity,
				Title: title, Evidence: evidence, Remediation: remediation, Status: "open",
			})
		}
		if o := n.Owner; o == nil || strings.TrimSpace(o.Name+o.Email+o.Team) == "" {
			n.Owner = nil
			add("nhi-no-owner", "", "high", fmt.Sprintf("Identity %s has no owner", n.DisplayName),
				"No owner name, email or team is recorded for this identity.",
				"Name the accountable person or team, or retire the identity on the platform that holds it.")
		}
		for i := range n.Credentials {
			c := &n.Credentials[i]
			due, errDue := time.Parse(time.RFC3339, c.ExpiresAt)
			what := "expired"
			if rotate, err := time.Parse(time.RFC3339, c.RotateBy); err == nil && (errDue != nil || rotate.Before(due)) {
				due, errDue, what = rotate, nil, "was due for rotation"
			}
			c.RotateBy = "" // ascc's own field: the suite schema has no place for it
			if errDue == nil && due.Before(now) && c.RevokedAt == "" && n.Status != "retired" {
				add("nhi-credential-overdue", "#"+c.ID, "medium", fmt.Sprintf("A credential of %s is overdue", n.DisplayName),
					fmt.Sprintf("Credential %s %s on %s and isn't recorded as revoked.", c.ID, what, due.Format(time.DateOnly)),
					"Rotate or revoke it on the platform that holds it, then update its dates. ascc holds no credential and can't do it.")
			}
		}
		var users, stopped []string
		for _, a := range usedBy[n.ID] {
			users = append(users, a.ID)
			n.Tags = append(n.Tags, "used-by:"+agentIDPrefix+a.ID)
			if a.Status != StatusActive {
				stopped = append(stopped, a.ID+" ("+a.Status+")")
			}
		}
		if len(users) == 0 && n.Kind != "ai-agent" && n.Status != "retired" {
			add("nhi-unlinked", "", "low", fmt.Sprintf("No agent uses identity %s", n.DisplayName),
				`No agent in the registry lists this identity under "uses".`,
				`Add it to the "uses" list of each agent that holds it, or retire it if nothing does.`)
		}
		if len(stopped) > 0 && (n.Status == "active" || n.Status == "unknown") {
			add("nhi-used-by-suspended-agent", "", "medium", fmt.Sprintf("Identity %s is still live after its agent was stopped", n.DisplayName),
				"Used by "+strings.Join(stopped, ", ")+". Stopping an agent in ascc doesn't touch the identities it holds.",
				"Rotate or revoke its credentials on the platform that holds it. ascc holds no credential and can't do it.")
		}
		inv.Identities = append(inv.Identities, n)
	}
}
