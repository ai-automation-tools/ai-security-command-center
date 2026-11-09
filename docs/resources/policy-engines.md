<h1 align="center">⚖️ Policy Engines</h1>

<p align="center">
  <em>The engines that could sit behind the one policy decision point, and the spike that picks between them.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/decides-ADR--03_·_TKT--002-8B5CF6?style=for-the-badge" alt="Decides ADR-03 and TKT-002">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

The package defaults to Cedar embedded in Go. DeepSeek's weighted matrix had OPA ahead by 0.02, which is
noise, so the Phase 0 spike decides ([E2 ADR-03](../product/plan/e2-adrs.md)).

| Engine | What it is | Licence | Fit for ASCC |
|:---|:---|:---|:---|
| [**Cedar**](https://github.com/cedar-policy/cedar) · [**cedarpolicy.com**](https://www.cedarpolicy.com/) | AWS's policy language with a Rust reference engine and formal analysis | Apache-2.0 | Default. Analysable policies, readable deny reasons |
| [**cedar-go**](https://github.com/cedar-policy/cedar-go) | Go port of Cedar | Apache-2.0 | What makes "embedded in Go" possible. Feature parity with Rust is the TKT-002 question |
| [**Open Policy Agent**](https://github.com/open-policy-agent/opa) · [**docs**](https://www.openpolicyagent.org/) | General policy engine with Rego; Envoy `ext_authz` support | Apache-2.0 | Fallback if cedar-go falls short; strongest ecosystem |
| [**Cerbos**](https://github.com/cerbos/cerbos) | Authorization service that now pitches agent and gateway use | Apache-2.0 | Reference for policy testing and audit output; possible competitor |
| [**OpenFGA**](https://github.com/openfga/openfga) | Zanzibar-style relationship authorization | Apache-2.0 | If resource ownership and sharing outgrow attribute rules, this is the add-on |

> [!TIP]
> Run the same policy set from [F. Data contracts](../product/plan/f-data-contracts.md) through each engine in TKT-002 and record p99 latency, deny-reason quality
> and policy-test ergonomics. That table is the ADR evidence.

---

<p align="center">
  <a href="identity-secrets-and-nhi.md">← Identity, secrets and NHI</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="supply-chain-and-scanners.md">Next: Supply chain and scanners →</a>
</p>
