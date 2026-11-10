package pdp

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var sampleDay = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// validates fails the test unless the document fits the suite schema, formats included.
func validates(t *testing.T, inv Inventory) string {
	t.Helper()
	raw, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	sch, err := c.Compile(filepath.Join("..", "..", "..", "schemas", "agent-inventory.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(doc); err != nil {
		t.Fatalf("the document doesn't validate against the suite schema: %v", err)
	}
	return string(raw)
}

func findings(inv Inventory) (out []string) {
	for _, f := range inv.Findings {
		out = append(out, f.RuleID+" "+f.IdentityID)
	}
	return out
}

func identity(t *testing.T, inv Inventory, id string) Identity {
	t.Helper()
	i := slices.IndexFunc(inv.Identities, func(n Identity) bool { return n.ID == id })
	if i < 0 {
		t.Fatalf("no identity %s", id)
	}
	return inv.Identities[i]
}

// Spec criterion 16, first half: the sample registry's planted ownerless service account, overdue
// credential and unlinked OAuth app come out as findings, and the document still validates.
func TestInventoryNHIs(t *testing.T) {
	inv := samplePDP(t).Inventory("0.0.0", sampleDay, Evidence{})
	raw := validates(t, inv)

	want := []string{
		"ascc/suspended local:agent/old-agent",
		"ascc/no-owner local:agent/ownerless-agent",
		"ascc/nhi-no-owner aws:arn:aws:iam::123456789012:role/ci-deploy",
		"ascc/nhi-credential-overdue github:app/release-bot",
		"ascc/nhi-used-by-suspended-agent local:service-account/nightly-backup",
		"ascc/nhi-unlinked okta:app/reporting-dashboard",
	}
	if got := findings(inv); !slices.Equal(got, want) {
		t.Fatalf("findings\n got %v\nwant %v", got, want)
	}
	// rotateBy is the registry's own field: the suite schema has no place for it.
	if strings.Contains(raw, "rotateBy") {
		t.Error("rotateBy reached the document")
	}
	bot := identity(t, inv, "github:app/release-bot")
	if !slices.Equal(bot.Tags, []string{"used-by:local:agent/dev-agent", "used-by:local:agent/ci-agent"}) || bot.Credentials[0].Fingerprint != "3f9a1c0b7e2d" {
		t.Errorf("release-bot: %+v", bot)
	}
	if ci := identity(t, inv, "local:agent/ci-agent"); !slices.Contains(ci.Tags, "uses:github:app/release-bot") {
		t.Errorf("ci-agent tags %v", ci.Tags)
	}
	// The same day a year earlier, the credential isn't due yet.
	if got := findings(samplePDP(t).Inventory("0.0.0", sampleDay.AddDate(-1, 0, 0), Evidence{})); slices.Contains(got, want[3]) {
		t.Error("a credential was overdue before its date")
	}
}

func nhiPDP(t *testing.T, identities, uses string) *PDP {
	t.Helper()
	reg := `{"agents":[{"id":"a","owner":"o","runtime":"claude-code","risk_tier":1,"status":"active","uses":[` + uses + `]},
		{"id":"gone","owner":"o","runtime":"claude-code","risk_tier":1,"status":"retired","uses":[` + uses + `]}],
		"identities":[` + identities + `],"tools":{"claude-code/Read":[]}}`
	p, err := New([]byte(reg), map[string][]byte{"p.cedar": []byte(okPolicy)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Each finding has a case that raises it and a neighbour that doesn't.
func TestNHIFindingRules(t *testing.T) {
	const owned = `"kind":"bot","platform":"github","owner":{"name":"o"}`
	tests := []struct{ name, identity, uses, want string }{
		{"an owner with only blanks is no owner", `{"id":"x:1","kind":"bot","platform":"github","owner":{"name":" "},"status":"active"}`, `"x:1"`, "ascc/nhi-no-owner x:1,ascc/nhi-used-by-suspended-agent x:1"},
		{"an email is an owner", `{"id":"x:1","kind":"bot","platform":"github","owner":{"email":"team@example.com"},"status":"disabled"}`, `"x:1"`, ""},
		{"expired", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2026-01-01T00:00:00Z"}]}`, `"x:1"`, "ascc/nhi-credential-overdue x:1"},
		{"expired in another zone, still in the future", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2026-11-09T23:00:00+14:00"}]}`, `"x:1"`, "ascc/nhi-credential-overdue x:1"},
		{"rotation due before it expires", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2099-01-01T00:00:00Z","rotateBy":"2026-01-01T00:00:00Z"}]}`, `"x:1"`, "ascc/nhi-credential-overdue x:1"},
		{"revoked credentials aren't overdue", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2026-01-01T00:00:00Z","revokedAt":"2026-02-01T00:00:00Z"}]}`, `"x:1"`, ""},
		{"not due yet", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2099-01-01T00:00:00Z","rotateBy":"2098-01-01T00:00:00Z"},{"id":"undated","type":"api-key"}]}`, `"x:1"`, ""},
		{"two overdue credentials are two findings", `{"id":"x:1",` + owned + `,"status":"disabled","credentials":[{"id":"k","type":"api-key","expiresAt":"2026-01-01T00:00:00Z"},{"id":"k2","type":"api-key","rotateBy":"2026-01-01T00:00:00Z"}]}`, `"x:1"`, "ascc/nhi-credential-overdue x:1,ascc/nhi-credential-overdue x:1"},
		{"unlinked", `{"id":"x:1",` + owned + `,"status":"active"}`, ``, "ascc/nhi-unlinked x:1"},
		{"a retired identity nobody uses is fine", `{"id":"x:1",` + owned + `,"status":"retired","credentials":[{"id":"k","type":"api-key","expiresAt":"2026-01-01T00:00:00Z"}]}`, ``, ""},
		{"live while a retired agent still lists it", `{"id":"x:1",` + owned + `,"status":"unknown"}`, `"x:1"`, "ascc/nhi-used-by-suspended-agent x:1"},
		{"a declared agent ascc can't govern is unmonitored, not unlinked", `{"id":"other:vendor-agent","kind":"ai-agent","platform":"other","owner":{"name":"o"},"status":"active","tags":["coverage:enforced"]}`, ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := nhiPDP(t, tt.identity, tt.uses).Inventory("0.0.0", sampleDay, Evidence{})
			raw := validates(t, inv)
			ids := map[string]bool{}
			for _, f := range inv.Findings {
				if ids[f.ID] {
					t.Errorf("finding id %s is used twice", f.ID)
				}
				ids[f.ID] = true
			}
			if got := strings.Join(findings(inv), ","); got != tt.want {
				t.Fatalf("findings %q, want %q", got, tt.want)
			}
			if strings.Contains(raw, "coverage:enforced") {
				t.Error("a declared tag raised a coverage label")
			}
		})
	}
	if n := identity(t, nhiPDP(t, tests[len(tests)-1].identity, "").Inventory("0.0.0", sampleDay, Evidence{}), "other:vendor-agent"); n.Tags[0] != "coverage:unmonitored" {
		t.Errorf("vendor agent tags %v", n.Tags)
	}
}

const importDoc = `{"schemaVersion":"0.1","generatedAt":"2026-11-01T00:00:00Z","producer":{"name":"ghost-grant","version":"1.2.0"},
	"identities":[
	 {"id":"github:app/release-bot","kind":"bot","platform":"github","displayName":"IMPORTED-COPY","status":"unknown"},
	 {"id":"local:agent/dev-agent","kind":"ai-agent","platform":"local","displayName":"IMPORTED-AGENT","status":"active","tags":["coverage:enforced"]},
	 {"id":"slack:bot/standup","kind":"bot","platform":"slack","displayName":"standup","status":"active",
	  "owner":{"name":"ops","email":"ops@example.com","verifiedAt":"2026-10-01T09:00:00+02:00"},"createdAt":null,
	  "credentials":[{"id":"bot-token","type":"oauth-token","fingerprint":"0a1b2c3d4e5f","location":"vault","expiresAt":"2026-03-01T00:00:00Z","lastVerifiedDeadAt":null}],
	  "grants":[{"id":"g1","resource":"channel:standup","permission":"chat:write","access":"write","usageKnown":true,"observationDays":30}],
	  "tools":[{"name":"post","server":"slack","capabilities":["send-message"]}],
	  "tags":["coverage:enforced","imported-from:someone-else","used-by:local:agent/dev-agent","team:ops"], "futureField": 1},
	 {"id":"slack:bot/standup","kind":"bot","platform":"slack","displayName":"SECOND-COPY","status":"active"},
	 {"id":"other:vendor/helpdesk-agent","kind":"ai-agent","platform":"other","displayName":"helpdesk-agent","status":"active","owner":{"team":"support"},"tags":["coverage:enforced"]}],
	"findings":[{"id":"f","identityId":"slack:bot/standup","ruleId":"ghost-grant/x","severity":"low","title":"IMPORTED-FINDING"}],
	"x":{"anything":true}}`

// Spec criterion 16, second half: --import adds a second document's identities without duplicating
// one that has the same id. Nothing in an imported file can raise a coverage label.
func TestInventoryImport(t *testing.T) {
	doc, err := ParseInventory([]byte(importDoc))
	if err != nil {
		t.Fatal(err)
	}
	p := samplePDP(t)
	inv := p.Inventory("0.0.0", sampleDay, Evidence{}, doc, doc) // the same document twice adds nothing twice
	raw := validates(t, inv)

	var ids []string
	for _, n := range inv.Identities {
		ids = append(ids, n.ID)
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	if len(ids) != 10 || len(slices.Compact(sorted)) != 10 {
		t.Fatalf("identities %v", ids)
	}
	for _, not := range []string{"IMPORTED-COPY", "IMPORTED-AGENT", "SECOND-COPY", "IMPORTED-FINDING", "coverage:enforced", "someone-else", "futureField", "anything"} {
		if strings.Contains(raw, not) {
			t.Errorf("the document holds %q", not)
		}
	}
	bot := identity(t, inv, "slack:bot/standup")
	if !slices.Equal(bot.Tags, []string{"team:ops", "imported-from:ghost-grant"}) || bot.Owner.VerifiedAt != "2026-10-01T07:00:00Z" ||
		len(bot.Grants) != 1 || len(bot.Tools) != 1 || bot.Credentials[0].Fingerprint != "0a1b2c3d4e5f" {
		t.Errorf("standup bot: %+v", bot)
	}
	if agent := identity(t, inv, "other:vendor/helpdesk-agent"); !slices.Equal(agent.Tags, []string{"coverage:unmonitored", "imported-from:ghost-grant"}) {
		t.Errorf("imported agent tags %v", agent.Tags)
	}
	got := findings(inv)
	for _, want := range []string{"ascc/nhi-credential-overdue slack:bot/standup", "ascc/nhi-unlinked slack:bot/standup"} {
		if !slices.Contains(got, want) {
			t.Errorf("no finding %q in %v", want, got)
		}
	}
	if len(got) != 8 {
		t.Errorf("findings %v", got)
	}
	// The registry is untouched by an import, and so is a second run.
	if again := validates(t, p.Inventory("0.0.0", sampleDay, Evidence{}, doc, doc)); again != raw {
		t.Error("two runs over the same inputs differ")
	}
	if strings.Contains(validates(t, p.Inventory("0.0.0", sampleDay, Evidence{})), "slack:bot") {
		t.Error("an import leaked into a later run")
	}
}

// An imported file can lower a label to unmonitored, as a discover report does, and nothing else.
func TestImportCoverageTags(t *testing.T) {
	doc, err := ParseInventory([]byte(`{"schemaVersion":"0.1","producer":{"name":"ascc"},"identities":[
		{"id":"local:mcp-server/paste","kind":"mcp-server","platform":"local","status":"unknown","tags":["coverage:unmonitored","source:.mcp.json"]},
		{"id":"x:lookalike","kind":"bot","platform":"other","status":"active","tags":["Coverage:enforced"," coverage:observed","USED-BY:local:agent/dev-agent"]},
		{"id":"local:agent/ghost","kind":"ai-agent","platform":"local","status":"unknown"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	inv := samplePDP(t).Inventory("0.0.0", sampleDay, Evidence{}, doc)
	validates(t, inv)
	for id, want := range map[string][]string{
		"local:mcp-server/paste": {"coverage:unmonitored", "source:.mcp.json", "imported-from:ascc"},
		"x:lookalike":            {"imported-from:ascc"},
		"local:agent/ghost":      {"coverage:unmonitored", "imported-from:ascc"},
	} {
		if n := identity(t, inv, id); !slices.Equal(n.Tags, want) || n.DisplayName != id {
			t.Errorf("%s: tags %v, name %q", id, n.Tags, n.DisplayName)
		}
	}
}

// Parsing changes a registry entry as little as it can, because kill and revive write it back.
func TestRegistryKeepsWhatWasWritten(t *testing.T) {
	reg, err := ParseRegistry([]byte(`{"agents":[{"id":"a","runtime":"r","risk_tier":1,"status":"active"}],
		"identities":[{"id":"x:1","kind":"bot","platform":"github","status":"active","createdAt":"2026-01-31T10:00:00.5+05:30"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(reg)
	if !strings.Contains(string(raw), `"createdAt":"2026-01-31T04:30:00.5Z"`) || strings.Contains(string(raw), "displayName\":\"x") {
		t.Errorf("registry after parsing: %s", raw)
	}
}

const plantedSecret = "sk-planted-not-a-real-key"

// A bad identity stops the registry loading and stops an import, with one line that names the entry
// by position and never repeats what was in it.
func TestIdentitiesFailClosed(t *testing.T) {
	const ok = `"kind":"bot","platform":"github","status":"active"`
	tests := []struct{ name, identity, want string }{
		{"no id", `{` + ok + `}`, "id must look like"},
		{"id with no platform", `{"id":"release-bot",` + ok + `}`, "id must look like"},
		{"id with a line break", `{"id":"github:app\nx",` + ok + `}`, "id must look like"},
		{"made-up kind", `{"id":"x:1","kind":"robot","platform":"github","status":"active"}`, "kind must be one of"},
		{"no platform", `{"id":"x:1","kind":"bot","status":"active"}`, "platform must be one of"},
		{"an agent status", `{"id":"x:1","kind":"bot","platform":"github","status":"suspended"}`, "status must be one of"},
		{"a date that isn't a time", `{"id":"x:1",` + ok + `,"createdAt":"2026-01-31"}`, "createdAt must be a time"},
		{"a bad email", `{"id":"x:1",` + ok + `,"owner":{"email":"` + plantedSecret + `"}}`, "owner.email"},
		{"credential with no id", `{"id":"x:1",` + ok + `,"credentials":[{"type":"api-key"}]}`, "credentials[0].id"},
		{"credential id used twice", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key"},{"id":"k","type":"api-key"}]}`, "credentials[1].id"},
		{"made-up credential type", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"` + plantedSecret + `"}]}`, "credentials[0].type must be one of"},
		{"a secret where the fingerprint goes", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key","fingerprint":"` + plantedSecret + `/+=="}]}`, "never the secret"},
		{"a token where the fingerprint goes", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key","fingerprint":"ghp_plantedFAKEfakeFAKEfakeFAKEfake0000"}]}`, "never the secret"},
		{"a token as the credential id", `{"id":"x:1",` + ok + `,"credentials":[{"id":"xoxb-1111-planted-FAKE","type":"api-key"}]}`, "credentials[0].id looks like a token"},
		{"a year the schema can't write", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key","expiresAt":"9999-12-31T23:59:59-14:00"}]}`, "credentials[0].expiresAt"},
		{"a year before year one", `{"id":"x:1",` + ok + `,"createdAt":"0000-01-01T00:00:00+14:00"}`, "createdAt must be a time"},
		{"an email with a long local part", `{"id":"x:1",` + ok + `,"owner":{"email":"` + strings.Repeat("a", 70) + `@example.com"}}`, "owner.email"},
		{"an email with a long label", `{"id":"x:1",` + ok + `,"owner":{"email":"a@` + strings.Repeat("a", 70) + `.example.com"}}`, "owner.email"},
		{"id that redirects a terminal", `{"id":"x:a\u009b31mb",` + ok + `}`, "id must look like"},
		{"id that reverses its text", `{"id":"x:a\u202eb",` + ok + `}`, "id must look like"},
		{"a bad rotation date", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key","rotateBy":"soon"}]}`, "credentials[0].rotateBy"},
		{"made-up location", `{"id":"x:1",` + ok + `,"credentials":[{"id":"k","type":"api-key","location":"desk-drawer"}]}`, "credentials[0].location"},
		{"grant with no permission", `{"id":"x:1",` + ok + `,"grants":[{"id":"g","resource":"r"}]}`, "grants[0].id, resource and permission"},
		{"negative observation window", `{"id":"x:1",` + ok + `,"grants":[{"id":"g","resource":"r","permission":"p","observationDays":-1}]}`, "observationDays"},
		{"made-up capability", `{"id":"x:1",` + ok + `,"tools":[{"name":"t","capabilities":["fly"]}]}`, "tools[0].capabilities"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, regErr := ParseRegistry([]byte(`{"agents":[{"id":"a","runtime":"r","risk_tier":1,"status":"active"}],"identities":[{"id":"ok:1",` + ok + `},` + tt.identity + `]}`))
			_, impErr := ParseInventory([]byte(`{"schemaVersion":"0.1","identities":[{"id":"ok:1",` + ok + `},` + tt.identity + `]}`))
			for _, err := range []error{regErr, impErr} {
				if err == nil || !strings.Contains(err.Error(), "identities[1]: ") || !strings.Contains(err.Error(), tt.want) ||
					strings.Contains(err.Error(), "\n") || strings.Contains(err.Error(), "planted") {
					t.Fatalf("got error %v, want one line containing %q", err, tt.want)
				}
			}
		})
	}

	agent := `{"id":"a","runtime":"r","risk_tier":1,"status":"active"`
	registries := map[string][2]string{
		"an identity field typo":           {`{"agents":[` + agent + `}],"identities":[{"id":"x:1",` + ok + `,"onwer":null}]}`, `unknown field "onwer"`},
		"an identity listed twice":         {`{"agents":[` + agent + `}],"identities":[{"id":"x:1",` + ok + `},{"id":"x:1",` + ok + `}]}`, "listed twice"},
		"an agent in the identities list":  {`{"agents":[` + agent + `}],"identities":[{"id":"local:agent/a",` + ok + `}]}`, `list it in "agents"`},
		"uses names an undeclared id":      {`{"agents":[` + agent + `,"uses":["x:1","sk-planted"]}],"identities":[{"id":"x:1",` + ok + `}]}`, `uses[1] must be an id from "identities"`},
		"uses with no identities list":     {`{"agents":[` + agent + `,"uses":["x:1"]}]}`, `uses[0] must be an id from "identities"`},
		"uses lists an id twice":           {`{"agents":[` + agent + `,"uses":["x:1","x:1"]}],"identities":[{"id":"x:1",` + ok + `}]}`, `uses[1] must be`},
		"uses that isn't a list of string": {`{"agents":[` + agent + `,"uses":"x:1"}]}`, "registry:"},
	}
	for name, tt := range registries {
		if _, err := ParseRegistry([]byte(tt[0])); err == nil || !strings.Contains(err.Error(), tt[1]) || strings.Contains(err.Error(), "\n") || strings.Contains(err.Error(), "planted") {
			t.Errorf("%s: got error %v, want one line containing %q", name, err, tt[1])
		}
	}

	imports := map[string]string{
		"not JSON":                        `{"schemaVersion":"0.1","identities":[{"id":"` + plantedSecret,
		"an array":                        `[]`,
		"another version":                 `{"schemaVersion":"9.9","identities":[]}`,
		"no version":                      `{"identities":[]}`,
		"no identities":                   `{"schemaVersion":"0.1","identity":[]}`,
		"a non-agent under an agent's id": `{"schemaVersion":"0.1","identities":[{"id":"local:agent/ghost","kind":"service-account","platform":"local","status":"retired"}]}`,
		"identities as map":               `{"schemaVersion":"0.1","identities":{"` + plantedSecret + `":1}}`,
	}
	for name, doc := range imports {
		if _, err := ParseInventory([]byte(doc)); err == nil || strings.Contains(err.Error(), "\n") || strings.Contains(err.Error(), "planted") {
			t.Errorf("%s: got error %v", name, err)
		}
	}
	// A document with no identities is valid and adds nothing.
	if doc, err := ParseInventory([]byte(`{"schemaVersion":"0.1","producer":{"name":"not a name!"},"identities":[]}`)); err != nil || len(doc.Identities) != 0 {
		t.Errorf("empty import: %v", err)
	}
}
