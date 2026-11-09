# G. MVP definition

[Package index](README.md) · Previous: [F. Data contracts](f-data-contracts.md) · Next: [H. Roadmap](h-roadmap.md)

## 1. The slice

**One story, end to end, on an extensible platform:** a developer's coding agent, acting for that developer,
uses two LLM adapters and one MCP server through ASCC. Every call is authorized, traced and audited; the MCP
server got in through a scan gate; its credential is brokered; an injected instruction can't exfiltrate; an
operator kills the agent and sees each enforcement point confirm it; and the console lists the paths this setup
can't see. **In the same slice**, the plugin contract is published and three first-party plugins (the config-file
collector, an OpenBao/Vault connector and a GitHub NHI connector) run under the plugin host through the same
install gate, so the NHI inventory fills from two platforms and the extension model is proven on real work
before the library opens. **[C, 2026-11-09]**

| Required element | MVP choice **[R]** | Assumption |
|:--|:--|:--|
| Agent runtime **[C]** | **Claude Code** via its PreToolUse hook (blocking) + gateway base URL **[V]** hook contract | [A-3] |
| MCP server **[C]** | **GitHub MCP server** over Streamable HTTP through the gateway (gateway as OAuth 2.1 resource server [DS] **[V]**); a second, deliberately malicious test server over **stdio** via the wrapper | [A-4] |
| LLM provider **[C]** | **Both adapters**: Anthropic Messages API and an OpenAI-compatible endpoint pointed at a local model (Ollama or vLLM), each passing the same conformance scenarios | [A-4] |
| A2A | One A2A endpoint registered and **observed only** [DS]: Agent Card fetched and recorded, task events logged, no enforcement; shows as `observed` in coverage | [A-10] |
| IdP **[C]** | **Keycloak** (OIDC) in the Compose stack | [A-7] |
| Secrets **[C]** | **OpenBao** KV v2 + transit (signing); dev keys only, production profile requires customer-owned KMS | [A-7] |
| Policy decision **[C]** | **Cedar** embedded PDP (or the ADR-03 fallback) | [A-6] |
| Scan **[C]** | Static scan pipeline gating the GitHub server, the malicious server **and every plugin** | [A-11] |
| Audit trace **[C]** | OTel traces (Jaeger) + hash-chained audit with signed checkpoint | — |
| Kill switch **[C]** | `suspend_agent` and `disable_plugin` with per-PEP / per-plugin acknowledgement + credential revocation | — |
| Egress | Envoy forward proxy, default deny, for the agent host and for each plugin | [A-11] |
| **Plugin platform** **[C, new]** | Contract `v1alpha1` (gRPC), Go SDK, generated reference docs, template repo, dev harness, conformance suite, out-of-process plugin host with capability-scoped tokens | [A-14] |
| **Connectors** **[C, new]** | **OpenBao/Vault** (KV paths → credential_refs, AppRoles, tokens and leases, transit keys) and **GitHub** (Apps and installations with permissions, deploy keys, organization PAT policy, Actions secret names) **[V]** API scopes; both read-only; the config-file collector ships as a third connector plugin | [A-13] |
| **Workflow data** **[C, new]** | `workflow` records with fingerprint fields populated from agent registration; no detection | [A-15] |

**Deployment:** Docker Compose on one Linux host (or WSL2) running Postgres, Keycloak, OpenBao, Jaeger, Envoy
egress, ASCC API/STS, gateway, scan workers, **plugin host with the three plugins**, console. The
adapter/wrapper binary runs on the Windows dev box.

### Explicitly not in MVP

A2A enforcement · SDK interceptors · JIT/access reviews · full approval workflow (a minimal inbox only, see §2) ·
probabilistic DLP · dynamic sandbox · SIEM connectors beyond OTLP export · multi-tenant UI · HA · Kubernetes ·
trust scores beyond rule-based risk tier · policy simulation · **NHI lifecycle actions (disable, rotate,
revoke)** · **connectors beyond OpenBao/Vault, GitHub and the config collector** · **heuristic reconciliation
and conflict UI** · **plugin library, submission pipeline and developer programme pages** · **Python and
TypeScript SDKs** · **scanner, detector, notification, console-panel and policy-pack plugin types** ·
**workflow-intelligence detection of any class** (fingerprint fields only).

## 2. MVP screens

