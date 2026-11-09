# B. Discovery questions

[Package index](README.md) · Previous: [A. Charter](a-product-charter.md) · Next: [C. Requirements](c-requirements-catalog.md)

Fifteen questions, ranked by how much the answer changes the design. Each has a default; the rest of the
package proceeds on that default and cites it as **[A-n]**. Questions 1 to 6 and 14 block Phase 1 start; 13
blocks the connector tickets; the rest can be answered during Phase 1 ([K](k-next-steps.md)).

Merged from both runs. Two low-ripple questions from the Claude run (acceptable latency, now NFR-01/02 defaults)
and two from the DeepSeek run (SIEM target, now the FR-INT-03 default; anti-gaming, now ADR-14) were folded into
defaults to make room for the three new topics.

| # | Question | Recommended default | Why it matters / what changes if answered differently |
|:--|:--|:--|:--|
| **1** | **Who is this for, and how is it deployed?** Private lab tool, org open-source project, or commercial product; fully self-hosted, hybrid, or a managed control plane with customer-hosted PEPs [DS] | Open-source core under the `ai-automation-tools` org (security pillar), built and dogfooded on the lab's own agent fleet first; fully self-hosted. A managed control plane is a Phase 3+ question | Sets licence choices (BSL components are out for a redistributable core), audience for docs, whether multi-tenancy and enterprise IdP integrations are near-term, and whether the plugin library needs a hosted service |
| **2** | **What does the first deployment protect?** | The lab's own fleet: Claude Code and Codex CLI on Windows dev boxes, scheduled agent jobs, the Hetzner `agent-runner` container, locally configured MCP servers, the lab's GitHub org, OpenBao and n8n | Picks the MVP runtimes (G) and the first two connectors. An enterprise pilot instead would push Entra ID, AWS IAM and Kubernetes into Phase 1 |
| **3** | **Which agent runtimes must be *enforced* (not just observed) in MVP?** | One CLI agent via its pre-tool hook (Claude Code), plus any MCP client via the gateway or stdio wrapper | Each enforced runtime is an adapter to build and test; observed-only runtimes are cheap. From Phase 2, adapters are plugins |
| **4** | **Which LLM providers and MCP transports in MVP?** | Anthropic Messages API **and** an OpenAI-compatible endpoint (covers Ollama, vLLM, llama.cpp and most hosted providers); MCP Streamable HTTP and stdio | Gateway adapter count. SSE-only legacy MCP servers would add a third transport **[V]** |
| **5** | **Fail-closed or fail-open when the PDP or gateway is down?** | Tiered: fail-closed for tiers 2 and 3 (write, credential, external send); cached-decision grace of 5 min for tier 1 reads, marked `degraded`; break-glass documented | Drives HA investment. Fail-closed everywhere means the gateway must be HA in MVP |
| **6** | **Authorization engine: Cedar or OPA/Rego?** | Cedar embedded in the PDP, subject to a Phase 0 spike on Go library parity; the DeepSeek weighted matrix scored them 0.78 vs 0.80, too close to decide on paper | Policy authoring UX, analysis tooling, Envoy integration path ([ADR-03](e2-adrs.md#adr-03)) |
| 7 | Which IdP and secrets manager for the first real deployment? | Keycloak (dev) + Entra ID or Okta (first enterprise); OpenBao default, Vault and cloud secret managers via connector plugins | Connector order; Vault's BSL licence matters if ASCC is redistributed with it **[V]** |
| 8 | Content capture, retention, residency and legal hold? | Metadata only by default; content capture per-policy, redacted, encrypted with a separate key, 30-day retention; metadata 13 months hot-searchable; jurisdiction-configurable; legal hold suspends deletion per case | Storage size, privacy review, who can read content, deletion jobs |
| 9 | Who may approve high-risk grants, plugin installs and NHI lifecycle actions, and who may pull the kill switch? | Grants and plugin installs: resource owner + security (two people, neither the requester). NHI disable/rotate/revoke: NHI owner + IAM engineer. Kill: any on-call security operator for a single agent or plugin; two operators for fleet-wide | SoD rules and the operator role model |
| 10 | Is A2A traffic in scope before Phase 2? | No: register A2A endpoints and observe traffic in MVP, enforce in Phase 2 [DS] | Saves one adapter in MVP. Answer "yes" if the first deployment is A2A-heavy |
| 11 | How deep may ASCC inspect: TLS interception at egress, and dynamic sandbox fidelity for scans? | No blanket TLS interception (allowlist by SNI/host only); static scanning in MVP; gVisor sandbox in Phase 2 with egress capture and no outbound access except a recorded allowlist | TLS interception adds a CA to every client and a large privacy and breakage cost; sandbox fidelity sets infra cost and the risk of detonating malicious code |
| 12 | Team size and timeline? | Solo builder + AI agents for Phase 0 and a narrowed Phase 1; 4 to 6 engineers for the full Phase 1 in [H](h-roadmap.md) | Every estimate in H. The solo path cuts scope, not quality bars |
| **13** | **NHI connector scope.** Which platforms first; does the inventory hold *every* NHI a connector can see or only AI-linked ones; are connectors read-only or may they change state? | First: OpenBao/Vault and GitHub (MVP, read-only), then AWS IAM, Entra ID, GCP, Okta, Kubernetes, Active Directory, CI secrets, n8n (Phase 2). The inventory holds **every** NHI the connector can see, with AI linkage as an attribute, because an unlinked NHI is a candidate for shadow AI and for retirement. Read-only by default; write scopes are a separate, approved connector credential | Decides the connector credential model, the size of the inventory, and whether ASCC is "an AI tool that lists some accounts" or "the NHI inventory". Limiting to AI-linked NHIs halves the data and most of the value |
| **14** | **Plugin platform model.** Runtime isolation (out-of-process gRPC, in-process WASM, or both); SDK licence; where the library lives; who verifies plugins; what incentives, if any, the developer programme offers | Out-of-process gRPC plugins under a plugin host with capability-scoped tokens; WASM later for hot-path detectors; SDK and contract Apache-2.0; library as a public index repo in the org plus the in-console browser; verification by automated scan plus core-team review for the *verified* tier; **incentives are Mike's decision [Q]**, none assumed | The runtime model shapes the connector architecture in MVP; the licence and library location shape the developer programme and whether the plugin SDK can go public before the core does |
| **15** | **Workflow intelligence inputs and audience.** May it use purpose text and prompts, or metadata only; who receives consolidation proposals; which workflow platforms are connected first? | Metadata, registry fields and declared purpose text by default; prompt content only where content capture is already enabled by policy; proposals go to the owners of every workflow involved plus the automation owner persona; n8n first (lab fleet), then GitHub Actions, then Power Automate / Make / Zapier / Airflow as connectors arrive | Privacy posture of the feature, and whether semantic similarity (Phase 3) has anything to work with |

## How the defaults ripple

| If you change | These documents move |
|:--|:--|
| Q1 to "commercial product" or "managed control plane" | A §4 positioning, E2 licence column in ADRs, H staffing, multi-tenancy pulled to Phase 2, a hosted plugin-library service appears in E §20 |
| Q2 to "enterprise pilot" | G runtimes, IdP and connectors; H Phase 1 adds Kubernetes, Entra ID and AWS IAM; I interop tests |
| Q5 to "fail-open" | NFR-03, E §9 failure modes, I failure-injection expected results |
| Q6 to OPA | F §5 sample policies, ADR-03, TKT-002, TKT-011, policy workbench design, policy-pack plugin type |
| Q8 to "full content" | D privacy threats, F event schema `content_ref`, NFR-09 storage sizing, Q15 inputs |
| Q13 to "AI-linked NHIs only" | FR-CON-03/06, F §2.5 `ai_linked` becomes a filter not an attribute, A §9 NHI measures, H Phase 2 connector effort shrinks |
| Q13 to "write from day one" | FR-CON-08 moves to P0, D T-23 severity rises, G adds a lifecycle-action demo step, I ABUSE-28 moves to MVP |
| Q14 to "in-process plugins" | ADR-15 rewritten, D T-24/T-25 severity rises, NFR-17 isolation guarantees weaken, E §20 host design |
| Q15 to "metadata only, no purpose text" | FR-WFI-04 (semantic similarity) drops to R, E §21 inputs table |
