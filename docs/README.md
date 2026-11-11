<a id="docs-top"></a>

<h1 align="center">📚 ASCC Documentation</h1>

<p align="center">
  <em>What ASCC goes public with, the full plan behind it, the research links, and what has shipped so far.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/audience-builders-8B5CF6?style=for-the-badge" alt="Audience: builders">
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/plan-ROADMAP-2ea44f?style=for-the-badge" alt="Roadmap"></a>
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-repository_root-6B7280?style=for-the-badge" alt="Repository root"></a>
</p>

---

## 🗺️ Planning

| Document | What's inside |
|:---|:---|
| [**🗺️ ROADMAP.md**](ROADMAP.md) | The release gates in build order, then the plan's Phase 1 and beyond. |
| [**📋 CHANGELOG.md**](CHANGELOG.md) | Every change, newest first. Roadmap items land here the run they're ticked. |

## 📦 Product and design

| Section | What's inside |
|:---|:---|
| [**📦 product/**](product/README.md) | The public-MVP spec with its Won't list, the build plan, and the combined planning package (charter to backlog, ADR-01 to ADR-17). |
| [**🔬 cedar-go check**](reports/2026-cedar-go-check.md) | Slice 0. cedar-go matches the Rust reference, and why the PDP must deny on any evaluation error. |
| [**🧭 Claude Code setup guide**](guides/claude-code-setup.md) | Wire the hook into a repo, check it blocks, verify the audit log, stop an agent, check the registration enforces with `posture`, find what isn't registered with `discover`, list the identities the agents use, and what none of it covers. |
| [**🖥️ Browser demo**](../site/) | The demo page's source. `go run ./src/cmd/buildsite` builds it to `_site/`: ascc as WebAssembly on the sample files, with no network call. Published from `main` to [ai-automation-tools.dev/ai-security-command-center](https://ai-automation-tools.dev/ai-security-command-center/). |
| [**🪝 Claude Code hook contract**](reports/2026-claude-code-hook-contract.md) | Slice 0. What the PreToolUse hook receives, how it blocks, where it fails open, and what it never sees. |
| [**🚪 Gateway: build or adopt**](reports/2026-gateway-build-vs-adopt.md) | Slice 4. agentgateway, Agent Router and ContextForge read at a commit each against ADR-04's criteria. agentgateway meets the adopt rule; a run has to confirm it. |
| [**🔑 Proof of possession**](reports/2026-proof-of-possession.md) | Slice 4. Which way of binding a token to a key each client can use, and the one path that can't be bound yet. |
| [**🧩 Plugin contract spike**](reports/2026-plugin-contract-spike.md) | Slice 4. The `v1alpha1` contract and a prototype host: what it proves, what fails closed, and why a laptop plugin isn't confined. |
| [**⚖️ The PDP as a service**](guides/pdp-service.md) | Slice 5b. `ascc-pdp`: how to run it, what it answers when something is wrong, how fast it is, and what it doesn't cover yet. |
| [**📐 Platform schemas**](../schemas/platform/README.md) | Slice 5a. JSON Schemas for the platform's contracts, their fixtures, and the `ascc-schema` validator. |
| [**🧰 resources/**](resources/README.md) | Specs, tools and standards grouped by the part of ASCC they feed. Look here before adding a dependency. |

<p align="right">(<a href="#docs-top">back to top</a>)</p>

---

<p align="center">
  <a href="../README.md">← Repository home</a> ·
  <a href="ROADMAP.md">Next: Roadmap →</a>
</p>
