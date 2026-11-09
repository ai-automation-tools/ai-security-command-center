<h1 align="center">📏 Standards and Research</h1>

<p align="center">
  <em>Threat taxonomies, governance frameworks and reading lists the threat model and compliance mapping cite.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/feeds-D_threat_model_·_I_validation-8B5CF6?style=for-the-badge" alt="Feeds D and I">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

Compliance mapping in ASCC shows where evidence comes from, never that a framework is satisfied
([K §3](../product/plan/k-next-steps.md)). Cite these by name and version, and re-check numbering
before it goes into a requirement ID.

## 🧨 Threat taxonomies

| Resource | What it is | Where it lands |
|:---|:---|:---|
| [**OWASP Top 10 for Agentic Applications (2026)**](https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/) | The agent-specific top 10. Item names and numbering are **[V]** in K | Threat IDs in [D](../product/plan/d-threat-model.md) should cross-reference these |
| [**OWASP Agentic Security Initiative**](https://genai.owasp.org/initiatives/agentic-security-initiative/) | The working group behind the agentic list, with threat and mitigation guides | Mitigation wording for D |
| [**OWASP Top 10 for LLM Applications**](https://genai.owasp.org/llm-top-10/) · [**project repo**](https://github.com/OWASP/www-project-top-10-for-large-language-model-applications) | The LLM top 10 (prompt injection, excessive agency, …) | DLP and gateway abuse cases |
| [**OWASP AI Agent Security Cheat Sheet**](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html) | Practical controls checklist for agents | Quick gap check against C |
| [**OWASP NHI Top 10**](https://owasp.org/www-project-non-human-identities-top-10/) | Non-human identity risks | Hygiene findings in EP-21 |
| [**MITRE ATLAS**](https://atlas.mitre.org/) · [**data repo**](https://github.com/mitre-atlas/atlas-data) | ATT&CK-style matrix for AI systems, machine-readable | Tag detections and abuse cases with ATLAS technique IDs |
| [**CSA MAESTRO**](https://github.com/CloudSecurityAlliance/MAESTRO) | Cloud Security Alliance's layered threat-modeling method for agentic AI | Cross-check D's trust boundaries layer by layer |

## 🏛️ Governance frameworks

| Resource | What it is | Where it lands |
|:---|:---|:---|
| [**NIST AI RMF**](https://www.nist.gov/itl/ai-risk-management-framework) | US AI risk management framework, including the GenAI profile (NIST AI 600-1) | Evidence-pack mapping in Phase 2 to 3 |
| [**ISO/IEC 42001**](https://www.iso.org/standard/81230.html) | AI management system standard (paywalled; the site blocks scripted checks) | Evidence-pack mapping for enterprise buyers |
| [**EU AI Act**](https://artificialintelligenceact.eu/) | The regulation and its implementation timeline | Logging and human-oversight evidence |
| [**OWASP ASVS**](https://owasp.org/www-project-application-security-verification-standard/) | Application security verification standard | ASVS L2 evidence for ASCC itself in Phase 3 |
| [**Google SAIF**](https://saif.google/) | Google's Secure AI Framework | Control naming comparison |
| [**CoSAI**](https://www.coalitionforsecureai.org/) | Coalition for Secure AI; publishes agentic and MCP security papers | Read their MCP security workstream output |
| [**NCSC secure AI development guidelines**](https://www.ncsc.gov.uk/collection/guidelines-secure-ai-system-development) | UK/US joint guidance | Secure-by-default checklist for the release gate |
| [**AIUC-1**](https://www.aiuc-1.com/) | Certification standard for AI agents | A buyer may ask whether ASCC evidence supports it |

## 📚 Reading lists and incident data

| Resource | What it is |
|:---|:---|
| [**awesome-mcp-security**](https://github.com/Puliczek/awesome-mcp-security) | MCP attacks, papers and tools. Last updated March 2026 |
| [**awesome-ai-security**](https://github.com/ottosulin/awesome-ai-security) | Broad AI security list, kept current |
| [**Awesome-LLMSecOps**](https://github.com/wearetyomsmnv/Awesome-LLMSecOps) | LLM and agent security operations |
| [**awesome-llm-security**](https://github.com/corca-ai/awesome-llm-security) | Older LLM security list; quiet since Aug 2025 |
| [**AI Incident Database**](https://incidentdatabase.ai/) | Reported real-world AI incidents |
| [**AVID**](https://avidml.org/) | AI vulnerability database |
| [**Simon Willison on prompt injection**](https://simonwillison.net/tags/prompt-injection/) | The running record of injection attacks and the "lethal trifecta" framing |
| [**Embrace The Red**](https://embracethered.com/blog/) | Johann Rehberger's agent exploit write-ups; good source for abuse cases |
| [**Invariant Labs blog**](https://invariantlabs.ai/blog) | Original MCP tool-poisoning research |
| [**Anthropic engineering blog**](https://www.anthropic.com/engineering) | Posts on agent sandboxing and Claude Code security |

---

<p align="center">
  <a href="plugins-and-connectors.md">← Plugins and connectors</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="../README.md">Docs home →</a>
</p>
