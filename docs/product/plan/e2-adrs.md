# E2. Architecture decision records

[Package index](README.md) · Back to [E. Architecture](e-architecture.md)

Status values: **Proposed** (this package), **Spike** (needs a Phase 0 spike before acceptance), **Accepted**
(after stakeholder review). Nothing is Accepted yet. ADR-01 to 14 carry the Claude run; where the DeepSeek run
or the 2026-11-09 additions changed a decision, the change is marked **Amended 2026-11-09** inside the ADR.
ADR-15 to 17 are new.

| ADR | Decision | Status |
|:--|:--|:--|
| [01](#adr-01) | Hybrid enforcement with one PDP | Proposed |
| [02](#adr-02) | Go for backend services and PEPs | Proposed |
| [03](#adr-03) | Cedar as the authorization language | Spike |
| [04](#adr-04) | Build the gateway in Go (unless the spike favours an existing project) | Spike |
| [05](#adr-05) | PostgreSQL as the single MVP store | Proposed |
| [06](#adr-06) | Token exchange + ASCC transaction tokens for delegation; SVIDs as server-side instance proof; gateway as OAuth 2.1 resource server | Spike |
| [07](#adr-07) | OpenBao as the bundled secrets backend; Vault, cloud secret managers and PAM via connector plugins | Proposed |
| [08](#adr-08) | Envoy forward proxy with ext_authz for egress | Proposed |
| [09](#adr-09) | gRPC server-stream control channel; no message bus in MVP | Proposed |
| [10](#adr-10) | Hash-chained audit in Postgres with checkpoints signed by customer-owned keys | Proposed |
| [11](#adr-11) | Compose open-source scanners plus ASCC-specific rules | Proposed |
| [12](#adr-12) | Metadata-only telemetry by default | Proposed |
| [13](#adr-13) | Tiered fail-closed behaviour | Proposed |
| [14](#adr-14) | Scores, model reviewers, plugin outputs and workflow findings are advisory only | Proposed |
| [15](#adr-15) | **Plugin runtime: out-of-process gRPC plugins under a host with capability-scoped tokens; WASM later for hot-path detectors** | Spike |
| [16](#adr-16) | **Connectors are plugins; NHI is a first-class registry type with cross-source reconciliation** | Proposed |
| [17](#adr-17) | **Workflow intelligence is advisory, metadata-first, deterministic before semantic, batch** | Proposed |

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
