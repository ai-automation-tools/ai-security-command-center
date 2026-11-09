# K. Next steps and decisions

[Package index](README.md) · Previous: [J. Backlog](j-backlog.md)

## 1. Prioritized next-steps checklist

- [ ] **1. Answer the blocking questions** in §2 (or accept the defaults as written): B Q1 to Q6 and Q14 block
      Phase 1; Q13 blocks the connector tickets; Q15 can follow.
- [ ] **2. Treat this package as superseding both runs.** Review the
      [Reconciliation table](README.md#reconciliation-where-the-two-runs-disagreed) and object to any row; an
      objection becomes an ADR amendment, not a merge of prose.
- [ ] **3. Verify every [V] item** that an ADR or a Phase 1 ticket depends on:
  - MCP authorization and transport spec revision (the DeepSeek run cites a 2026-07-28 revision introducing
    transport-level auth [DS]) and the current stdio auth posture
  - RFC 9728 protected-resource metadata as used by MCP clients
  - OAuth 2.1 draft status (`draft-ietf-oauth-v2-1-16` per the DeepSeek run [DS]); no certification claims
  - A2A v1.0 Agent Card path, bindings and auth schemes, plus the *A2ABreak* findings [DS]
  - Claude Code hook contract (blocking PreToolUse, tool coverage, stdin/stdout format, version stability)
  - cedar-go parity with the Rust reference for the features in ADR-03
  - agentgateway and Envoy AI Gateway capabilities and licences (TKT-001)
  - HashiCorp Vault BUSL terms for redistribution; OpenBao API parity for KV v2, AppRole, transit and lease listing
  - OWASP Agentic Top 10 item names and numbering; OTel GenAI semantic conventions status
  - IETF transaction-tokens draft status and claim names
  - **GitHub REST/GraphQL coverage of Apps, installations and their permissions, deploy keys, organization PAT
    policy and Actions secret names, and the scopes each read needs** (TKT-037)
  - **Sigstore cosign for plugin signing and verification offline** (TKT-033)
  - **wazero / Extism maturity for a later WASM plugin host** (Phase 3 spike)
  - **Licence and offline availability of the candidate local embedding model for workflow intelligence**
    (Phase 3)
- [ ] **4. Run Phase 0 spikes** TKT-001 to TKT-004 **and TKT-031** together (about 4 weeks of focused work;
      TKT-031 also records the connector scope verification above).
- [ ] **5. Accept or amend ADR-01 to ADR-17** with spike results; ADR-15 may stay *Spike* until the first
      connector runs on the real host (M1.4).
- [ ] **6. Re-rate the threat model** (D) with the chosen runtimes, deployment and connector set, including
      T-23 to T-27.
- [ ] **7. Re-estimate Phase 1** from the J tickets by whoever will build them; pick team path or solo path (H).
- [ ] **8. Build the Compose skeleton** (TKT-006), the schemas (TKT-005) and the plugin contract draft
      (TKT-031); these are safe to start before ADRs land.
- [ ] **9. Decide where the code lives** (Q-K6) before the first commit; this folder is a non-git Draft. The plugin
      SDK and template repo need their own homes and licence headers from day one (K7).
- [ ] **10. Book the MVP exit review:** a criteria owner, a second person to run the demo script cold, and an
      outside developer to attempt the connector-from-template test (MVP-11).
- [ ] **11. Freeze the plugin contract at MVP exit** (`v1alpha1` → `v1beta1` or one more iteration), then apply
      the two-minor deprecation rule (NFR-19) to every later change.
- [ ] **12. Publish the "wanted connectors" list** (FR-PLG-13) before the library opens in Phase 2, ordered by
      the Phase 2 connector set in FR-CON-10, so outside developers pick up what the core team won't reach.

## 2. Decisions required before implementation

These block Phase 1 (K1 to K7) or the connector tickets (K8). Each has the default the package assumes.

| # | Decision | Default assumed | Who decides |
|:--|:--|:--|:--|
| **K1** | Product intent and deployment: private lab tool, open-source org project, or commercial product; self-hosted only, or a managed control plane later [DS] | Open-source core in the `ai-automation-tools` org (security pillar), dogfooded privately first, fully self-hosted; public release is a separate gate | Mike |
| **K2** | First protected environment and MVP runtime | Lab fleet; Claude Code on Windows as the enforced runtime; lab GitHub org and OpenBao as first connector targets | Mike |
| **K3** | Fail-closed tiers and break-glass rules (ADR-13) | Tier 1 cached ≤ 5 min; tiers 2/3 closed; break-glass for tier 2 only, two approvers, ≤ 4 h | Mike + security owner |
| **K4** | Authorization engine (ADR-03) | Cedar, subject to TKT-002 | Architecture, after spike |
| **K5** | Build vs adopt the gateway (ADR-04) | Build in Go unless TKT-001 shows an existing project passes 4 of 5 criteria | Architecture, after spike |
| **K6** | Repo home and visibility | New **private** org repo created at Phase 1 start with a noreply commit identity from the first commit; going public only after the release checklist and an explicit "make it public" | Mike |
| **K7** | **Plugin runtime model, SDK licence and library location** (ADR-15, B Q14) | Out-of-process gRPC plugins under a host with capability-scoped tokens, WASM later; SDK and contract Apache-2.0; library as a public index repo in the org plus the in-console browser; verification by automated pipeline plus core-team review for `verified` | Mike + architecture, after TKT-031 |
| **K8** | **NHI connector scope** (ADR-16, B Q13): every NHI the connector sees or AI-linked only; read-only in MVP; first platforms | Every NHI, `ai_linked` as an attribute; read-only in MVP with write scopes as separate approved credentials in Phase 2; OpenBao/Vault and GitHub first, then AWS IAM, Entra ID, GCP, Okta, Kubernetes, AD, CI secrets, n8n | Mike |
| **K9** | **Workflow-intelligence inputs and audience** (ADR-17, B Q15) | Metadata, registry fields and declared purpose text; prompts only where content capture is already on by policy; proposals go to every involved owner and the automation-owner role | Mike + DPO |
| **K10** | **Developer-programme incentives**: bounties, revenue share, partner badges, or none | None assumed; the programme ships with docs, templates, harness, listing and credits only | Mike |

Decisions that can wait until Phase 1 is underway: approver roles beyond the defaults (B Q9), A2A timing
(B Q10), inspection depth and sandbox fidelity (B Q11), team path (B Q12), content-capture and retention
numbers (B Q8), Phase 2 connector order within FR-CON-10, WFI overlap thresholds and the `stale` NHI window.

## 3. What this package does not claim

- No code exists; nothing here has been built, measured or penetration-tested.
- Latency, propagation, ingest, sync and throughput numbers are targets.
- Integration support for named products is a plan, not a fact; every one is subject to the [V] checks above.
- **Connector support for any named platform is a plan until its API scopes are verified against that
  platform's current documentation and the connector passes the conformance and fixture-replay suites.**
- **Plugin-library install counts, third-party plugin numbers and developer adoption are goals, not forecasts.**
- **Workflow-intelligence savings are estimates until calibrated against verified outcomes (FR-WFI-07).**
- Compliance mapping shows where evidence comes from, not that any framework is satisfied.
