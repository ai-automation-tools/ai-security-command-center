<h1 align="center">🚪 Gateways and Proxies</h1>

<p align="center">
  <em>Open-source MCP, A2A and LLM gateways to adopt, borrow from, or beat in the build-vs-adopt spike.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/decides-ADR--04_·_TKT--001-8B5CF6?style=for-the-badge" alt="Decides ADR-04 and TKT-001">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

ADR-04 defaults to building the gateway in Go unless an existing project passes four of the five TKT-001
criteria. These are the candidates for that test. Licence and activity were read from GitHub on 2026-11-09.

## 🧪 Build-vs-adopt candidates

| Project | What it is | Licence | Why it matters to ASCC |
|:---|:---|:---|:---|
| [**agentgateway**](https://github.com/agentgateway/agentgateway) | Rust proxy for MCP and A2A with authn, RBAC and telemetry. On Mike's original list | Apache-2.0 | The closest match to the ASCC gateway. First thing TKT-001 tests |
| [**Envoy AI Gateway**](https://github.com/theagentrouter/agent-router) | LLM and MCP routing on Envoy Gateway. The `envoyproxy/ai-gateway` URL now redirects to `theagentrouter/agent-router` | Apache-2.0 | If ASCC ends up on Envoy anyway (egress, ext_authz), this is the path of least code. Check the rename before citing it |
| [**IBM ContextForge**](https://github.com/IBM/mcp-context-forge) | Gateway and registry in front of MCP, A2A and REST, with guardrails and plugins | Apache-2.0 | Has a plugin model and a registry; worth reading before writing EP-22 |
| [**MCP Gateway & Registry**](https://github.com/agentic-community/mcp-gateway-registry) | MCP gateway plus registry with Keycloak or Entra OAuth | Apache-2.0 | Same shape as the ASCC MVP slice; useful for UX and OAuth flow ideas |
| [**Docker MCP Gateway**](https://github.com/docker/mcp-gateway) | Docker's MCP gateway CLI plugin; runs servers in containers | MIT | Reference for running MCP servers isolated; possible sandbox backend |
| [**Microsoft MCP Gateway**](https://github.com/microsoft/mcp-gateway) | Session-aware reverse proxy and lifecycle manager for MCP on Kubernetes | MIT | Phase 3 Kubernetes routing and session affinity |
| [**Lasso MCP Gateway**](https://github.com/lasso-security/mcp-gateway) | Plugin-based MCP gateway with security plugins | MIT | Small codebase; quick read on plugin hooks. Last push Jan 2026 |

## 🧭 LLM gateways (the "proxy" half)

| Project | What it is | Licence | Why it matters to ASCC |
|:---|:---|:---|:---|
| [**LiteLLM**](https://github.com/BerriAI/litellm) | Proxy over 100+ LLM APIs with cost tracking, guardrails and logging | Mixed (enterprise dir is separate) | The cost and routing proxy most teams already run. ASCC should sit beside it or in front, not compete |
| [**Portkey Gateway**](https://github.com/Portkey-AI/gateway) | LLM router with built-in guardrails | MIT | Guardrail hook design to compare with FR-DLP |
| [**Kong**](https://github.com/Kong/kong) | API gateway with AI plugins | Apache-2.0 | Enterprises with Kong will ask for a plugin, not a new proxy |
| [**Envoy**](https://github.com/envoyproxy/envoy) | The proxy under several of the above | Apache-2.0 | Default-deny egress proxy and `ext_authz` to the PDP (EP-08) |

> [!TIP]
> Score each candidate against the five TKT-001 criteria in the same table, with the commit hash you tested.
> A row without a hash is an opinion.

---

<p align="center">
  <a href="protocols-and-specs.md">← Protocols and specs</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="identity-secrets-and-nhi.md">Next: Identity, secrets and NHI →</a>
</p>
