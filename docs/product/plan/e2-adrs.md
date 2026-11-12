# E2. Architecture decision records

[Package index](README.md) · Back to [E. Architecture](e-architecture.md)

Status values: **Accepted** (the decision stands, with its evidence linked), **Spike** (not accepted yet; the
row says what it waits for: a proof, an owner's decision, or the owner's sign-off), **Replaced**. The statuses
below were set on 2026-11-10 from the Slice 4 spikes and the
[[V] checks](k-next-steps.md#11-v-check-results-2026-11-10). **Only two are Accepted, the two the public MVP
already runs on and tests.** Accepting any other is the owner's call, not a run's: the evidence column is what
there is to decide on, and the roadmap carries the sign-off as an owner item. Where a spike or a check changed
a decision, the change is marked **Amended 2026-11-10** inside the ADR; earlier changes are marked
**Amended 2026-11-09**. ADR-01 to 14 carry the Claude run; ADR-15 to 17 were new on 2026-11-09.

| ADR | Decision | Status | Evidence, and what it waits for |
|:--|:--|:--|:--|
| [01](#adr-01) | Hybrid enforcement with one PDP | **Spike** | Half proven: the public MVP is one PDP (`src/internal/pdp`) with a hook as its first enforcement point, checked against a live Claude Code session. The gateway and egress halves aren't built. Waits for sign-off |
| [02](#adr-02) | Go for backend services and PEPs | **Spike** | The MVP is Go throughout and builds for Windows, Linux and WebAssembly. Amended: an adopted gateway would not be ASCC code, so this turns on K5. Waits for K5 and sign-off |
| [03](#adr-03) | Cedar as the authorization language | **Spike** | [cedar-go check](../../reports/2026-cedar-go-check.md): 9 of 9 requests match the Rust reference. Timed at 500 policies and 5,000 entities: 1.0 ms p99 ([service guide](../../guides/pdp-service.md#how-fast)). Waits for K4 |
| [04](#adr-04) | Build the gateway in Go (unless the spike favours an existing project) | **Spike** | [Gateway report](../../reports/2026-gateway-build-vs-adopt.md): agentgateway meets the adopt rule on a source read, and a [run of v1.7.0](../../reports/2026-gateway-build-vs-adopt.md#the-run-2026-11-11) confirmed it; a deny doesn't end an open stream. Waits for K5 |
| [05](#adr-05) | PostgreSQL as the single MVP store | **Spike** | No evidence yet: nothing is built on it and no check touched it. First used in Slice 5. Waits for sign-off. Since 2026-11-10 PostgreSQL 18 runs in the [development stack](../../../deploy/compose/README.md) with an empty database; that shows it starts, nothing more |
| [06](#adr-06) | Token exchange + ASCC transaction tokens for delegation; SVIDs as server-side instance proof; gateway as OAuth 2.1 resource server | **Spike** | [Proof-of-possession report](../../reports/2026-proof-of-possession.md) settles TKT-003. Waits for the token service (Slice 5), the claim names (TKT-005) and K3 for the unbound-token rule |
| [07](#adr-07) | OpenBao as the bundled secrets backend; Vault, cloud secret managers and PAM via connector plugins | **Spike** | [V] checks: OpenBao is MPL-2.0 and documents the KV v2, AppRole, transit and lease APIs. Read, not run. Waits for TKT-036's test and for K2 and K8. Since 2026-11-10 OpenBao 2.7 runs in the [development stack](../../../deploy/compose/README.md), unsealed, with KV v2 and transit mounted and nothing stored; no ASCC code calls it yet |
| [08](#adr-08) | Envoy forward proxy with ext_authz for egress | **Spike** | No evidence yet; egress is Slice 7. Amended: the Smokescreen alternative was described wrongly. Waits for sign-off. Since 2026-11-10 an Envoy with no upstream runs in the [development stack](../../../deploy/compose/README.md) and refuses every request and `CONNECT`; the allowlist and ext_authz are not built |
| [09](#adr-09) | gRPC server-stream control channel; no message bus in MVP | **Spike** | No evidence yet: nothing is built on it and no check touched it. First used in Slice 5. Waits for sign-off |
| [10](#adr-10) | Hash-chained audit in Postgres with checkpoints signed by customer-owned keys | **Spike** | Half proven: the hash chain runs in the MVP (`src/internal/audit`, `ascc audit verify`). Postgres storage and signed checkpoints aren't built. Waits for sign-off |
| [11](#adr-11) | Compose open-source scanners plus ASCC-specific rules | **Spike** | [V] checks: Semgrep's engine is LGPL-2.1. Amended: its rules can't be redistributed at all. Nothing built. Waits for sign-off |
| [12](#adr-12) | Metadata-only telemetry by default | **Accepted** | The MVP audit line keeps a hash of the tool input and never the input; a test asserts it, and it is an invariant in `AGENTS.md` |
| [13](#adr-13) | Tiered fail-closed behaviour | **Spike** | Waits for K3, the owner's decision on tiers and break-glass. Until then the MVP hook denies on every error of its own, and blocks on a crash or timeout only when registered with `"onFailure": "block"` |
| [14](#adr-14) | Scores, model reviewers, plugin outputs and workflow findings are advisory only | **Accepted** | The MVP decides on policy alone, with no score or model on the path; it is an invariant in `AGENTS.md` |
| [15](#adr-15) | **Plugin runtime: out-of-process gRPC plugins under a host with capability-scoped tokens; WASM later for hot-path detectors** | **Spike** | [Plugin contract spike](../../reports/2026-plugin-contract-spike.md): the model is proven by a prototype. Waits for K7. Amended: a laptop process isn't confined |
| [16](#adr-16) | **Connectors are plugins; NHI is a first-class registry type with cross-source reconciliation** | **Spike** | Waits for K8, the owner's decision on connector scope. Amended with the verified GitHub and OpenBao scopes |
| [17](#adr-17) | **Workflow intelligence is advisory, metadata-first, deterministic before semantic, batch** | **Spike** | Waits for K9, the owner's decision on inputs and audience. Nothing built; Phase 3 |

---

<a id="adr-01"></a>
## ADR-01 Hybrid enforcement with one PDP

- **Context:** Agents act through network calls (LLM, MCP, A2A) and through local tools (shell, files). No single
  choke point sees both. **[C]** principle 3.
- **Decision:** One policy decision point (same policies, same bundle, same decision log format) consulted by
  several PEPs: gateway, runtime adapter, stdio wrapper, credential broker, egress proxy. Coverage per path is
  reported as `enforced | observed | unmonitored` [DS naming].
- **Rejected:** *Proxy only*: blind to local actions. *SDK only*: trivially skipped, not a boundary. *Endpoint
  agent / eBPF first*: overlaps EDR, high cost, admin-level install; revisit Phase 4.
- **Consequences:** More components to version and test; the PDP contract becomes the most important interface.
  Each PEP must implement the same obligation semantics or deny.
- **Amended 2026-11-09:** the plugin host is *not* a PEP. It is a management-side component that feeds the
  registry; it asks the PDP only for its own actions (may this plugin start, may it call this route). Keeping
  extension code off the hot path is what lets NFR-17 hold.

<a id="adr-02"></a>
## ADR-02 Go for backend services and PEPs

- **Decision:** Go for API, STS, PDP host, gateway, adapter/wrapper (single static binary per OS), scanner
  orchestrator, **plugin host and the first-party connectors**. TypeScript/React for the console.
- **Why:** Single static binaries simplify laptop adapters on Windows/macOS/Linux; strong HTTP/2, gRPC and
  streaming support; the cloud-native security ecosystem (OPA, SPIRE, Envoy control planes, go-plugin) is largely Go.
- **Rejected:** *Rust*: best for the hot path and has the reference Cedar implementation, but slower iteration for
  a small team; reconsider for the PDP if ADR-03's spike finds Go Cedar gaps. *Python*: easiest for ML detectors
  but weak for the gateway hot path; detectors may run as separate Python services later, and the plugin
  contract lets a detector plugin be Python anyway (ADR-15). *Node*: fine, but single-binary distribution is weaker.
- **Amended 2026-11-10:** This covers the code ASCC writes. If [ADR-04](#adr-04) ends in adopting agentgateway, the
  proxy itself is Rust and is not ASCC code; the service behind its callout, where ASCC's logic lives, is Go.

<a id="adr-03"></a>
## ADR-03 Cedar as the authorization language

- **Decision (pending spike):** Cedar policies with a typed schema for principals (users, agents, agent
  instances, **plugins**), actions (tool calls, model use, credential use, egress, delegation, **plugin start,
  NHI lifecycle action**) and resources.
- **Why:** `forbid` overrides `permit` by design (matches default-deny + hard-deny rules); schema validation at
  authoring time catches typos that would otherwise silently fail open or closed; evaluation is bounded (no
  unbounded recursion), which suits a latency budget; the policy language is readable enough for a policy
  workbench and for "why denied" explanations; formal analysis tooling exists **[V]**.
- **Alternatives:** *OPA/Rego*: broader ecosystem (Envoy ext_authz, Kubernetes), more flexible, but harder to
  author and to explain; no built-in typed schema for entities. The DeepSeek weighted matrix scored OPA 0.80
  against Cedar 0.78 [DS]: too close to decide on paper, hence the spike. *OpenFGA/SpiceDB*: excellent
  relationship graphs, but ABAC conditions (time, budget, taint, args) need a second engine.
- **Spike (TKT-002):** confirm the Go implementation (`cedar-go`) supports the features we need (schema
  validation, entity hierarchy, context records, partial evaluation not required) at parity with the Rust
  reference **[V]**. If not: run the PDP as a Rust sidecar/library via FFI, or fall back to OPA.
- **Consequences:** Argument constraints (path prefixes, host allowlists) need helper attributes computed by the
  PEP before evaluation, since Cedar has limited string functions; that pre-processing is part of the PEP contract.
- **Amended 2026-11-10:** The parity half of the spike is done in the
  [cedar-go check](../../reports/2026-cedar-go-check.md): cedar-go v1.9.0 gave the Rust reference's answer on 9 of 9 requests and on schema validation. Two things the full TKT-002
  asked for were cut from that check and are still owed: a comparison with OPA (dropped; the MVP runs on Cedar)
  and the timing at 500 policies and 5,000 entities, which is measured when the PDP becomes a service in
  Slice 5 (measured 2026-11-10 with `ascc-pdp`: 0.04 ms mean and 1.0 ms p99 for a decision in the engine,
  inside NFR-01's 2 ms, on one quiet machine; the [service guide](../../guides/pdp-service.md#how-fast) has
  the run). The check also found that a `forbid` that errors is skipped, so the PDP denies unknown entities and
  any evaluation error itself. The analysis tools the **[V]** refers to exist (`cedar-policy-symcc`,
  `cedar-lean-cli`) but are Rust and need an SMT solver, so they run in CI or by hand, not in the Go binary.

<a id="adr-04"></a>
## ADR-04 Build the gateway in Go (unless the spike favours an existing project)

- **Decision (pending spike):** A purpose-built gateway with LLM (Anthropic Messages and OpenAI-compatible), MCP
  and A2A adapters sharing auth, PDP, obligations and telemetry.
- **Why build:** The differentiating features are protocol-semantic: `tools/list` filtering, description
  pinning, per-call authz with delegation intersection, kill confirmation per session. Generic API gateways see
  HTTP requests, not MCP messages.
- **Spike (TKT-001), 1 week:** evaluate agentgateway (Linux Foundation) **[V]** and Envoy AI Gateway **[V]** for:
  MCP Streamable HTTP proxying, `tools/list` filtering hook, external authz with message-level context, session
  termination API, licence, extension model. Adopt if 4 of 5 pass without forking.
- **Rejected:** *LiteLLM as core*: strong LLM routing, but Python hot path and an open-core split where some
  governance features are enterprise-licensed **[V]**; may be supported *behind* ASCC as an upstream. *Kong AI
  Gateway*: key AI features in the commercial edition **[V]**.
- **Amended 2026-11-10:** The spike is done as a source read, not a run:
  [gateway report](../../reports/2026-gateway-build-vs-adopt.md). **agentgateway passes four of the five criteria
  without a fork**, which is this ADR's rule for adopting; Agent Router (the renamed Envoy AI Gateway) passes
  none and ContextForge three. The decision is not changed here. It waits for the run the report lists and for
  decision K5. Two corrections to the text above: Envoy AI Gateway is now Agent Router, an Agentic AI Foundation
  project; and MCP revision 2026-07-28 has no protocol sessions, so "session termination API" and "kill
  confirmation per session" mean cutting off one caller, by a PDP deny on its next request.
- **Amended 2026-11-11:** agentgateway v1.7.0 was run against a stub of the callout
  ([the run](../../reports/2026-gateway-build-vs-adopt.md#the-run-2026-11-11)). A deny reached the client with
  its reason, `tools/list` was filtered per caller, a call was refused with the policy service down, a changed
  tool description was rejected, and the gateway with its callouts added about 1.4 ms a call. So the adopt
  rule is met on a run, not only a reading. **One result went the other way: a deny issued while a response
  stream is open doesn't end it**; the suspended caller gets the rest of the stream and is refused from its
  next call on. The decision is still not changed here: it is K5, the owner's. If it is "adopt", cutting an
  open stream is added work this ADR has to name.

<a id="adr-05"></a>
## ADR-05 PostgreSQL as the single MVP store

- **Decision:** One Postgres for registry, **NHI inventory and reconciliation**, policy store, approvals,
  findings, **workflow fingerprints and optimization findings**, audit chain, and session budget state. JSONB
  for type-specific asset attributes; RLS for tenant isolation from day one.
- **Why:** One backup, one HA story, one set of credentials. MVP volume (single tenant, thousands of events per
  day, a lab-sized NHI estate) is far below Postgres limits.
- **Later:** ClickHouse for analytics queries over events (Phase 3) fed by CDC/outbox; the hash chain stays the
  system of record. Graph queries (blast radius, including `uses_nhi`) use recursive CTEs; a graph DB is not
  justified until measured. NHI inventories of hundreds of thousands of rows are still Postgres-sized.
- **Rejected:** *Graph DB (Neo4j) for registry*: licence (GPLv3/commercial) and an extra store for a graph that
  fits in Postgres. *Separate audit DB in MVP*: extra ops for no MVP benefit; role separation inside Postgres
  instead (writer role can only INSERT).

<a id="adr-06"></a>
## ADR-06 Token exchange + ASCC transaction tokens for delegation

- **Decision:** OAuth 2.0 Token Exchange (RFC 8693) at the ASCC STS. Input: the initiating principal's token
  (human OIDC token or parent transaction token) plus the agent instance's proof. Output: a short-lived
  (≤ 15 min) signed JWT "transaction token" with `sub` (initiator), nested `act` (actor chain, per RFC 8693),
  `task_id`, `purpose`, `scope` (ceiling), `depth`, `budget_ref`, `cnf` (key binding where supported).
- **Rules enforced at exchange:** `may_delegate_to` edge exists; `depth + 1 ≤ max_depth`; requested scope ⊆
  parent scope; no actor appears twice (cycle); parent not revoked.
- **Why:** Standard grant type, standard `act` claim, so SIEMs and other tools can read it. The IETF
  transaction-tokens draft covers a similar need; track it and align claim names if it stabilizes **[V]**.
- **Rejected:** *Shared service accounts per agent*: no initiator, no intersection. *Passing the user's token
  through to tools*: breaks audience binding and the MCP guidance against token passthrough **[V]**. *Macaroons /
  Biscuit attenuation tokens*: elegant attenuation, but non-standard for enterprise IdPs and SIEMs; revisit if
  offline attenuation becomes a requirement. *SPIFFE/SPIRE as the only identity* [DS primary choice]: laptops
  can't run a SPIRE agent sensibly, and the lab's first fleet is laptops.
- **Spike (TKT-003):** proof-of-possession option for laptop agents (R2 in E §18).
- **Amended 2026-11-09:**
  - **Instance proof accepts a SPIFFE SVID** for server-side and Kubernetes agents from Phase 2 (FR-IAM-13,
    pulled forward from P2 to P1) [DS]. Laptops keep instance keypairs. Both bind to the same transaction
    token, so PEPs see one token format.
  - **The MCP gateway is an OAuth 2.1 resource server** to clients: it validates the transaction token as the
    bearer, answers unauthenticated requests with `WWW-Authenticate` pointing at RFC 9728 protected-resource
    metadata, and remains the OAuth *client* to upstream MCP servers through the broker [DS] **[V]** (OAuth 2.1
    is still a draft, `draft-ietf-oauth-v2-1-16` as of 2026-09; no "OAuth 2.1 certification" exists, so none is
    claimed).
- **Amended 2026-11-10:**
  - **Proof of possession is per client** ([report](../../reports/2026-proof-of-possession.md), TKT-003). The
    platform's own clients (the stdio wrapper, the gateway-era hook adapter and CLI, the plugin host; not the
    public MVP's `ascc` binary, which makes no network call) use DPoP (RFC 9449) and the token
    carries `cnf.jkt`. Claude Code presents a client certificate to the LLM gateway (RFC 8705), and the token
    carries `cnf.x5t#S256`. **Claude Code's connection to a remote MCP server can't be bound today:** it sends
    no DPoP proof and the MCP specification names no binding, so that token has no `cnf`. Signed HTTP requests
    (RFC 9421) are not used: no published OAuth profile binds a token to such a key.
  - **A token with no `cnf` should not be enough for a tier 2 or 3 action.** That is a tiering rule and goes to
    decision K3 with [ADR-13](#adr-13).
  - **Claim names.** The transaction-tokens draft is at `-11`, past working-group consensus, and uses `txn`,
    `scope`, `req_wl`, `tctx` and `rctx`. This ADR's token uses `task_id`, `purpose`, `depth` and `budget_ref`.
    TKT-005 decides the mapping when it writes the schema; until then F §3 stands as written.
  - **Corrections.** OAuth 2.1 is at `draft-ietf-oauth-v2-1-17`, still a draft. MCP revision 2026-07-28 did not
    add transport-level authorization; it removed sessions and replaced server-initiated requests with results
    the client answers on a retry. The gateway as an OAuth 2.1 resource server with RFC 9728 metadata, and the
    ban on passing a client's token through, are both confirmed in that revision.

<a id="adr-07"></a>
## ADR-07 OpenBao as the bundled secrets backend; Vault supported

- **Decision:** Dev stack and any redistributed bundle ship OpenBao (MPL-2.0). The broker speaks the KV v2,
  transit and lease APIs common to OpenBao and Vault, so enterprise Vault works unchanged **[V]** API parity
  for the endpoints used.
- **Why:** HashiCorp Vault moved to the Business Source License in 2023; using it internally is generally fine,
  but bundling it in a redistributed product needs legal review **[V]**. The DeepSeek run chose Vault as the
  default [DS]; the difference is only what ships in the box.
- **Amended 2026-11-09:** Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager and PAM products
  (CyberArk and peers) arrive as **connector plugins** (ADR-16): the same plugin both inventories the NHIs the
  secret store knows about and, where the broker needs it, provides the backend adapter. The broker's own
  backend interface stays small (read by reference, lease, revoke); connectors supply the platform-specific part.
- **Amended 2026-11-10:** The **[V]** items are checked. OpenBao is MPL-2.0 and its API documents KV v2, AppRole,
  transit and lease lookup; that was read, not run against both servers, and TKT-036 tests it. Vault's licence
  (BUSL 1.1, now IBM's) bars offering it hosted or embedded in a competing product; it doesn't bar all
  redistribution, so "needs legal review" is the accurate wording and the OpenBao default stands.

<a id="adr-08"></a>
## ADR-08 Envoy forward proxy with ext_authz for egress

- **Decision:** Envoy as an explicit forward proxy (HTTP CONNECT) with an ext_authz callout to the PDP keyed by
  proxy-authenticated identity (agent, **or plugin**); allow by host/SNI; no TLS interception.
- **Alternatives:** *Smokescreen*: simplest egress allowlist, weaker per-identity policy **[V]**. *Squid*: mature,
  helper-based authz, more config surface. *Cloud-native egress (VPC firewalls, Kubernetes NetworkPolicy)*: used
  in addition for server agents and plugin containers.
- **Consequences:** Laptops must route agent traffic via the proxy (PAC file or per-tool proxy env) and a host
  firewall must block direct egress for the agent process, or B5 in E §6 stays open. That's an endpoint
  management dependency, called out in the coverage panel. Plugin containers get the same treatment with no
  endpoint-management caveat: the host owns their network namespace.
- **Amended 2026-11-10:** The Smokescreen alternative above is described wrongly. It does per-client rules,
  keyed by the client certificate's name. Its limit is that a rule is a host allowlist and nothing more, with
  no callout to a policy engine, which is why Envoy with `ext_authz` is still the choice.

<a id="adr-09"></a>
## ADR-09 gRPC server-stream control channel; no message bus in MVP

- **Decision:** Each PEP **and the plugin host** holds a gRPC server-stream to the Mgmt API, receiving bundles,
  revocation deltas and kill commands (each with an epoch) and sending acks + heartbeats on a companion unary
  call. API replicas fan out via Postgres LISTEN/NOTIFY.
- **Why:** Fewest moving parts that still give push latency in seconds and per-PEP acknowledgement; the DeepSeek
  alternative of versioned bundles with ETag polling [DS] gives kill latency equal to the poll interval.
- **Later:** NATS JetStream when PEP count or event volume outgrows Postgres fan-out (measure, don't guess).
- **Rejected:** *Polling only*: kill latency equals poll interval. *Kafka in MVP*: heavy for a dozen PEPs.

<a id="adr-10"></a>
## ADR-10 Hash-chained audit in Postgres with KMS-signed checkpoints

- **Decision:** Append-only `audit_event` table per tenant chain; INSERT-only role for writers; checkpoints signed
  by a KMS key and exported to WORM storage. A verifier CLI checks the chain.
- **Why:** Detects modification, deletion, insertion and reordering between checkpoints; cheap; no new store.
- **Limits:** Tamper-evident, not tamper-proof; a superuser can delete the un-checkpointed tail (≤ 5 min), which
  verification reports as a gap.
- **Later:** External transparency log anchoring (Phase 3) **[V]**.
- **Rejected:** *Blockchain anchoring*: cost and complexity without a stronger guarantee than a signed,
  externally stored checkpoint.
- **Amended 2026-11-09:** in production the **checkpoint key and the token-signing keys are owned by the
  customer in their KMS/HSM; ASCC holds no tenant root key** [DS]. The Claude run had the security team own the
  checkpoint key; the DeepSeek position is stronger and costs nothing in the design, since the signer is already
  an external KMS call. The dev Compose stack generates its own OpenBao transit keys and the production profile
  refuses to start on them (FR-SEC-05). The plugin host writes audit events on behalf of plugins with
  `plugin_id`; plugins never hold a writer role.

<a id="adr-11"></a>
## ADR-11 Compose open-source scanners plus ASCC-specific rules

- **Decision:** gitleaks (secrets), OSV-Scanner (known vulns), Syft (SBOM) + Grype (vulns from SBOM), Semgrep CE
  with ASCC rules (unsafe shell, exfil patterns), plus ASCC's own analyzers: MCP manifest/tool-description
  analyzer, skill/prompt analyzer (hidden Unicode, instruction-like text, external URLs), typosquat check,
  permission-request analyzer, **and a plugin-manifest analyzer** (capability over-request, non-platform egress
  hosts, missing signature or security contact). Normalized into one finding schema (SARIF-compatible).
- **Why:** Mature tools for generic problems; ASCC adds value only on AI-specific artefacts, and its own plugins
  are such artefacts.
- **Licence note:** Semgrep's engine is LGPL-2.1; some rule sets have their own licences **[V]**. Write our own
  rules.
- **Rejected:** *One commercial scanner*: vendor lock, opaque rules. *LLM-only review*: probabilistic, gameable
  (ADR-14).
- **Amended 2026-11-09:** from Phase 2 the ASCC analyzers move onto the plugin contract as `scanner` plugins
  (FR-PLG-15, FR-SC-10), so a third party can add a scanner the same way. The gate semantics do not change.
- **Amended 2026-11-10:** The licence note is stricter than written. Semgrep's engine is LGPL-2.1, and the
  `semgrep-rules` licence allows internal use only: the rules can't be shipped with ASCC or offered as a
  service. "Write our own rules" is therefore a requirement, not a preference.

<a id="adr-12"></a>
## ADR-12 Metadata-only telemetry by default

- **Decision:** Events store identifiers, decisions, hashes, sizes, token counts, classifications and redacted
  previews of whitelisted fields. Full content capture is a policy obligation (`log_content`), per agent or
  tool, encrypted with a separate key, short retention, every read audited.
- **Why:** Prompts and tool results routinely contain personal and regulated data; collecting them by default
  turns the security product into the largest privacy risk in the estate.
- **Trade-off:** Some investigations need content; that's what scoped, time-boxed capture is for.
- **Amended 2026-11-09:** the same rule bounds the new data: NHI records hold credential *metadata* never values;
  workflow fingerprints hold declared purpose and metadata, and prompt text only where capture is already on
  (FR-WFI-08).

<a id="adr-13"></a>
## ADR-13 Tiered fail-closed behaviour

- **Decision:** Tier 1 (low-risk reads) may use a cached decision for up to 5 minutes during a PDP or control-plane
  outage, marked `degraded`; tiers 2 and 3 fail closed. Break-glass enables named tier 2 actions for ≤ 4 h with
  two approvers and loud alerting. Tier 3 has no break-glass.
- **Why:** Fail-closed everywhere pushes operators to disable the product during outages; fail-open everywhere
  makes DoS a bypass (T-21).
- **Amended 2026-11-09:** the DeepSeek rule "sensitive actions fail closed; explicitly tagged observability-only
  paths may fail open" [DS] is the same decision stated as two classes: those tagged paths are tier 1. NHI
  lifecycle actions through connectors are tier 3 (fail closed when the connector, broker or platform is
  unhealthy). A plugin failure never changes a tier's behaviour because plugins are not on the decision path.

<a id="adr-14"></a>
## ADR-14 Scores, model reviewers, plugin outputs and workflow findings are advisory only

- **Decision:** Trust/risk scores and model-based findings can add friction (require approval, lower a budget)
  but cannot be the condition that permits a tier 2/3 action, and can never override a `forbid`. Enforced by a
  policy linter in CI.
- **Why:** **[C]** principle 9 and "never permit an agent's self-rating to grant access". Scores are gameable
  (T-16); friction-only use limits the damage of a wrong score.
- **Amended 2026-11-09 (FR-POL-09):** the same rule covers **everything a plugin reports** (findings, NHI
  attributes, detector signals, `discovered` assets) and **every workflow-intelligence finding**. Plugin-sourced
  and WFI attributes may appear in `forbid` conditions and obligation triggers, never as the sole condition of a
  `permit` on tier 2/3. The linter knows which schema fields are plugin- or WFI-sourced and rejects such permits.
  Reason: T-25 (a plugin shaping decisions) and T-27 (a finding retiring a control) are both "advisory data
  mistaken for authority", the same failure as a gamed score.

<a id="adr-15"></a>
## ADR-15 Plugin runtime: out-of-process gRPC plugins under a host with capability-scoped tokens

- **Context:** Connectors, scanners, detectors, notifiers, runtime adapters and console panels must be addable
  by people outside the core team **[C, 2026-11-09]**. Extension code is untrusted (T-24), and the platform's
  value depends on extension being easy enough that developers actually do it (FR-PLG-09, A §9).
- **Decision (pending spike):** Every plugin runs **out of process** (a container in Compose/Kubernetes, a
  process on a laptop adapter) under an ASCC **plugin host**. The host speaks a versioned gRPC contract
  (`ascc.plugin.v1alpha1`, one `Plugin` service plus one service per type), mints a **capability-scoped token**
  per plugin instance (`plugin_id`, tenant, approved scopes, egress hosts, TTL ≤ 1 h), proxies every call from
  the plugin to the Mgmt API with a per-route scope check, performs platform calls through the broker so the
  plugin never holds a credential, confines egress to the manifest's hosts, applies resource limits, and writes
  audit events on the plugin's behalf. Plugins are registry assets that pass the scan and signature gate before
  the host may start them. **WASM** (wazero or Extism **[V]**) is reserved for a later spike on hot-path
  detectors (R10); until then no third-party code is on the enforcement path.
- **Why:** Process isolation is the only boundary a small team can reason about for hostile code: a crash,
  leak or exploit stays in the plugin's process and network namespace (NFR-17). gRPC with generated SDKs lets a
  plugin be written in Go, Python or TypeScript with the same contract (FR-PLG-08). Capability tokens make
  least privilege mechanical: the approver sees the scopes, the host enforces them, nothing else is reachable
  (T-25). Building first-party connectors on the same contract (FR-PLG-15) is the only honest way to know the
  contract works before advertising it.
- **Spike (TKT-031, TKT-032):** prove the contract and host with the OpenBao and GitHub connectors; decide
  whether to depend on HashiCorp go-plugin for handshake/health **[V]** or write the small host ourselves; measure
  NFR-20 overhead.
- **Rejected:** *In-process Go plugins* (`plugin` package, or linking at build): one trust domain, one crash
  kills the host, Go-only, rebuild per version. *Scripting runtimes* (embedded Lua/JS/Starlark): sandbox
  quality varies, hard to give network isolation, poor fit for long-running connectors. *Webhook-only
  extensibility*: no inventory streaming, no health model, pushes hosting onto the developer, and turns every
  extension into an internet-facing endpoint. *Hosting plugins as a service for customers*: makes ASCC a
  multi-tenant code host; plugins run in the customer's deployment, the library is an index only ([A-14]).
- **Consequences:** A new component (the host) in the trust domain, small and reviewed. Every plugin type needs
  a proto service, a template, a harness fixture set and conformance tests before it is "supported". Latency-
  sensitive extensions wait for the WASM spike. The developer programme (FR-PLG-13) is a product commitment,
  not a side effect: contract stability (NFR-19) binds the core team from `v1` onward.
- **Amended 2026-11-10:** The half of the spike that TKT-031 owns is done:
  [plugin contract spike](../../reports/2026-plugin-contract-spike.md), code in `src/spikes/pluginhost/`. A
  connector on the `v1alpha1` contract runs as a separate process, holds no credential, is refused a call
  outside its grant, and every event carries its id, set by the host. What it changes:
  - **A process is not a sandbox.** The host checks what a plugin asks it for. On a laptop, run as a plain
    process, a plugin can still open its own connections and read its user's files. "Confines egress to the
    manifest's hosts" and the resource limits hold only under a container or an OS sandbox, so a laptop
    adapter needs one of those too, or the plugin there is reported as unconfined.
  - **Write the host; don't depend on go-plugin.** Its two-way calls need a broker protocol every non-Go SDK
    would have to implement, and it starts child processes where Compose and Kubernetes run containers. Plain
    gRPC on two endpoints, one per direction, works for both. go-plugin itself was not prototyped.
  - **The contract carries no identity fields, and no call returns the host's credential.** Who is asking
    comes from the token, so there is nothing for a plugin to forge. A platform's answer can still hold a
    secret (reading an OpenBao KV value is a `GET`): the prototype limits the host a plugin may call, not
    the path, so what a connector can read is set by its platform credential's scope. TKT-032 adds path limits.
  - **The token needs a renewal path.** A one-hour token with no renew call ends a long-running plugin.
  - **NFR-20:** one host round trip measured 0.34 ms mean and 1.0 ms p95 on one machine, inside the 2 ms target.

  The status stays Spike: the runtime model, the SDK licence and the library's home are decision K7.

<a id="adr-16"></a>
## ADR-16 Connectors are plugins; NHI is a first-class registry type with cross-source reconciliation

- **Context:** ASCC must manage and track non-human identities, which live in clouds, IdPs, vaults, source hosts,
  directories and workflow platforms **[C, 2026-11-09]**. An agent's authority is only as governed as those
  identities. Nobody owns most of them ([B Q13]).
- **Decision:** A `connector` is a plugin type (ADR-15) implementing `Discover`, `Enrich`, optional `Act` and
  `Health` for one platform. Its output lands in a new registry type **`nhi_account`** with a platform-neutral
  `kind`, a `sources[]` list and `uses_nhi` edges to agents and workflows. **Reconciliation** merges records
  from several connectors on deterministic keys (ARN, client ID, object ID, key fingerprint, vault path);
  heuristic merges are suggestions a human confirms. The inventory holds **every** NHI a connector can see,
  with `ai_linked` as an attribute [A-13]. Lifecycle actions (`disable`, `rotate`, `revoke`, …) run through the
  connector's `Act` with a **separate write credential**, two approvers, dry-run, rate limit and verification
  (FR-CON-08). Read and write credentials are brokered and never in the plugin process (FR-SEC-06). MVP ships
  two read-only connectors (OpenBao/Vault, GitHub) and the config-file collector on the same contract.
- **Why:** Connectors are the archetypal third-party extension: many platforms, platform-specific APIs, a shape
  the core team cannot keep up with alone, and a long tail where a customer's own engineers know the platform
  best. Making them plugins means the plugin platform is exercised on day one. A first-class NHI type (instead
  of attributes on `credential_ref`) is needed because most NHIs have no `credential_ref` in ASCC: they are
  identities ASCC has never brokered and may never broker, and those are exactly the ones that need an owner
  and a retirement date. Reconciliation on deterministic keys only (MVP) because a false merge hides an identity
  (R11); duplicates are visible, merges are not.
- **Rejected:** *NHIs as attributes on `credential_ref` only*: covers brokered credentials, misses the estate.
  *Pulling NHIs from the IdP or IGA only*: most cloud keys, deploy keys, CI tokens and vault roles never appear
  in an IdP; IGA products are a connector target, not the source. *ASCC as system of record* for identities:
  would make it an IAM product and put it in the write path of every platform; ASCC reads, reconciles and
  requests changes. *Write scope from day one*: tenants will not hand a new product a key that can disable
  their service accounts; read-only first earns that (FR-CON-08 is P1). *AI-linked NHIs only* [B Q13
  alternative]: halves the data and most of the value; the unlinked ones are the shadow-AI and retirement
  candidates.
- **Consequences:** The registry grows a potentially large table and a reconciliation job (NFR-18). Coverage
  gains a per-platform dimension (`not_connected`). The kill switch gains NHI actions routed through connectors
  (FR-CON-12). Every connector needs a documented, verified scope list (**[V]** per platform, R13), which is
  real work per platform and the reason Phase 2 lists eight of them, not thirty. The inventory itself is a
  high-value asset (T-26) and gets its own role and audited reads.
- **Amended 2026-11-10:** The first two connectors' scopes are checked against the platforms' documentation
  (the table is in the
  [plugin contract spike](../../reports/2026-plugin-contract-spike.md#connector-scopes-v-recorded-for-tkt-036-and-tkt-037)).
  Two things differ from the plan. GitHub's organization token lists can be read only by a GitHub App, and no
  endpoint reads the organization's token *policy*, so the GitHub connector authenticates as an App and reports
  that setting as not readable. And listing OpenBao leases needs `sudo` on the lease-lookup path, so that
  connector's "read-only" credential is not read-only in OpenBao's own terms; its scope list must say so.

<a id="adr-17"></a>
## ADR-17 Workflow intelligence is advisory, metadata-first, deterministic before semantic, and runs as a batch

- **Context:** With agents, tools, data sources, destinations, schedules and credentials for every workflow in
  one registry, and their calls in one event stream, ASCC can see where several workflows solve the same job
  **[C, 2026-11-09]**. The ask is to detect redundancy and *suggest* consolidation.
- **Decision:** A `workflow` registry type carries a **fingerprint** (declared and observed) from registration,
  connectors and daily event rollups. A scheduled **batch engine** runs deterministic rules first (set overlap
  on tools, inputs, destinations, schedules and shared NHIs with configurable thresholds; idle, pass-through,
  shared-service and credential-consolidation rules) and writes `optimization_finding` records with evidence
  and an *estimate*. **Semantic similarity** (local embedding model, P2) can raise confidence or create `low`
  findings, never a `medium`/`high` finding alone. Findings become **proposals** sent to every involved owner;
  owners accept, dismiss with a reason, or snooze; the engine later **verifies** the measured effect and uses it
  to calibrate estimates. ASCC never edits, merges or retires a workflow, and WFI attributes never condition a
  permit (ADR-14, FR-WFI-10). Inputs are metadata and declared purpose by default; prompt content only where
  capture is already on by policy [A-15].
- **Why:** Advisory because the failure mode is retiring a control that looks redundant (T-27, S12), and only
  the owners know intent. Deterministic first because set overlap on tools, sources and destinations is
  explainable evidence an owner can check in a minute, and it needs no model, no content and no GPU; semantic
  similarity is probabilistic and belongs in the same "signal, not control" box as every other classifier in
  this package. Batch because nothing here is time-critical and nothing may touch the request path (NFR-21).
  Metadata-first because the workflow map is itself sensitive aggregation (T-27) and the privacy default of
  ADR-12 should not be weakened by an optimization feature.
- **Rejected:** *LLM-judged similarity as the primary method*: sends every workflow's purpose (and possibly
  prompts) to a model, opaque, un-auditable, gameable, and the lab's own fleet is the first customer. *Auto-
  consolidation* (ASCC disables or merges workflows): ASCC is not the system of record for n8n, GitHub Actions
  or any platform, and a wrong merge is an outage with ASCC's name on it. *Content-based by default*: breaks
  the metadata-only promise. *Real-time detection on the gateway*: no user value in a sub-second redundancy
  alert, and a cost on the hot path.
- **Consequences:** A new registry type, a batch job, two console surfaces (workflow map, optimization inbox)
  and a notification path. The first labelled ground-truth set has to come from the lab fleet (R12), so the
  default thresholds will be wrong at first and are configuration, not code. Value is measured by accepted
  proposals and verified deltas (A §9), not by findings produced.
