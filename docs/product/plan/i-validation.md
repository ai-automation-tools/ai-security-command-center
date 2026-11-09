# I. Validation plan

[Package index](README.md) · Previous: [H. Roadmap](h-roadmap.md) · Next: [J. Backlog](j-backlog.md)

Every control in [C](c-requirements-catalog.md) gets at least one automated test; every threat in
[D](d-threat-model.md) rated High or Critical gets an abuse case. Tests run in CI against the Compose stack
unless marked *manual* or *exercise*. The DeepSeek run's security test plan and red-team bullets [DS] are folded
in below: its stdio/direct-socket bypass, bundle replay and audit-tamper exercises are ABUSE-13, FAIL-10/11 and
ABUSE-17; its abuse cases T5 to T7 are ABUSE-25 to 27.

## 1. Test layers

| Layer | What | Tooling **[R]** | Gate |
|:--|:--|:--|:--|
| Policy unit tests | Each policy: permit and deny cases, incl. edge args | Cedar test harness (or OPA test) in CI | Merge |
| Policy lint | No score-only or plugin-attribute-only permits on tier 2/3 (ADR-14, FR-POL-09); every `permit` has an `@id`; no wildcard principal on tier 3 | Custom linter | Merge |
| Contract tests | JSON Schemas for entities (incl. `nhi_account`, `connector`, `plugin`, `workflow`), events, tokens, plugin manifest; OpenAPI conformance; **plugin proto compatibility check (no reused tags, no breaking change within a major)** | Schema validators, OpenAPI linter, buf breaking | Merge |
| PEP conformance suite | Same decision scenarios run against gateway, wrapper, adapter: identical decisions and events | Go test suite with a scenario table | Merge |
| **Plugin conformance suite** | Every plugin type: `Describe`/`Health` semantics, capability-token refusal, no raw-secret path, events carry `plugin_id`, graceful shutdown, pagination and cursor resume for connectors | Go suite shipped in the dev kit; runs in the harness against a mock control plane | Merge (first-party plugins); required for `verified` tier |
| **Connector fixture replay** | Each connector against recorded platform API traffic (success, pagination, rate limit, permission denied, schema drift) | Record/replay in the harness | Nightly; release |
| Abuse suite | §2 below | Go + scripted MCP clients + fixture servers + fixture plugins | Merge (MVP set) |
| Failure injection | §4 below | Compose + toxiproxy-style fault injection; plugin host fault hooks | Nightly |
| Performance | §5 below | k6 or vegeta load against gateway; batch timings for connectors and WFI | Nightly + release |
| Interop | §6 below | Real clients, servers and platforms pinned by version | Release |
| ASCC's own AppSec | OWASP ASVS L2 checklist, SAST, dependency scan, container scan, secret scan; same for first-party plugins | Semgrep, OSV-Scanner, Trivy/Grype, gitleaks | Release |
| Red team | §3 below | *Exercise* | Phase exit |

## 2. Abuse-case catalogue

