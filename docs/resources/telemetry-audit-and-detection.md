<h1 align="center">📡 Telemetry, Audit and Detection</h1>

<p align="center">
  <em>Tracing conventions, tamper-evident logs, endpoint sensors and SOC tooling the evidence plane plugs into.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/epics-EP--12_·_EP--17-8B5CF6?style=for-the-badge" alt="Epics EP-12 and EP-17">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

Every decision ASCC makes lands in a hash-chained audit log and an OpenTelemetry trace. The first table is
what defines that output; the rest is where it goes next.

## 🧵 Tracing and LLM observability

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**OTel GenAI semantic conventions**](https://opentelemetry.io/docs/specs/semconv/gen-ai/) · [**repo**](https://github.com/open-telemetry/semantic-conventions) | Attribute names for LLM, agent and tool spans. Status is still experimental | Apache-2.0 | Span names and attributes in F; status is **[V]** |
| [**OpenLLMetry**](https://github.com/traceloop/openllmetry) | OTel instrumentation for LLM SDKs | Apache-2.0 | SDK-side spans for the Phase 2 Python and TS interceptors |
| [**Langfuse**](https://github.com/langfuse/langfuse) | Self-hosted LLM tracing and evals | MIT core, enterprise dir separate | Likely already in use by teams ASCC protects; an export target, not a dependency |
| [**Arize Phoenix**](https://github.com/Arize-ai/phoenix) | LLM observability and eval UI | Elastic-style (read LICENSE) | Same role as Langfuse; check licence before recommending |

## 🧾 Tamper-evident audit

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**Trillian**](https://github.com/google/trillian) | Verifiable append-only log (the engine under Certificate Transparency) | Apache-2.0 | External anchoring for audit checkpoints in Phase 3 (ADR-10) |

## 🚨 Endpoint sensors and SOC

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**Tetragon**](https://github.com/cilium/tetragon) | eBPF security observability and in-kernel enforcement | Apache-2.0 | Research path for OS-level mediation of agents that bypass hooks (Phase 4) |
| [**Falco**](https://github.com/falcosecurity/falco) | Runtime threat detection from syscalls | Apache-2.0 | Detect agent processes spawning shells or reaching for credentials on Linux hosts |
| [**Wazuh**](https://github.com/wazuh/wazuh) | Open-source XDR and SIEM | GPL-2.0 (read LICENSE) | First SIEM export target for the lab (EP-17) |
| [**Sigma**](https://github.com/SigmaHQ/sigma) | Vendor-neutral detection rule format | Rules DRL-1.1 | Ship ASCC detections as Sigma rules so any SIEM can load them |
| [**Elastic detection rules**](https://github.com/elastic/detection-rules) | Elastic's rule repo | Elastic License 2.0 | Reference for rule metadata and testing |
| [**AiSOC**](https://github.com/beenuar/AiSOC) | Self-hosted AI SOC: alert fusion, LLM-agent triage, replayable decision ledger, MCP server. On Mike's list | MIT | Consumer of ASCC alerts; its decision ledger is worth comparing with ASCC's audit chain |

---

<p align="center">
  <a href="runtime-guardrails-and-red-team.md">← Runtime guardrails and red team</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="plugins-and-connectors.md">Next: Plugins and connectors →</a>
</p>
