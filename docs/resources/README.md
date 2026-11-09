<h1 align="center">🧰 Resources</h1>

<p align="center">
  <em>Tools, specs and sites worth reading or reusing before ASCC writes its own version.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/links_checked-2026--11--09-2ea44f?style=for-the-badge" alt="Links checked 2026-11-09">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

Each page groups resources by the part of ASCC they feed and says where in the
[combined plan](../product/plan/README.md) they land. GitHub links were checked for redirects, licence and
last push on 2026-11-09; several had moved, and the pages link the new home. Three projects turned out to be
archived and are flagged rather than recommended.

## 📂 By topic

| Page | What's in it |
|:---|:---|
| [**📜 Protocols and specs**](protocols-and-specs.md) | MCP (dated revision), A2A, OAuth token exchange, transaction tokens, SPIFFE, runtime hook contracts |
| [**🚪 Gateways and proxies**](gateways-and-proxies.md) | agentgateway, Envoy AI Gateway, ContextForge and other candidates for the TKT-001 build-vs-adopt test |
| [**🪪 Identity, secrets and NHI**](identity-secrets-and-nhi.md) | OpenBao, Vault, Keycloak, SPIRE, NHI references, and the platform APIs each connector will read |
| [**⚖️ Policy engines**](policy-engines.md) | Cedar, cedar-go, OPA, Cerbos, OpenFGA for the ADR-03 spike |
| [**🔎 Supply chain and scanners**](supply-chain-and-scanners.md) | SkillSpector, MCP Scanner, Snyk agent-scan, cosign, SBOM and secret scanners for the install gate |
| [**🛡️ Runtime guardrails and red team**](runtime-guardrails-and-red-team.md) | PII and injection detectors, plus garak, promptfoo and PentAGI to attack the result |
| [**📡 Telemetry, audit and detection**](telemetry-audit-and-detection.md) | OTel GenAI conventions, Trillian, eBPF sensors, SIEM targets, AiSOC |
| [**🧩 Plugins and connectors**](plugins-and-connectors.md) | go-plugin, Extism, wazero, and connector ecosystems (Steampipe, CloudQuery, Backstage) to copy from |
| [**📏 Standards and research**](standards-and-research.md) | OWASP agentic, LLM and NHI top 10s, MITRE ATLAS, NIST, ISO 42001, reading lists |

> [!TIP]
> Before you add a dependency or write your own version of something, check the page for that part of ASCC.
> Every entry says where in the plan it lands.

---

<p align="center">
  <a href="../README.md">← Docs home</a> ·
  <a href="../product/plan/README.md">Plan</a> ·
  <a href="protocols-and-specs.md">Start: Protocols and specs →</a>
</p>