| ID | Threat | Attack | Expected | MVP? |
|:--|:--|:--|:--|:--|
| ABUSE-01 | T-02 | Call a tool the agent has no grant for | JSON-RPC error; server receives nothing; `deny` event | ✅ |
| ABUSE-02 | T-04 | Call a tool hidden from `tools/list` by guessing its name | Same as 01 | ✅ |
| ABUSE-03 | T-04, T-07 | Indirect injection makes the agent call an *allowed* tool against a non-allowlisted repo/owner | Arg constraint deny | ✅ |
| ABUSE-04 | T-04 | Arg-constraint evasion: case changes, Unicode confusables, `../` traversal, URL-encoding, trailing whitespace, JSON duplicate keys | All denied; PEP normalizes before evaluation | ✅ |
| ABUSE-05 | T-06 | Server changes a pinned tool description or schema | Tool hidden; asset `restricted`; alert | ✅ |
| ABUSE-06 | T-05 | Invoke a `quarantined` or `pending_review` server | Deny | ✅ |
| ABUSE-07 | T-02 | Confused deputy: initiator lacks permission the agent's grant has | Deny with `forbid-exceeds-initiator` | ✅ |
| ABUSE-08 | T-17 | Token exchange widening scope, exceeding depth, or delegating to an agent without `may_delegate_to` | Exchange refused | ✅ |
| ABUSE-09 | T-05 | Typosquatted skill with install script and hidden Unicode | High findings; quarantined | ✅ |
| ABUSE-10 | T-05 | MCP server with hardcoded token and outbound `curl` | Findings; quarantined; its own egress denied if run | ✅ |
| ABUSE-11 | T-03 | Replay a stolen transaction token from another host | Rejected with PoP; without PoP: logged anomaly, valid ≤ TTL (documented) | ✅ |
| ABUSE-12 | T-03 | Use an expired, revoked or wrong-audience token; alg=none; key confusion | Rejected | ✅ |
| ABUSE-13 | T-11 | Point the agent directly at the provider with a personal key (managed host); launch a stdio server without the wrapper; open a direct socket [DS] | Egress refused; coverage flags `unmonitored` or `observed` honestly | ✅ |
| ABUSE-14 | T-07 | Egress to non-allowlisted host via IP literal, alternate port, IPv6, CONNECT to allowed host with different SNI | All refused | ✅ |
| ABUSE-15 | T-10, T-17 | Delegation loop A → B → A; runaway tool calls | Cycle/depth deny; budget exhaustion 429 | ✅ (depth, budget) |
| ABUSE-16 | T-07 | Secrets in tool args: AWS keys, GitHub tokens, PEM keys, card numbers (Luhn-valid) | Blocked by deterministic DLP | ✅ |
| ABUSE-17 | T-15, T-13 | Admin self-approves a grant; edits/deletes audit rows | API refuses (P1); verifier detects tamper | Tamper ✅ |
| ABUSE-18 | T-22 | Agent runs `curl … \| sh`, `rm -rf`, writes outside project via `Bash` | Hook denies by command policy; documented as bypassable (R6) | ✅ |
| ABUSE-19 | T-09 | Request routed to a provider not allowed for the data class | Deny | P1 |
| ABUSE-20 | T-04 | Tier 3 action after untrusted content without approval | `require_approval` obligation | P1 |
| ABUSE-21 | T-18 | Tenant A reads tenant B's assets, NHIs, events, bundles via API, cache, or crafted IDs | 404/403; isolation suite | P3 |
| ABUSE-22 | T-16 | Inflate a trust score to unlock an action | Linter + PDP prove score can't permit | P4 |
| ABUSE-23 | T-20 | Console/API: IDOR, CSRF, SSRF via asset URLs (scan fetch), stored XSS via asset names, findings **or NHI display names from connectors** | Blocked; ASVS tests | ✅ |
| ABUSE-24 | T-05 | Scanner escape: artifact tries to reach DB or network from a worker | Worker has no route or creds; attempt logged | ✅ (static), P1 (sandbox) |
| ABUSE-25 | T-07 | Agent tries to print or echo a brokered vault token into a response or tool arg [DS T5] | Never present to print (brokered); if a brokered prefix appears, redacted and leak alert raised 🔍 | ✅ (brokered), P1 (leak detection) |
| ABUSE-26 | T-19 | Poisoned retrieval source feeds the agent a persistent instruction [DS T6] | Content tagged untrusted with provenance; taint rule gates tier 3 (P1); detector flag 🔍 | P1 |
| ABUSE-27 | T-15, T-16 | Two agents approve each other's access requests or validate each other's output [DS T7] | SoD across the actor chain refuses; human approval required for tier 3; self-rating ignored | P1 |
| **ABUSE-28** | T-24, T-25 | Plugin requests a raw secret value, calls a management API outside its capability token, or tries to write an audit event directly | All refused with 403; events with `plugin_id` logged; repeated attempts raise an alert | ✅ |
| **ABUSE-29** | T-24 | Install an unsigned plugin; a tampered artifact (digest mismatch with manifest); an upgrade whose manifest adds a capability | Unsigned and tampered → `quarantined`, never started; upgrade stops at approval showing the delta | ✅ |
| **ABUSE-30** | T-23 | Connector write credential used to disable many NHIs at once | Each action needs two approvers and a dry-run; rate limit refuses above N per connector per hour; every attempt audited | P1 |
| **ABUSE-31** | T-23 | Connector credential replayed from outside the plugin host (copied token, different source) | Credential is host-bound and brokered; broker refuses; event and alert | ✅ |
| **ABUSE-32** | T-27 | Attacker registers a decoy workflow to make a real one look redundant and get it retired | Finding is advisory; every involved owner must accept; decoy flagged by provenance and age; nothing is changed by ASCC | P1 |

## 3. Red-team exercises

