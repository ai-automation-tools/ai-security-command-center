# K. Next steps and decisions

[Package index](README.md) · Previous: [J. Backlog](j-backlog.md)

## 1. Prioritized next-steps checklist

- [ ] **1. Answer the blocking questions** in §2 (or accept the defaults as written): B Q1 to Q6 and Q14 block
      Phase 1; Q13 blocks the connector tickets; Q15 can follow.
- [ ] **2. Treat this package as superseding both runs.** Review the
      [Reconciliation table](README.md#reconciliation-where-the-two-runs-disagreed) and object to any row; an
      objection becomes an ADR amendment, not a merge of prose.
- [x] **3. Verify every [V] item** that an ADR or a Phase 1 ticket depends on. Checked 2026-11-10; results in
      [§1.1](#11-v-check-results-2026-11-10). The list as it stood before the check:
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
- [x] **4. Run Phase 0 spikes** TKT-001 to TKT-004 **and TKT-031** together (about 4 weeks of focused work;
      TKT-031 also records the connector scope verification above). Done 2026-11-10, each as a report:
      [gateway build or adopt](../../reports/2026-gateway-build-vs-adopt.md) (TKT-001, a source read, not a run),
      [cedar-go check](../../reports/2026-cedar-go-check.md) (TKT-002, cut down),
      [proof of possession](../../reports/2026-proof-of-possession.md) (TKT-003),
      [Claude Code hook contract](../../reports/2026-claude-code-hook-contract.md) (TKT-004, Claude Code only) and
      [plugin contract spike](../../reports/2026-plugin-contract-spike.md) (TKT-031).
- [-] **5. Accept or amend ADR-01 to ADR-17** with spike results; ADR-15 may stay *Spike* until the first
      connector runs on the real host (M1.4). Amended 2026-11-10, **not yet accepted**: nine carry an amendment and
      every one has its evidence and what it waits for in the [status table](e2-adrs.md). Two are Accepted
      (ADR-12 and ADR-14, which the public MVP runs on); accepting the other fifteen is the owner's. ADR-04 waits on a run of agentgateway
      and on K5: the spike found it meets the adopt rule, so "build in Go" is no longer the safe default.
- [x] **6. Re-rate the threat model** (D) with the chosen runtimes, deployment and connector set, including
      T-23 to T-27. Done 2026-11-10: [D §4.1](d-threat-model.md#41-re-rating-2026-11-10). T-03 goes up; T-23 and
      T-24 come down for Phase 1 only.
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

### 1.1 [V] check results (2026-11-10)

Each item was read from its own source on 2026-11-10: the spec site, the IETF datatracker, the project's
repository or the vendor's documentation. The two exceptions are the in-repo reports, checked 2026-11-09. **Confirmed** means the plan's claim holds. **Changed** means the
source says something else; the plan text that carries the old claim is named, and is left as written until
the spike that owns it amends the ADR. Version numbers are the latest release on the day and will move.

**Protocols and standards**

| Item | Result | What the source says | Source |
|:--|:--|:--|:--|
| **MCP revision 2026-07-28** | **Changed** | The revision exists and is current, but it did not "introduce transport-level auth" (E §18 R4, README reconciliation table). Its major changes: protocol sessions and the `Mcp-Session-Id` header removed, the `initialize` handshake removed (every request carries its version and capabilities), `server/discover` added, and server-initiated requests (`sampling`, `elicitation`, `roots/list`) replaced by multi round-trip requests the client answers on a retry. Roots, Sampling and Logging are deprecated. Authorization changes are minor: RFC 9207 `iss` validation, credentials bound to their issuer, Dynamic Client Registration deprecated for Client ID Metadata Documents. **This reaches TKT-016:** "session handling" has nothing to handle at this revision, and "server→client requests blocked" becomes a rule on `input_required` results | [changelog](https://modelcontextprotocol.io/specification/2026-07-28/changelog) |
| **MCP stdio auth posture** | Confirmed | stdio implementations "SHOULD NOT follow this specification, and instead retrieve credentials from the environment". Authorization is optional overall | [authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) |
| **MCP token passthrough** | Confirmed | "The MCP server MUST NOT pass through the token it received from the MCP client" (ADR-06) | [security considerations](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations) |
| **RFC 9728 in MCP** | Confirmed | Published RFC (April 2025). MCP servers MUST implement protected-resource metadata and clients MUST use it to find the authorization server; a protected MCP server "acts as an OAuth 2.1 resource server" (ADR-06, TKT-016) | [RFC 9728](https://datatracker.ietf.org/doc/rfc9728/), [authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) |
| **OAuth 2.1 draft** | **Changed** | Latest is `draft-ietf-oauth-v2-1-17`, dated 2026-10-31, not `-16` (ADR-06, E §18 R3). Still an Internet-Draft, not an RFC; no certification exists. The MCP spec itself cites `-13` | [datatracker](https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/) |
| **A2A v1.0** | Confirmed | v1.0.0 released 2026-03-12, latest v1.0.1. Agent Card at `/.well-known/agent-card.json`. Bindings: JSON-RPC, gRPC, HTTP+JSON. Schemes: API key, HTTP auth, OAuth2, OpenID Connect, mutual TLS; cards may be JWS-signed | [specification](https://a2a-protocol.org/latest/specification/), [releases](https://github.com/a2aproject/A2A/releases) |
| ***A2ABreak*** | Confirmed, with a caveat | The paper exists: arXiv 2609.10871, submitted 2026-09-09, reporting 11 vulnerabilities open to a specification-compliant adversary. It is a preprint with no review venue found, and its findings were not read against ADRs here; do that before the A2A work in Phase 2 | [arXiv](https://arxiv.org/abs/2609.10871) |
| **OWASP Agentic Top 10** | Confirmed | *Top 10 for Agentic Applications 2026*, published 2025-12-09: ASI01 Agent Goal Hijack, ASI02 Tool Misuse, ASI03 Identity & Privilege Abuse, ASI04 Agentic Supply Chain Vulnerabilities, ASI05 Unexpected Code Execution, ASI06 Memory & Context Poisoning, ASI07 Insecure Inter-Agent Communication, ASI08 Cascading Failures, ASI09 Human-Agent Trust Exploitation, ASI10 Rogue Agents. Names taken from OWASP's announcement, not the PDF | [OWASP](https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/) |
| **OTel GenAI conventions** | Confirmed | Status is **Development**, not stable, and they moved to their own repository. Agent and tool spans are defined (create agent, invoke agent, execute tool). Expect attribute names to change | [semconv](https://opentelemetry.io/docs/specs/semconv/gen-ai/), [repository](https://github.com/open-telemetry/semantic-conventions-genai) |
| **Transaction tokens** | **Changed** | `draft-ietf-oauth-transaction-tokens-11`, dated 2026-07-30, past working-group consensus, not an RFC. Claims: `txn`, `sub`, `scope`, `req_wl`, `aud`, `iat`, `exp` required; `tctx` and `rctx` recommended. Earlier drafts' `azd` is now `tctx` and the purpose claim is now `scope`. ADR-06's token has `purpose`, `task_id` and `depth`, and no `txn`, `req_wl`, `tctx` or `rctx`; it says to align "if it stabilizes", and the draft is close, so TKT-003, which already ends by updating ADR-06, takes the naming question with it | [datatracker](https://datatracker.ietf.org/doc/draft-ietf-oauth-transaction-tokens/) |
| **Claude Code hook contract** | Confirmed | Checked 2026-11-09 and against a live Claude Code 2.1.321 session. Fails open on a crash, timeout or bad JSON unless registered with `"onFailure": "block"` | [report](../../reports/2026-claude-code-hook-contract.md) |
| **cedar-go parity** | Confirmed | cedar-go v1.9.0 matched the Rust reference on 9 of 9 requests and on schema validation. A `forbid` that errors fails open in both engines | [report](../../reports/2026-cedar-go-check.md) |

**Products and licences**

| Item | Result | What the source says | Source |
|:--|:--|:--|:--|
| **agentgateway** | Confirmed | A Linux Foundation project, Apache-2.0, v1.7.0 (2026-10-27). Proxies MCP and A2A with RBAC in CEL; no Cedar | [repository](https://github.com/agentgateway/agentgateway) |
| **Envoy AI Gateway** | **Changed** | Renamed: "Envoy AI Gateway is now Agent Router, an Agentic AI Foundation project", at `theagentrouter/agent-router`. Apache-2.0, v1.2.1 (2026-11-06). It does MCP now (`MCPRoute`, OAuth, tool filtering, CEL rules on JWT claims). TKT-001 and ADR-04 should use the new name | [repository](https://github.com/envoyproxy/ai-gateway), [MCP docs](https://theagentrouter.ai/docs/capabilities/mcp/) |
| **ContextForge** | Confirmed | IBM `mcp-context-forge`, Apache-2.0, v1.1.1. A registry and proxy federating MCP, A2A and REST/gRPC | [repository](https://github.com/IBM/mcp-context-forge) |
| **LiteLLM** | Confirmed | MIT except `enterprise/`. Audit logs, RBAC, SSO past five users, JWT auth and per-key guardrails need the enterprise licence (ADR-04) | [LICENSE](https://github.com/BerriAI/litellm/blob/main/LICENSE), [enterprise](https://docs.litellm.ai/docs/enterprise) |
| **Kong AI Gateway** | Partly confirmed | AI Proxy Advanced and AI Semantic Prompt Guard are enterprise-only; basic AI Proxy shows no tier. Three plugin pages read, not the whole list | [AI Proxy Advanced](https://developer.konghq.com/plugins/ai-proxy-advanced/) |
| **Vault licence** | **Changed** | BUSL 1.1, licensor now IBM, converting to MPL 2.0 after four years. It bars offering Vault to third parties on a hosted or embedded basis that competes with the paid product; it does not bar all redistribution. "Blocks redistribution" (README reconciliation table) overstates it; "needs legal review" (ADR-07) stands, and the OpenBao default with it | [LICENSE](https://github.com/hashicorp/vault/blob/main/LICENSE) |
| **OpenBao API parity** | Confirmed | MPL 2.0, v2.7.2 (2026-10-26). KV v2, AppRole, transit and lease lookup (`sys/leases/lookup`, list needs `sudo`) are all in its API docs (ADR-07, TKT-036). Read from the docs, not run against both servers | [leases](https://openbao.org/docs/api/system/leases/), [KV v2](https://openbao.org/docs/api/secret/kv/kv-v2/), [AppRole](https://openbao.org/docs/api/auth/approle/), [transit](https://openbao.org/docs/api/secret/transit/) |
| **Smokescreen** | **Changed** | MIT. It does per-client ACLs, by client certificate name with a replaceable role function, so "weaker per-identity policy" (ADR-08) is wrong as written. Its limit is that policy is a host allowlist only | [repository](https://github.com/stripe/smokescreen) |
| **Semgrep** | Confirmed, stricter than written | Engine is LGPL-2.1. The `semgrep-rules` licence allows internal use only and forbids redistributing the rules or offering them as a service, so ADR-11's "write our own rules" isn't optional if rules ship with ASCC | [engine](https://github.com/semgrep/semgrep/blob/develop/LICENSE), [rules licence](https://semgrep.dev/legal/rules-license) |
| **go-plugin** | Confirmed | MPL 2.0, v1.8.0 (2026-04-29), commits in November 2026 (ADR-15) | [repository](https://github.com/hashicorp/go-plugin) |
| **Cedar analysis tooling** | Confirmed | `cedar-policy-symcc` compiles policies to SMT and checks always-allow, always-deny, equivalence and implication (needs cvc5); `cedar-lean-cli` has `analyze`. Both Apache-2.0, both Rust: neither runs in the Go binary (ADR-03) | [symcc](https://github.com/cedar-policy/cedar/tree/main/cedar-policy-symcc), [cedar-spec](https://github.com/cedar-policy/cedar-spec) |
| **wazero / Extism** | Confirmed | wazero: Apache-2.0, v1.12.0, stable API since 1.0, no CGO, now at `wazero/wazero`. Extism: BSD-3-Clause, v1.41.0; its Go SDK runs on wazero but was last tagged 2025-03-19 | [wazero](https://github.com/wazero/wazero), [Extism Go SDK](https://github.com/extism/go-sdk) |
| **Local embedding model** | Confirmed | `all-MiniLM-L6-v2` is Apache-2.0 and `bge-small-en-v1.5` is MIT; both run from a local cache with `HF_HUB_OFFLINE` set. `nomic-embed-text-v1.5` is Apache-2.0 but runs repository code on older library versions; prefer the first two | [MiniLM](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2), [bge-small](https://huggingface.co/BAAI/bge-small-en-v1.5) |

**Connector and plugin facts**

| Item | Result | What the source says | Source |
|:--|:--|:--|:--|
| **GitHub: app installations** | Confirmed | `GET /orgs/{org}/installations` returns each installation's `permissions`. Needs organization "Administration" read (classic: `admin:read`, as an org owner) | [orgs](https://docs.github.com/en/rest/orgs/orgs) |
| **GitHub: deploy keys** | Confirmed | `GET /repos/{owner}/{repo}/keys`. Needs repository "Administration" read. Also in GraphQL (`Repository.deployKeys`) | [deploy keys](https://docs.github.com/en/rest/deploy-keys/deploy-keys) |
| **GitHub: organization PATs** | **Changed** | The token and token-request lists can be called **only by a GitHub App** (organization "Personal access tokens" read), never with a PAT. So the connector in TKT-037 must authenticate as an App | [personal access tokens](https://docs.github.com/en/rest/orgs/personal-access-tokens) |
| **GitHub: PAT policy setting** | **Changed** | No read endpoint was found for the organization's PAT *policy*; only the tokens and requests are listable. Plan the connector to report tokens, and the policy as not readable, unless TKT-037 finds one | [personal access tokens](https://docs.github.com/en/rest/orgs/personal-access-tokens) |
| **GitHub: Actions secret names** | Confirmed | Organization and repository lists return names and dates, never values. Need "Secrets" read (classic: `admin:org`, `repo`) | [secrets](https://docs.github.com/en/rest/actions/secrets) |
| **GitHub: REST or GraphQL** | Not fully verified | Installations, secrets and PATs look REST-only; the GraphQL reference pages were too long to read whole. Assume REST | [GraphQL orgs](https://docs.github.com/en/graphql/reference/orgs) |
| **cosign offline** | **Changed** | cosign v3.1.4, Apache-2.0. There is no `--offline` flag: offline verification is `verify-blob --bundle <file> --trusted-root <file>` (keyless) or `--bundle <file> --key <file>`. No page promises zero network calls, so TKT-033 must test it with the network blocked | [verify](https://docs.sigstore.dev/cosign/verifying/verify/), [verify-blob](https://github.com/sigstore/cosign/blob/main/doc/cosign_verify-blob.md) |

**Not checked,** because no ADR decision or Phase 1 ticket rests on them: the remaining ADR-08 alternatives
(Squid and the others), external transparency-log anchoring (ADR-10, Phase 3), A2A spec churn as a Phase 2 risk,
hook contracts for runtimes other than Claude Code (TKT-004; out of scope for this version), and the API scopes
of the Phase 2 connector platforms (FR-CON-10), each of which is verified when its connector is built.

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