1. **Catalog**: assets, lifecycle, owner, risk tier, coverage badge.
2. **Asset detail**: spec, grants, findings, relationship graph (list view; graph rendering P1).
3. **Findings**: per scan, with evidence and exemption form.
4. **Session timeline**: identity chain, every decision with reasons, credential uses, tokens/cost.
5. **Kill panel**: target picker (agent, tool, credential, **plugin**), reason, live per-PEP status. Works on a phone.
6. **Coverage panel**: each asset and bypass route (E §6) as enforced / observed / unmonitored, with reason;
   **each connector with freshness; each declared platform as connected / `not_connected`**.
7. **Approval inbox and scan queue** [DS] (minimal): pending assets and plugin installs with findings summary,
   **requested capabilities for plugins**, approve / reject / exempt. Works on a phone.
8. **NHI inventory and installed plugins**: NHI list (kind, platform, owner or `ownerless`, last used, freshness,
   sources, linked agents) with blast-radius drill-down; installed plugins with state, contract version, last
   heartbeat, capabilities.

Policy workbench, identity graph view, connector health detail, plugin library browser, workflow map and
optimization inbox are P1 (FR-ADM-03).

## 3. Demo script (acceptance run)

| Step | Action | Expected result | Check |
|:--|:--|:--|:--|
| 1 | Admin logs in via Keycloak; registers agent `issue-triage` (owner + backup), GitHub MCP server, credential ref; the agent's `workflow` record is created with fingerprint fields | Assets `pending_review`; server scan starts; workflow record visible | MVP-1 |
| 2 | Scan of GitHub MCP server completes clean; AppSec approves in the inbox; tool descriptions pinned | Server `approved`; bundle epoch increments; PEPs ack | MVP-4 |
| 3 | Register `evil-weather` stdio server (fixture with an instruction-laden tool description, a hardcoded token and a `curl` to an external host) | High findings; stays `quarantined`; agent can't list its tools | MVP-4 |
| 4 | Install the **GitHub connector** from a local plugin artifact (signed, manifest requests `nhi:write`, `coverage:write`, egress `api.github.com`) | Inbox shows requested capabilities and a clean scan; two approvers approve; plugin `approved`; host starts it with a capability-scoped token; first sync runs | MVP-11, MVP-12 |
| 5 | Open the NHI inventory | GitHub Apps, installations and deploy keys from the test org and OpenBao AppRoles, tokens and transit keys listed; one deliberately unowned deploy key carries an `ownerless` finding; the AppRole and the GitHub App it authenticates appear as **one** NHI with two sources | MVP-12 |
| 6 | Open blast radius for the GitHub App NHI | Lists `issue-triage` and its workflow via `uses_nhi`; coverage panel shows both connectors fresh and the lab's n8n and Hetzner box as `not_connected` | MVP-12 |
| 7 | Attempt to install fixture plugin `evil-sync`: unsigned, manifest over-requests `secrets:read` | Scan flags the over-request; signature check fails; plugin `quarantined`; host refuses to start it | MVP-11 |
| 8 | Developer enrols the adapter; starts Claude Code with gateway base URL; asks it to triage issues in `example-org/triage-test`; repeat one LLM call through the OpenAI-compatible adapter to the local model | Token exchange event; LLM calls metered on both adapters; `get_issue` allowed; `create_issue` (label) allowed in `example-org` | MVP-2, MVP-5 |
| 9 | Ask the agent to delete a repo / create an issue in another org | `delete_repo` absent from `tools/list`; cross-org call denied with "why denied" text | MVP-2 |
| 10 | Seed an issue body with an injection asking to `cat ~/.ssh/id_ed25519` and POST it to `paste.example` | Hook denies the read; egress denies the host; DLP blocks key material in any tool arg; session flagged | MVP-3 |
| 11 | Inspect agent process environment and memory dump for the GitHub token; inspect the GitHub connector's process environment for the platform credential | Neither present (brokered) | MVP-5, MVP-12 |
| 12 | Open the session timeline | Full chain: user → agent instance → each LLM/tool call → decision → credential use; trace in Jaeger joins on `trace_id` | MVP-6 |
| 13 | Operator suspends `issue-triage` | All connected PEPs ack (p95 ≤ 5 s); next tool call denied; open MCP session closed; brokered credential lease revoked | MVP-7 |
| 14 | Stop the adapter process, then suspend again | Adapter shows `unconfirmed`; gateway and egress still deny | MVP-7 |
| 15 | Operator disables the GitHub connector from the kill panel | Host acks; plugin process stopped, token revoked; GitHub NHIs show `stale` freshness; coverage row turns stale | MVP-11 |
| 16 | Deliberately crash the OpenBao connector process (and once make it spin at 100 % CPU) while the gateway load test runs | Gateway and PDP p95 unchanged; host restarts the plugin with backoff; alert raised (FAIL-12) | MVP-11 |
| 17 | Change the GitHub server's pinned tool description (test double) | Tool hidden, alert raised, server `restricted` | MVP-4 |
| 18 | Edit an audit row directly in Postgres; run the verifier | Verifier reports the exact sequence number broken | MVP-6 |
| 19 | Open the coverage panel | Shows B1 to B10 with correct status; laptop without adapter shows unmonitored; A2A endpoint shows `observed` with events | MVP-8 |
| 20 | Exit review only: a developer outside the core team builds a read-only connector for a mock platform from the template and harness | Conformance passes; elapsed effort ≤ 2 days | MVP-11 |