| Exercise | When | Scope | Success for the defenders |
|:--|:--|:--|:--|
| RT-1 "Steal a token" | Phase 1 exit | Internal team gets a laptop with a managed agent; goal: obtain the GitHub credential or act outside grants | No credential obtained; every attempt in audit; bypasses found are added to E §6 |
| RT-2 "Poisoned tool" | Phase 2 | Publish a malicious MCP server/skill through the onboarding pipeline | Stopped at scan or approval, or caught by sandbox; time-to-detect recorded |
| RT-3 "Rogue insider" | Phase 2 | Policy author tries to grant themselves prod access and hide it | SoD blocks; tamper evident; alerting fires |
| RT-4 "Make it fail open" | Phase 3 | DoS gateway/PDP/control channel/plugin host | Tier 2/3 stay closed; plugin host outage never opens enforcement; break-glass usage audited |
| RT-5 External pen test | Phase 3 | Whole product incl. plugin host and two connectors | No open High/Critical at GA |
| RT-6 Prompt-injection campaign | Each phase | Library of indirect injections (web, issues, files, tool results) against the demo agent | Measured: % that reach a *denied* action vs. % that achieve harmful *allowed* actions |
| **RT-7 "Malicious plugin"** | Phase 2 | Publish a plugin through the submission pipeline that tries to exfiltrate NHI inventory data or escalate its capabilities | Stopped at scan, conformance or review; if listed as `community`, capability token and egress allowlist stop the exfil; takedown exercised |
| **RT-8 "Connector takeover"** | Phase 2 | Obtain a connector's platform credential (from the host, broker or a backup) and map the estate or disable NHIs | Credential unusable outside the host; write scopes need approvals; NHI inventory access is role-gated and audited (T-26) |

## 4. Failure-injection tests

| ID | Fault | Expected behaviour (from E §9) |
|:--|:--|:--|
| FAIL-01 | PDP service unreachable from a remote PEP | Tier 1 cached ≤ 5 min `degraded`; tiers 2/3 deny |
| FAIL-02 | Bundle older than `max_staleness` | Tiers 2/3 deny; PEP reported stale |
| FAIL-03 | Postgres down | Mgmt read-only; audit buffers then tier 2/3 deny when buffer full |
| FAIL-04 | OpenBao down | Brokered calls fail closed; clear error; connector syncs pause and show `error` |
| FAIL-05 | Keycloak down | Existing tokens work until TTL; new logins fail |
| FAIL-06 | Adapter offline during a kill | Status `unconfirmed` ≤ 30 s; gateway/egress deny; escalation event |
| FAIL-07 | Gateway replica dies mid-stream | Client error; no duplicate tool execution on retry (idempotency key on `tools/call` where server supports) **[V]** |
| FAIL-08 | Clock skew ±5 min on a PEP | Token validation within allowed leeway (60 s) fails closed beyond it; alert |
| FAIL-09 | Audit buffer disk full | Tier 2/3 deny; alert |
| FAIL-10 | Control channel replays an old epoch | PEP rejects lower epoch |
| FAIL-11 | Corrupted / unsigned bundle | PEP rejects, keeps last good, reports |
| **FAIL-12** | Plugin crashes, hangs, or spins at 100 % CPU | Gateway and PDP p95 unchanged (NFR-17); host restarts with backoff within 10 s; coverage it provided marked `stale`; alert |
| **FAIL-13** | Connected platform API down, rate-limited or returning a changed schema | Sync fails cleanly; connector coverage `stale` or `error` with reason; no partial overwrite of the inventory (last good snapshot kept); retry with backoff |
| **FAIL-14** | Plugin host down | Enforcement path unaffected; all connector coverage `stale`; plugin installs blocked; NHI inventory read-only with freshness shown |

## 5. Performance and SLO targets (provisional)

