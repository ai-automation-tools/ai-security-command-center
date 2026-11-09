<h1 align="center">📜 Protocols and Specs</h1>

<p align="center">
  <em>The specs ASCC has to speak, enforce or verify against: MCP, A2A, OAuth, workload identity and runtime hooks.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/links_checked-2026--11--09-2ea44f?style=for-the-badge" alt="Links checked 2026-11-09">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

Most of these are behind a **[V]** tag somewhere in the [combined package](../product/plan/README.md). The
[K checklist](../product/plan/k-next-steps.md) lists which ones block an ADR. Read the dated revision, not
whatever the landing page shows that week: MCP and A2A both moved under us during planning.

## 🔌 Agent protocols

| Resource | What it is | Where ASCC uses it |
|:---|:---|:---|
| [**MCP specification (2026-07-28)**](https://modelcontextprotocol.io/specification/2026-07-28) | The current dated MCP revision. The unversioned spec URL now redirects here | Gateway message handling (E §5), the stdio wrapper, TKT-004. Pin this revision for Phase 1 |
| [**MCP authorization**](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) | How an MCP server acts as an OAuth resource server, including protected-resource metadata | The gateway as OAuth 2.1 resource server; no token passthrough (ADR-06, F §7.3) |
| [**MCP security best practices**](https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices) | The spec team's own list: confused deputy, token passthrough, session hijack, SSRF | Seed list for abuse cases in [I §2](../product/plan/i-validation.md) |
| [**MCP spec repo**](https://github.com/modelcontextprotocol/modelcontextprotocol) | Source, schema and open proposals for the spec | Watch for transport and auth changes during Phase 1 |
| [**MCP Registry**](https://github.com/modelcontextprotocol/registry) | Community registry service for MCP servers | A connector source for shadow-MCP discovery and catalog metadata (EP-21) |
| [**MCP Inspector**](https://github.com/modelcontextprotocol/inspector) | Test client for MCP servers | Fixture capture and gateway conformance tests |
| [**A2A protocol site**](https://a2a-protocol.org/) · [**A2A repo**](https://github.com/a2aproject/A2A) | Agent2Agent spec, Agent Cards and SDKs (Apache-2.0) | Registry entries and observe-only in MVP, enforce in Phase 2 (FR-GW-09) |

## 🪪 Identity and authorization standards

| Resource | What it is | Where ASCC uses it |
|:---|:---|:---|
| [**RFC 8693: OAuth token exchange**](https://www.rfc-editor.org/rfc/rfc8693) | Swap one token for another with an `act` (actor) chain | The STS and the delegation intersection rule (ADR-06) |
| [**RFC 9728: protected resource metadata**](https://www.rfc-editor.org/rfc/rfc9728) | How a resource server tells clients which authorization server to use | Gateway metadata endpoint for MCP clients |
| [**RFC 9449: DPoP**](https://www.rfc-editor.org/rfc/rfc9449) | Sender-constrained tokens bound to a client key | Candidate for proof-of-possession tokens (TKT-003) |
| [**OAuth 2.1 draft**](https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/) | The consolidated OAuth draft MCP builds on. Still a draft | Check the revision before claiming anything about compliance |
| [**Transaction Tokens draft**](https://datatracker.ietf.org/doc/draft-ietf-oauth-transaction-tokens/) | Short-lived tokens that carry the caller context through a call chain | Shape of the ASCC transaction token (F §3); claim names are **[V]** |
| [**WIMSE architecture draft**](https://datatracker.ietf.org/doc/draft-ietf-wimse-arch/) | IETF workload identity in multi-system environments | Cross-check for the SPIFFE plan in Phase 2 |
| [**SPIFFE**](https://spiffe.io/) · [**spec repo**](https://github.com/spiffe/spiffe) | Workload identity standard (SVIDs, trust domains) | Server and Kubernetes agent identity from Phase 2 |
| [**OpenID IPSIE working group**](https://openid.net/wg/ipsie/) | Interoperability profile for enterprise identity, including session and lifecycle | Watch for NHI and agent lifecycle profiles that affect connectors |

## 🪝 Runtime hook contracts

| Resource | What it is | Where ASCC uses it |
|:---|:---|:---|
| [**Claude Code hooks**](https://code.claude.com/docs/en/hooks) | PreToolUse and friends: blocking decisions, stdin/stdout JSON | The MVP runtime adapter. TKT-004 checks blocking coverage per tool |
| [**Codex hooks**](https://developers.openai.com/codex/hooks) | Hook support in the Codex CLI (the link now redirects to `learn.chatgpt.com`) | The Phase 2 second runtime adapter; verify blocking semantics first |

---

<p align="center">
  <a href="README.md">← Resources home</a> ·
  <a href="gateways-and-proxies.md">Next: Gateways and proxies →</a>
</p>