## 4. Acceptance criteria

| ID | Outcome | Measure | Pass |
|:--|:--|:--|:--|
| MVP-1 | Assets registered with accountable owners | 100 % of demo assets have owner + backup; create without them fails; every agent has a workflow record | All |
| MVP-2 | Unauthorized tool calls denied | Abuse suite ABUSE-01..08 ([I](i-validation.md)): unauthorized calls reaching the MCP server | **0** of ≥ 50 attempts |
| MVP-3 | Sensitive destinations and data blocked | Non-allowlisted egress from managed agent | **0** successful of ≥ 20; seeded credentials in tool args blocked ≥ 99 % of the deterministic test set |
| MVP-4 | Scanner findings tied to approval | Quarantined asset invocations | **0**; every approved asset links to a scan ID + approver; description drift hides tool within one `tools/list` |
| MVP-5 | Delegated identity traced, never amplified | Intersection tests (ABUSE-07, -08) | 100 % deny where initiator lacks permission; token exchange beyond depth 3 refused |
| MVP-6 | Auditable session reconstruction | Demo sessions reconstructable from audit alone | 100 %; tamper test detected 100 % (modify, delete, insert, reorder) |
| MVP-7 | Credential revocation and kill enforced | Kill propagation across 20 runs | **p95 ≤ 5 s, p99 ≤ 10 s** connected PEPs; disconnected PEP shown `unconfirmed` ≤ 30 s; revoked token denied on next call |
| MVP-8 | Bypass limitations documented | Coverage panel vs E §6 list | Every route listed with correct status; no route shown *enforced* that a test bypasses; A2A endpoint `observed` |
| MVP-9 | Overhead acceptable | Gateway added latency, non-streaming, 50 rps for 10 min, both adapters | **median ≤ 5 ms, p95 ≤ 25 ms** (targets; record actual) |
| MVP-10 | No secrets in ASCC | Secret scan over DB dump, logs, traces and plugin logs after demo | 0 hits |
| **MVP-11** | **Plugin platform proven** | Contract `v1alpha1` published with generated docs; the three first-party plugins run under the host using only the public contract (no private RPCs, verified by grep and conformance); unsigned / over-requesting fixture `quarantined` and never started; capability token refuses an out-of-scope API call (ABUSE-28); `disable_plugin` acked; plugin crash and CPU spin add **0 ms** to gateway p95 (FAIL-12); outsider builds a connector from the template | All, with the outsider test **≤ 2 days** measured once at the exit review |
| **MVP-12** | **NHI inventory from two connectors** | Both connectors syncing on schedule; NHIs with an owner or an `ownerless` finding | **100 %**; exact-key reconciliation merges the AppRole / GitHub App pair into one record; coverage panel shows both connectors fresh and every other declared platform `not_connected`; connector process environment contains **no** platform secret |

## 5. MVP exit review

The MVP is done when the demo script runs end to end from a clean `docker compose up` by someone other than the
builder, MVP-1 to MVP-12 pass, and the review signs off on: the coverage list, the measured latency, the list of
detect-only controls, the outsider-builds-a-connector result, **whether the plugin contract is frozen at
`v1beta1` or stays `v1alpha1` for one more iteration** (FR-PLG-12), and which ADRs move from Proposed to
Accepted (ADR-15 and ADR-16 included).