| SLO / test | Target | Method |
|:--|:--|:--|
| Gateway added latency, non-streaming | median ≤ 5 ms [DS], p95 ≤ 25 ms, p99 ≤ 50 ms at 50 rps per replica, both LLM adapters | Mock upstream with fixed latency; subtract baseline |
| Streaming time-to-first-token overhead | ≤ 10 ms p95 | Mock streaming upstream |
| PDP evaluation | embedded p99 ≤ 2 ms with 500 policies, 5k entities; remote p95 ≤ 10 ms [DS] | Micro-benchmark; networked benchmark |
| Kill propagation (connected PEPs and plugins) | p95 ≤ 5 s, p99 ≤ 10 s | 20 runs, 10 PEPs + 3 plugins |
| Revocation enforced (connected) | ≤ 10 s | Same |
| Event ingest | 1,000 events/s sustained [DS] | Load generator against the event ingest |
| Audit verification throughput | ≥ 50k events/s on one core | Benchmark |
| Scan turnaround (static, typical MCP server repo or plugin) | ≤ 5 min p95 | Pipeline timing |
| Control-plane availability (MVP target) | 99.5 % [DS] | Synthetic probes |
| Availability of the enforcement path (Phase 3) | 99.9 % monthly | Synthetic probes |
| **Plugin isolation (NFR-17)** | 0 ms added to gateway p95 during FAIL-12; restart ≤ 10 s | FAIL-12 under load |
| **Connector freshness (NFR-18)** | hourly sync; `stale` after 2 missed intervals; 50,000 NHIs reconciled in ≤ 15 min | Phase 2 load test with synthetic platform |
| **Plugin contract stability (NFR-19)** | 0 breaking changes within a major; compatibility matrix published per release | buf breaking in CI; release checklist |
| **Plugin host overhead (NFR-20)** | ≤ 2 ms p95 per connector API round trip; ≤ 5 % CPU per idle plugin | Benchmark through the host vs direct |
| **WFI batch (NFR-21)** | 1,000 workflows in ≤ 10 min on the reference Compose host | Phase 2 batch timing |

Targets are hypotheses until Phase 1 measurements; results are published with hardware and config.

## 6. Interoperability tests

| Area | Matrix (MVP → later) |
|:--|:--|
| MCP clients | Claude Code (MVP), Claude Desktop, VS Code, Cursor, an SDK client (Python, TS) **[V]** current transport support |
| MCP servers | GitHub (Streamable HTTP, OAuth 2.1 bearer + RFC 9728 metadata [DS] **[V]**), filesystem (stdio), one OAuth-protected remote server, malicious fixtures |
| MCP spec revisions | Pinned current revision **[V]**; previous revision if clients still use it |
| LLM APIs | Anthropic Messages (streaming + tools), OpenAI-compatible chat (vLLM/Ollama/llama.cpp server), OpenAI Responses (P1) |
| IdPs | Keycloak (MVP), Entra ID, Okta (P1/P3) |
| Secrets | OpenBao (MVP), Vault (P1) |
| A2A | Reference SDK client/server **[V]**, observe-only (MVP), enforce (P1) |
| Runtimes | Claude Code version range (MVP); second runtime (P2) |
| OS for adapter | Windows 11 (MVP), macOS, Linux |
| **Connector targets** | OpenBao and HashiCorp Vault API versions **[V]** (KV v2, AppRole, transit, lease listing); GitHub REST and GraphQL API version **[V]** (Apps, installations, deploy keys, PAT policy, Actions secret names); later AWS IAM, Entra ID / Microsoft Graph, GCP IAM, Okta, Kubernetes, AD/LDAP, n8n **[V]** each |
| **Plugin SDK** | Go on Windows 11, macOS, Linux (MVP); Python and TypeScript (P1) |
| **Plugin host** | Docker Compose (MVP); Kubernetes with per-plugin NetworkPolicy (P2) |

## 7. Compliance evidence mapping

Mapping shows which ASCC control *produces evidence for* a framework objective. It is not a claim of compliance;
framework references are to be checked against current official text **[V]**.

