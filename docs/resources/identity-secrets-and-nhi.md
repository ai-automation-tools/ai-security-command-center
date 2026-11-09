<h1 align="center">🪪 Identity, Secrets and NHI</h1>

<p align="center">
  <em>Identity providers, secret stores, workload identity, and the platform APIs the NHI connectors will read.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/epics-EP--03_·_EP--09_·_EP--21-8B5CF6?style=for-the-badge" alt="Epics EP-03, EP-09, EP-21">
  <a href="README.md"><img src="https://img.shields.io/badge/↩-resources-6B7280?style=for-the-badge" alt="Back to resources"></a>
</p>

---

## 🔑 Identity and secrets components

| Project | What it is | Licence | Where ASCC uses it |
|:---|:---|:---|:---|
| [**OpenBao**](https://github.com/openbao/openbao) | Community fork of Vault under the Linux Foundation | MPL-2.0 | The bundled secrets backend and the first MVP connector (ADR-07) |
| [**HashiCorp Vault**](https://github.com/hashicorp/vault) | The original; now BUSL | BUSL (source-available) | Supported through a connector, never redistributed. BUSL terms are **[V]** |
| [**Infisical**](https://github.com/Infisical/infisical) | Secrets, certificates and access platform that now markets agent support | MIT core, enterprise dir separate | A likely Phase 2 connector target and a competitor for the "broker" story |
| [**Keycloak**](https://github.com/keycloak/keycloak) | Self-hosted IdP with token exchange support | Apache-2.0 | Dev-stack IdP for the human side of delegation |
| [**Ory Hydra**](https://github.com/ory/hydra) | OAuth 2.1 and OIDC provider in Go | Apache-2.0 | Option if the STS should be a certified OAuth server rather than hand-rolled |
| [**SPIRE**](https://github.com/spiffe/spire) | SPIFFE runtime: issues and rotates SVIDs | Apache-2.0 | Server and Kubernetes agent identity, Phase 2 |
| [**Teleport**](https://github.com/gravitational/teleport) | Access proxy for SSH, Kubernetes, DBs, with recorded sessions and MCP access | AGPL-3.0 | Competitor and pattern source for JIT access and session recording |

## 🔭 NHI references

| Resource | What it is | Why it matters |
|:---|:---|:---|
| [**OWASP Non-Human Identities Top 10**](https://owasp.org/www-project-non-human-identities-top-10/) | The ten most common NHI failures: improper offboarding, secret leakage, overprivilege, long-lived secrets… | Map each hygiene finding in FR-CON to one of these items |
| [**BloodHound**](https://github.com/SpecterOps/BloodHound) | Attack-path graph over AD and Entra, with OpenGraph for custom node types | Model for the identity graph and "who can reach what" queries |
| [**Cartography**](https://github.com/cartography-cncf/cartography) | CNCF tool that pulls cloud assets and relationships into Neo4j | Reference for multi-cloud NHI ingestion and reconciliation |

## 🧩 Connector target APIs

The first connectors (OpenBao, GitHub) and the Phase 2 set in FR-CON-10. Each one needs its read scopes
written down before a ticket starts (TKT-037, K item 3).

> [!NOTE]
> The network dropped while these platform doc links were being checked, so this table was **not
> link-checked** on 2026-11-09. They are long-lived doc URLs, but confirm each before citing it in an ADR.

| Platform | API reference | NHI objects to read |
|:---|:---|:---|
| **OpenBao** | [OpenBao API docs](https://openbao.org/api-docs/) | KV v2 metadata, AppRole roles, transit keys, lease listing |
| **GitHub** | [REST: Apps](https://docs.github.com/en/rest/apps) | Apps, installations and permissions, deploy keys, PAT policy, Actions secret names |
| **AWS IAM** | [IAM API reference](https://docs.aws.amazon.com/IAM/latest/APIReference/welcome.html) | Roles, users, access keys and last-used dates |
| **Microsoft Entra ID** | [Graph: servicePrincipal](https://learn.microsoft.com/en-us/graph/api/resources/serviceprincipal) | Service principals, app registrations, credentials and expiry |
| **Google Cloud** | [Service accounts overview](https://cloud.google.com/iam/docs/service-account-overview) | Service accounts, keys, IAM bindings |
| **Okta** | [Okta API](https://developer.okta.com/docs/api/) | API service integrations, OAuth clients, tokens |
| **Kubernetes** | [Service accounts admin](https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/) | ServiceAccounts, token secrets, RoleBindings |

---

<p align="center">
  <a href="gateways-and-proxies.md">← Gateways and proxies</a> ·
  <a href="README.md">Resources home</a> ·
  <a href="policy-engines.md">Next: Policy engines →</a>
</p>
