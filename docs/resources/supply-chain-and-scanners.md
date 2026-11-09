<h1 align="center">🔎 Supply Chain and Scanners</h1>

<p align="center">
  <em>Scanners for MCP servers, skills and plugins, plus the signing and SBOM tools behind the install gate.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/epics-EP--10_·_EP--22-8B5CF6?style=for-the-badge" alt="Epics EP-10 and EP-22">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

The scan gate (U4) is a pipeline of these, wrapped as scanner plugins. The point is not to write a scanner;
it's to make the result change what the gateway allows.

## 🧩 Agent, MCP and skill scanners

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**SkillSpector**](https://github.com/NVIDIA/SkillSpector) | NVIDIA's scanner for Claude Code, Codex and MCP skills: injection, exfiltration, supply-chain risk. On Mike's list | Apache-2.0 | First scanner plugin for skills in the install gate |
| [**MCP Scanner**](https://github.com/cisco-ai-defense/mcp-scanner) | Cisco AI Defense's scanner for MCP servers and tools. On Mike's list | Apache-2.0 | Scanner plugin for MCP servers |
| [**Snyk agent-scan**](https://github.com/snyk/agent-scan) | Formerly Invariant Labs' `mcp-scan`; the old URL redirects here. Scans agents, MCP servers and skills | Apache-2.0 | Second opinion on MCP findings; tool-poisoning checks |
| [**Invariant guardrails**](https://github.com/invariantlabs-ai/invariant) | Rule language for agent traces | Apache-2.0 | Idea source for trace-based detection rules. Quiet since Jan 2026 |
| [**ModelScan**](https://github.com/protectai/modelscan) | Detects unsafe serialization in model files | Apache-2.0 | Scanner plugin for local models pulled into the fleet |

## 🔐 Signing, provenance and SBOM

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**Sigstore cosign**](https://github.com/sigstore/cosign) · [**sigstore.dev**](https://www.sigstore.dev/) | Signing and verification for containers and blobs | Apache-2.0 | Plugin signing; offline verification is **[V]** (TKT-033) |
| [**SLSA**](https://slsa.dev/) · [**repo**](https://github.com/slsa-framework/slsa) | Provenance levels for build pipelines | Community spec | Target level for first-party plugins and release images |
| [**in-toto**](https://github.com/in-toto/in-toto) | Signed supply-chain step attestations | Apache-2.0 | Attestation format for scan results (Phase 2) |
| [**Syft**](https://github.com/anchore/syft) · [**Grype**](https://github.com/anchore/grype) | SBOM generation and vulnerability matching | Apache-2.0 | SBOM and CVE stage of the scan pipeline |
| [**Trivy**](https://github.com/aquasecurity/trivy) | Vulns, misconfig, secrets and SBOM in one binary | Apache-2.0 | Alternative to Syft and Grype if one tool is easier to ship |
| [**OpenSSF Scorecard**](https://github.com/ossf/scorecard) | Health checks for open-source repos | Apache-2.0 | Input to plugin trust tiers |
| [**CycloneDX**](https://cyclonedx.org/) · [**SPDX**](https://spdx.dev/) | SBOM formats | Open standards | Pick one for F; CycloneDX also covers ML-BOM |

## 🕵️ Code and secret scanning

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**Semgrep**](https://github.com/semgrep/semgrep) | Pattern-based static analysis | LGPL-2.1 | Custom rules for MCP server source (shell exec, broad file access) |
| [**Gitleaks**](https://github.com/gitleaks/gitleaks) | Secret scanner for git history | MIT | Install gate check and the pre-public scan |
| [**TruffleHog**](https://github.com/trufflesecurity/trufflehog) | Finds and verifies leaked credentials | AGPL-3.0 | Verification of found keys; AGPL rules out bundling, run it as an external tool |
| [**Security Audit Skill**](https://github.com/cloudflare/security-audit-skill) | Cloudflare's multi-phase audit skill for coding agents with verified findings. On Mike's list | MIT | Audit ASCC's own code; model for machine-readable findings |
| [**Mantis**](https://github.com/google/mantis) | Google's toolkit for agents that find, reproduce and patch vulns. On Mike's list | Apache-2.0 | Build-time hardening of ASCC itself; reference for findings-to-fix loops |

---

<p align="center">
  <a href="policy-engines.md">← Policy engines</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="runtime-guardrails-and-red-team.md">Next: Runtime guardrails and red team →</a>
</p>