| ASCC control | Evidence produced | NIST AI RMF | NIST CSF 2.0 | ISO/IEC 27001:2022 Annex A | ISO/IEC 42001 | OWASP LLM / Agentic | HIPAA Security Rule | SOX ITGC | GDPR | PCI DSS v4 |
|:--|:--|:--|:--|:--|:--|:--|:--|:--|:--|:--|
| Registry + owners + lifecycle | Inventory export, owner list, lifecycle history | GOVERN, MAP | ID.AM | 5.9, 5.10 | AI system inventory and roles | Agentic supply chain / rogue agents **[V]** | §164.308(a)(1) risk analysis input | Change mgmt inventory | Art. 30 records (input) | Req. 12.5 scope inventory |
| Agent identity + delegation tokens | Token logs, actor chains | GOVERN, MANAGE | PR.AA | 5.15–5.18, 8.2, 8.5 | Access to AI resources | LLM06 Excessive Agency; identity & privilege abuse **[V]** | §164.312(a)(1), (d) | Logical access | Art. 32 | Req. 7, 8 |
| Per-call authorization + decision logs | Decision events with policy IDs | MANAGE | PR.AA, PR.PS | 8.3, 5.15 | Operational controls | LLM06, tool misuse **[V]** | §164.312(a)(1) | Logical access | Art. 25, 32 | Req. 7 |
| Supply-chain scan gate | Scan reports, approvals, exemptions | MAP, MEASURE | ID.RA, GV.SC | 5.19–5.22, 8.8, 8.28 | Third-party AI components | LLM03 Supply Chain | §164.308(a)(1) | Change mgmt | Art. 28 (processor diligence input) | Req. 6.3 |
| **Plugin install gate** | Plugin scan reports, signature checks, capability approvals, upgrade deltas | MAP, MEASURE | ID.RA, GV.SC | 5.19–5.22, 8.8, 8.28 | Third-party AI components | LLM03 Supply Chain | §164.308(a)(1) | Change mgmt | Art. 28 (input) | Req. 6.3, 6.4 |
| Credential broker | Credential-use events, no secrets in agents or plugins | MANAGE | PR.AA, PR.DS | 8.24, 5.17 | — | LLM02 Sensitive Info Disclosure | §164.312(a)(2)(iv), (e) | Access | Art. 32 | Req. 3, 8.6 |
| **NHI inventory + hygiene findings + lifecycle actions** | NHI export with owner, last use, credential age; `ownerless` / `stale` / `over_privileged` findings; approved disable/rotate records | GOVERN, MAP | ID.AM, PR.AA | 5.16, 5.18, 8.2, 8.5 | Roles and resources for AI systems | Agentic identity and privilege abuse **[V]** | §164.308(a)(3), (a)(4) | Logical access, periodic review | Art. 32 | Req. 7, 8.6 |
| Egress + deterministic DLP | Blocked destination events, DLP hits | MANAGE | PR.DS, DE.CM | 8.12, 8.23 | Data for AI systems | LLM02 | §164.312(e)(1) | — | Art. 32, 44+ (transfer control input) | Req. 1, 3.3 |
| Audit chain + checkpoints | Verifier reports | MEASURE | DE.CM, PR.PS | 8.15, 8.16 | Logging of AI system events | — | §164.312(b) | Audit trail integrity | Art. 5(2) accountability | Req. 10 |
| Kill switch + acks | Containment records incl. plugin and connector acks | MANAGE | RS.MI | 5.24–5.26 | AI incident handling | Rogue agents / cascading failures **[V]** | §164.308(a)(6) | Incident mgmt | Art. 33 (input) | Req. 12.10 |
| Budgets + depth limits | Budget events | MANAGE | PR.PS | 8.6 | — | LLM10 Unbounded Consumption | — | — | — | — |
| Metadata-only telemetry, retention | Retention config (13 months [DS]), DSAR exports | GOVERN | GV.PO | 5.34 | Data management | — | §164.530(j) retention (input) | — | Art. 5(1)(c), 15, 17 | Req. 3.2 |
| **Workflow intelligence** | Workflow inventory with fingerprints; overlap findings; accepted/dismissed proposals with rationale. *Evidence of inventory and rationalization, not a control* | MAP, MANAGE | ID.AM | 5.9 (input) | Impact assessment input | — | — | — | Art. 5(1)(c) minimization (input) | — |

Notes:
- HIPAA and PCI DSS applicability depends on whether agents touch PHI or cardholder data; ASCC evidence supports
  but doesn't establish the control.
- OWASP Agentic item names are marked **[V]**; align to the published list at Phase 0.
- The DeepSeek run's framework-level table [DS] is subsumed by the rows above.

## 8. Definition of "validated" per phase

| Phase | Validated means |
|:--|:--|
| 1 | MVP abuse set (✅ rows incl. ABUSE-28, 29, 31) green; FAIL-01..06, 10, 11, **12, 14** green; plugin conformance suite green for the three first-party plugins; SLO measurements recorded; RT-1 run |
| 2 | All P1 abuse rows green incl. **ABUSE-30, 32**; **FAIL-13**; connector fixture replay suite green for every shipped connector; detectors have published precision/recall; **WFI deterministic class has a published precision and accept-rate report**; RT-2, RT-3, **RT-7, RT-8** run |
| 3 | Full suite + isolation suite; connector scale test; RT-4, RT-5; 30-day soak |
| 4 | Scoring calibration report incl. NHI hygiene; WFI savings calibration; ABUSE-22 proven |
