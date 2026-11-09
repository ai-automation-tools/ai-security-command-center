<h1 align="center">🧩 Plugins and Connectors</h1>

<p align="center">
  <em>Plugin runtimes to build on, and connector ecosystems to copy the shape of.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/decides-ADR--15_·_ADR--16-8B5CF6?style=for-the-badge" alt="Decides ADR-15 and ADR-16">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

The plugin platform (EP-22) is the headline feature, and connectors (EP-21) are its first customers. Several
open-source projects already run large connector ecosystems. Their contracts, SDKs and contributor docs are
the fastest way to avoid designing `v1alpha1` twice.

## ⚙️ Plugin runtimes

| Project | What it is | Licence | How ASCC would use it |
|:---|:---|:---|:---|
| [**go-plugin**](https://github.com/hashicorp/go-plugin) | HashiCorp's out-of-process plugins over gRPC; what Terraform and Vault use | MPL-2.0 | The default ADR-15 host model: process isolation, versioned handshake |
| [**Extism**](https://github.com/extism/extism) | Cross-language WASM plugin framework with host functions | BSD-3-Clause | Phase 3 WASM spike for hot-path detectors |
| [**wazero**](https://github.com/wazero/wazero) | Pure-Go WASM runtime. Moved from `tetratelabs/wazero` | Apache-2.0 | The WASM host if ASCC skips Extism |

## 🔗 Connector ecosystems to learn from

| Project | What it is | Licence | What to borrow |
|:---|:---|:---|:---|
| [**Steampipe**](https://github.com/turbot/steampipe) | SQL over cloud and SaaS APIs through plugins | AGPL-3.0 | Plugin SDK ergonomics and table schemas per platform. Read, don't embed (AGPL) |
| [**CloudQuery**](https://github.com/cloudquery/cloudquery) | Cloud and SaaS data pipelines with 70+ source plugins | MPL-2.0 | Sync scheduling, incremental state, schema migration for connectors |
| [**Prowler**](https://github.com/prowler-cloud/prowler) | Multi-cloud security and compliance checks | Apache-2.0 | Check catalog for cloud IAM hygiene findings |
| [**ScoutSuite**](https://github.com/nccgroup/ScoutSuite) | Multi-cloud auditing tool | GPL-2.0 | Older reference; quiet since Sept 2025 |
| [**Cartography**](https://github.com/cartography-cncf/cartography) | Asset and relationship graph across clouds | Apache-2.0 | Cross-source reconciliation model for NHIs |
| [**Backstage**](https://github.com/backstage/backstage) | Developer portal with a large plugin marketplace | Apache-2.0 | How to run a plugin library: listing pages, ownership metadata, the console-panel plugin type |
| [**Terraform**](https://github.com/hashicorp/terraform) | IaC with the largest provider registry | BUSL | Registry tiers (official, partner, community) map onto `verified` and `community` |
| [**GitHub MCP Server**](https://github.com/github/github-mcp-server) | GitHub's official MCP server | MIT | A realistic upstream to put behind the gateway in the MVP demo |

---

<p align="center">
  <a href="telemetry-audit-and-detection.md">← Telemetry, audit and detection</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="standards-and-research.md">Next: Standards and research →</a>
</p>
