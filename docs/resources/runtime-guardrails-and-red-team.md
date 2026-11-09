<h1 align="center">🛡️ Runtime Guardrails and Red Team</h1>

<p align="center">
  <em>Detectors for prompts, responses and tool results, and the tools that attack ASCC to prove they work.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/epics-EP--15_·_validation_I-8B5CF6?style=for-the-badge" alt="EP-15 and validation plan">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

DLP and injection detection are probabilistic, so the package ships them after the deterministic controls
(U9, Phase 2) and in flag mode first. These are the detector candidates and the test harnesses for
[I. Validation](../product/plan/i-validation.md).

## 🧯 Detectors and guardrail frameworks

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**Presidio**](https://github.com/microsoft/presidio) | PII detection and redaction. Now lives at `data-privacy-stack/presidio`; the Microsoft URL redirects | MIT | First PII detector plugin (FR-DLP) |
| [**NeMo Guardrails**](https://github.com/NVIDIA/NeMo-Guardrails) | Programmable rails for LLM apps. Moved to `NVIDIA-NeMo/Guardrails` | Read the repo LICENSE (GitHub shows no SPDX id) | Pattern source for dialog and tool rails |
| [**PurpleLlama**](https://github.com/meta-llama/PurpleLlama) | Meta's Llama Guard, Prompt Guard, CodeShield and CyberSecEval | Mixed; models carry Llama licences | Local injection and jailbreak classifiers; check model licences before bundling |
| [**Guardrails AI**](https://github.com/guardrails-ai/guardrails) | Validator framework for LLM output | Apache-2.0 | Validator ideas for structured tool results |
| [**OpenAI Guardrails (Python)**](https://github.com/openai/openai-guardrails-python) | OpenAI's guardrail library | MIT | Comparison point for check taxonomy |
| [**Agent Governance Toolkit**](https://github.com/microsoft/agent-governance-toolkit) | Microsoft's policy, identity and sandboxing toolkit for agents; claims all OWASP Agentic Top 10 | MIT | Closest open-source overlap with ASCC. Read it before Phase 1 and say how ASCC differs |

> [!WARNING]
> **LLM Guard** (`protectai/llm-guard`) and **Rebuff** (`protectai/rebuff`) are archived on GitHub. Don't
> build a detector plugin on either.

## 🎯 Red-team and evaluation harnesses

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**garak**](https://github.com/NVIDIA/garak) | LLM vulnerability scanner with many probe families | Apache-2.0 | Probe the gateway's LLM path in CI |
| [**promptfoo**](https://github.com/promptfoo/promptfoo) | Eval and red-team runner with agent and MCP support | MIT | Abuse-case suite runner (ABUSE-01..27) |
| [**PentAGI**](https://github.com/vxcontrol/pentagi) | Autonomous multi-agent pentest system. On Mike's list | MIT | Point it at the dev stack as an adversary in RT exercises; also a realistic "agent to govern" |

---

<p align="center">
  <a href="supply-chain-and-scanners.md">← Supply chain and scanners</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="telemetry-audit-and-detection.md">Next: Telemetry, audit and detection →</a>
</p>
