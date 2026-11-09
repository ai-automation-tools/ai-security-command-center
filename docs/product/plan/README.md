<h1 align="center">✅ Combined Planning Package</h1>

<p align="center">
  <em>The ASCC plan from charter to backlog, merged from two runs. The source of truth for planning and analysis.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-draft_v0.3-F59E0B?style=for-the-badge" alt="Status: draft v0.3">
  <img src="https://img.shields.io/badge/code-none_yet-6B7280?style=for-the-badge" alt="No code yet">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-product-6B7280?style=for-the-badge" alt="Back to product"></a>
</p>

---


**Status:** Draft v0.3 for review · **Date:** 2026-11-09 · **Code written:** none
**Sources:** a Claude run (Opus 5.5, 2026-11-07) and a DeepSeek run (2026-11-07), both answering one master
planning prompt built on an idea analysis. Merged by Claude (Fable 5.1). The two runs, the prompt and the idea
analysis stay in the private planning workspace; this copy is the one the roadmap works from.

Working name: **ASCC** (AI Security Command Center).

## What this package is

One planning package that supersedes both runs. The Claude run supplies the document structure, ID schemes,
ADRs and the detailed catalog; the DeepSeek run supplies the calls it made differently (listed in
[§ Reconciliation](#reconciliation-where-the-two-runs-disagreed)) and the items it had that Claude lacked. Where
the runs disagreed, this package records one decision and the reason; nobody has to read the originals.

Three requirements were added on 2026-11-09 and are new to both runs:

| # | Requirement | Where it lands | Epic |
|:--|:--|:--|:--|
| **N1** | **NHI connectors.** ASCC manages and tracks non-human identities (NHIs), so it needs connectors to cloud platforms and on-premise systems that pull NHI data into one inventory and push lifecycle actions (disable, rotate, revoke) back out | Charter §3 U13, [C §3.11](c-requirements-catalog.md#311-nhi-connectors-ep-21), D T-23/T-26, E §19, ADR-16, F §2.5, G, H, I, J | **EP-21** |
| **N2** | **Third-party plugin platform.** Developers outside the core team can extend ASCC with plugins; the platform, its SDK and its plugin library are a headline feature, built and advertised to attract contributors | Charter §3 U14 and §4, [C §3.12](c-requirements-catalog.md#312-plugin-platform-ep-22), D T-24/T-25, E §20, ADR-15, F §2.5 and §7.6, G, H, I, J | **EP-22** |
| **N3** | **Workflow intelligence.** Because ASCC sees every agent workflow across platforms, it detects redundant workflows and use cases solved more than once, and suggests combining or optimizing them | Charter §3 U15, [C §3.13](c-requirements-catalog.md#313-workflow-intelligence-ep-23), D T-27, E §21, ADR-17, F §2.5, H, I, J | **EP-23** |

A requirements review later the same day (2026-11-09, v0.3) checked the package against Mike's stated purpose,
*agent management within an enterprise, and governing non-human accounts including AI agents*, and added two more:

| # | Requirement | Where it lands | Epic |
|:--|:--|:--|:--|
| **N4** | **Agents are NHIs.** One non-human inventory with agents in it, an `agent_identity` kind, vendor-hosted agents discovered as `observed`, inventory documents imported and exported, hygiene rules mapped to the OWASP NHI Top 10, and containment that lists the credentials left standing | Charter §1 and U17, [C §3.11](c-requirements-catalog.md#311-nhi-connectors-ep-21) FR-CON-15 to 18, FR-KS-08, F §2.2 and §2.5 | EP-21 |
| **N5** | **Agent fleet management.** Groups and profiles as policy targets, a runtime posture check behind every `enforced` label, generated managed settings, a rule that the agent can't touch its own enforcement point, change control on approved agents, ownership transfer and expiry | Charter §1 and U16, C FR-REG-12 to 15, FR-POL-11, FR-RT-05 to 07, D T-28, F §2.2, I ABUSE-33/34, J TKT-041/042 | **EP-24** |

The public-MVP [spec](../spec.md) takes the offline part of both: posture, discovery, declared and imported
NHIs, groups, and three hardening rules the review found missing (self-protection, serialized audit appends, a
head anchor against truncation).

Everything else in both runs is carried forward; nothing was dropped silently. Items that existed only in the
DeepSeek run are tagged **[DS]** the first time they appear.

## Read in this order

| # | Document | What it answers |
|:--|:--|:--|
| A | [Product charter](a-product-charter.md) | Mission, personas, use cases, system boundaries, capability map, non-goals |
| B | [Discovery questions](b-discovery-questions.md) | 15 questions, each with a default the rest of the package assumes |
| C | [Requirements catalog](c-requirements-catalog.md) | 24 epics, stories, FR/NFR with acceptance criteria, priority, traceability |
| D | [Threat model](d-threat-model.md) | Assets, adversaries, trust boundaries, STRIDE, AI attack scenarios, mitigations |
| E | [Architecture package](e-architecture.md) | Planes, components, diagrams, enforcement comparison, connector and plugin runtime, tech options, failure modes |
| E2 | [Architecture decision records](e2-adrs.md) | 17 ADRs with rejected alternatives |
| F | [Data contracts](f-data-contracts.md) | Registry entities (incl. NHI, connector, plugin, workflow), token, event schema, policies, APIs, plugin contract, versioning |
| G | [MVP definition](g-mvp.md) | The vertical slice, demo script, measurable acceptance criteria |
| H | [Roadmap](h-roadmap.md) | Phases 0 to 4, effort ranges, staffing, milestones, exit criteria |
| I | [Validation plan](i-validation.md) | Security tests, abuse cases, red team, SLOs, failure injection, compliance evidence |
| J | [Backlog](j-backlog.md) | Epics and the first 42 engineering tickets |
| K | [Next steps and decisions](k-next-steps.md) | Prioritized checklist and the questions that block implementation |

## Labels used throughout

| Label | Meaning |
|:--|:--|
| **[C]** | Confirmed: stated in the master prompt, the idea analysis, or Mike's 2026-11-09 additions |
| **[A-n]** | Assumption: the default from [B](b-discovery-questions.md) question *n*, used until answered |
| **[R]** | Recommendation: this team's call, with reasoning |
| **[Q]** | Open question: needs a stakeholder decision ([K](k-next-steps.md) lists the blocking ones) |
| **[V]** | Verify: a protocol, standard, product or licence detail to check against the current official source before building on it |
| **[DS]** | Item that came from the DeepSeek run only |
| ⛔ **Prevent** | Control stops the action before it happens |
| 🔍 **Detect** | Control only observes and alerts; the action still happens |

ID schemes: requirements `FR-<AREA>-nn` / `NFR-nn`, epics `EP-nn`, user stories `US-nn`, threats `T-nn`, ADRs
`ADR-nn`, tickets `TKT-nnn`, abuse cases `ABUSE-nn`, failure tests `FAIL-nn`, MVP criteria `MVP-n`. Areas: `REG`
registry, `IAM` identity, `POL` policy, `GW` gateway, `RT` runtime adapter, `EG` egress, `SC` supply chain, `DLP`
data protection, `TEL` telemetry/audit, `KS` kill switch, `SEC` secrets, `TRS` trust scoring, `MAO` multi-agent
oversight, `INT` integrations, `ADM` admin/ops, **`CON` connectors and NHI**, **`PLG` plugin platform**, **`WFI`
workflow intelligence**.

Coverage classes are `enforced | observed | unmonitored` everywhere (the Claude run said *dark*, DeepSeek said
*unmonitored*; *unmonitored* needs no explanation to a buyer).

## The package in one paragraph

Build ASCC around **identity + policy + enforcement + evidence**, and make it **extensible by design**. One
policy decision point (Cedar, embedded in a Go service) answers every authorization question; several enforcement
points ask it: an MCP/LLM gateway, a local runtime adapter (CLI agent hooks plus a stdio MCP wrapper), a credential
broker that injects secrets so agents never hold them, and a default-deny egress proxy that turns bypass into a
blocked connection instead of a silent one. Every decision lands in a hash-chained audit log and an OpenTelemetry
trace. Delegation uses token exchange with an actor chain and an **intersection** rule, so authority can only
shrink down a chain. **Connectors**, built as plugins, pull non-human identities and AI assets from clouds,
identity providers, vaults, source hosts and on-premise directories into one inventory with cross-source
reconciliation, and push lifecycle actions back. The **plugin platform** (out-of-process, capability-scoped, scanned
and signed like any other asset) is how connectors, scanners, detectors, runtime adapters and integrations are
added, by the core team and by outside developers alike. **Workflow intelligence** reads the same registry and
telemetry to find workflows that solve the same job twice and proposes consolidation, as advice an owner accepts
or dismisses. The MVP proves the core story end to end and ships the plugin SDK with two real connectors built on
it, so the extension model is proven before the library opens.

## Reconciliation: where the two runs disagreed

Each row is a decision for this package. Changing one re-opens the documents in the last column.

| Topic | Claude run | DeepSeek run | Combined decision | Why | Moves |
|:--|:--|:--|:--|:--|:--|
| Workload identity | RFC 8693 token exchange + instance keys; SPIFFE P2 | SPIFFE/SPIRE primary | Token exchange and instance keys for laptops (MVP); **SPIFFE/SPIRE for server and Kubernetes agents from Phase 2** (pulled forward). Both bind to the same transaction token | Laptops can't run SPIRE agents sensibly; servers can, and should | C FR-IAM-13, E §5, ADR-06, H |
| Secrets backend | OpenBao bundled, Vault supported | Vault | OpenBao bundled; Vault, cloud secret managers and PAM via **connector plugins** | Vault's BUSL licence blocks redistribution **[V]**; connectors make the choice the customer's | ADR-07, C FR-INT-02, FR-CON-05 |
| MVP LLM providers | Anthropic Messages + one OpenAI-compatible | OpenAI-compatible hosted + local | **Both adapters**: Anthropic Messages and OpenAI-compatible (covers Ollama, vLLM, llama.cpp) | The lab fleet uses Anthropic; OpenAI-compatible covers local models and most others | G, B Q4 |
| MCP authorization | Noted as OAuth 2.1-based **[V]** | Gateway is an OAuth 2.1 resource server; RFC 9728 protected-resource metadata | Gateway is an **OAuth 2.1 resource server** to clients (RFC 9728 metadata **[V]**) and an OAuth client to upstream servers via the broker; no token passthrough | Matches the MCP authorization spec direction; keeps audience binding | E §5, F §7.3, ADR-06 |
| Gateway added latency | p95 ≤ 25 ms | median ≤ 5 ms | **median ≤ 5 ms, p95 ≤ 25 ms, p99 ≤ 50 ms**, non-streaming, no ML detectors | Both are useful: median for the common path, tail for budgets | NFR-02, I §5 |
| PDP latency | p99 ≤ 2 ms embedded | p95 ≤ 10 ms local, ≤ 50 ms networked | Embedded p99 ≤ 2 ms; **remote PDP p95 ≤ 10 ms** | Two deployment shapes, two targets | NFR-01 |
| Kill propagation | p95 ≤ 10 s; `unconfirmed` at 30 s | p99 ≤ 5 s | **p95 ≤ 5 s, p99 ≤ 10 s** to connected PEPs; `unconfirmed` at 30 s | Tighter than Claude, with a tail bound DeepSeek lacked | NFR-04, G MVP-7 |
| Audit retention | 400 days metadata, 30 days content | 13 months | **13 months** (≈ 400 days) metadata, 30 days content, legal hold suspends deletion | Same number, friendlier wording | B Q8, E §15 |
| Coverage labels | enforced / observed / dark | enforced / observed-only / unmonitored | **enforced / observed / unmonitored** | Self-explanatory to a buyer | Everywhere |
| Signing-key ownership | Audit checkpoint key owned by security team | Customer owns root keys in their HSM/KMS; ASCC holds none | **Customer-owned KMS/HSM for audit checkpoint and token signing keys in production**; ASCC-managed keys only in the dev stack | Removes ASCC admins from the trust chain of their own audit | E §15, ADR-10, FR-SEC-05 |
| MVP console | 6 screens; approvals via API form | Adds policy workbench, approval inbox, scan queue | Claude's 6 screens **plus a minimal approval inbox and scan queue**; policy workbench stays P1 | The scan gate needs an approve action in the UI to demo honestly | G §2, FR-ADM-01 |
| Fail-open classes | Three action tiers | Sensitive fail-closed; tagged observability paths fail open | **Tiered (1/2/3)**; DeepSeek's "observability may fail open, tagged" is tier 1 behaviour | Tiers subsume the binary rule | ADR-13, NFR-03 |
| A2A in MVP | Registry entries only | Registry + observe-only | **Registry + observe-only** in MVP; enforce in Phase 2 | Observing is cheap and gives coverage data | FR-GW-09, G |
| Availability | 99.9 % enforcement path in Phase 3 | 99.5 % control plane in MVP | **99.5 % control plane as an MVP target** (single node, best effort); 99.9 % enforcement path in Phase 3 with HA | One is a design target, the other an SLO | NFR-07 |
| Event ingest | Not stated | 1,000 events/s sustained | Adopt **1,000 events/s** sustained for MVP sizing | Gives NFR-09 a number | NFR-09, I §5 |
| Spec verification items | Generic **[V]** | Dated: OAuth 2.1 `draft-ietf-oauth-v2-1-16`, MCP revision 2026-07-28 transport auth, A2A v1.0 and the *A2ABreak* findings | Carry the dated items into the **[V]** list in K, still marked **[V]** | More specific checks are better checks | K §1, E §18 |
| Policy engine comparison | Qualitative | Weighted matrix (Cedar 0.78, OPA 0.80) | Keep both; **the Phase 0 spike decides** (ADR-03) | Scores are too close to decide on paper | ADR-03, E §16 |
| Config standard | Not stated | NFR: `.env` + `.env.example` house standard | Adopt as **NFR-16** | House rule | C §4 |
| Deployment options | Self-hosted; Compose → Kubernetes | Adds "managed control plane + customer-hosted PEPs" as a question | Add the option to **B Q1**; default stays self-hosted | Worth asking, not worth assuming | B Q1 |
| Abuse cases | 24 | 7 (T1 to T7) | Claude's 24, plus DeepSeek's T5 (agent prints a vault token), T6 (retrieval poisoning), T7 (agents approve each other) as ABUSE-25 to 27, plus new ones for connectors and plugins | Nothing lost | I §2 |
| Tickets | 30 (Phase 0 and 1) | 24 (Phase 1) | Claude's 30, plus DeepSeek's end-to-end acceptance suite and one-command demo runbook as tickets, plus new tickets for EP-21 to EP-23 (40 total) | Both missing pieces were real | J |
| Discovery questions | 15, six blocking | 15 | 15 merged: two low-ripple questions folded into others to make room for connector scope and plugin model | Cap is 15 | B |

> [!NOTE]
> Nothing here claims implemented capability, measured performance, integration support or compliance. Every
> latency figure is a target to be measured; every product, API and spec reference marked **[V]** must be
> checked before an ADR is accepted. Connector support for a named platform is a plan until its connector exists
> and its API scopes are confirmed against that platform's current documentation.

---

<p align="center">
  <a href="../README.md">← Product</a> ·
  <a href="../../README.md">Docs home</a> ·
  <a href="a-product-charter.md">Start: A. Product charter →</a>
</p>
