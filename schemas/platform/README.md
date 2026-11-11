<a id="schemas-top"></a>

<h1 align="center">📐 ASCC platform schemas</h1>

<p align="center">
  <em>The platform's data contracts as JSON Schema, with fixtures and a validator. All of it is <code>v1alpha1</code>.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/contracts-v1alpha1-8B5CF6?style=for-the-badge" alt="Contracts: v1alpha1">
  <a href="../../docs/product/plan/f-data-contracts.md"><img src="https://img.shields.io/badge/source-data_contracts-2ea44f?style=for-the-badge" alt="Source: data contracts"></a>
  <a href="../../README.md"><img src="https://img.shields.io/badge/↩-repository_root-6B7280?style=for-the-badge" alt="Repository root"></a>
</p>

---

These files turn [F. Data contracts](../../docs/product/plan/f-data-contracts.md) into something a machine can
check. F is the source: where a schema and F disagree, F wins and the schema has a bug. Field names may still
change before `v1`.

**The `$id` URLs are not published yet.** Each schema names itself
`https://ai-automation-tools.dev/schemas/ascc/v1alpha1/<name>.json`, and nothing is served there today. Use the
files in this folder.

## 📄 The schemas

Each one is JSON Schema draft 2020-12 and self-contained: it refers to nothing outside its own file.

| Schema | What it checks | F section | Compatibility rule (F §8) |
|:---|:---|:---|:---|
| [**registry-entity**](v1alpha1/registry-entity.schema.json) | One registry entity: the common envelope and, under `spec`, the fields of its `type`. Fifteen types, from `agent` to `optimization_finding`, with `nhi_account`, `connector`, `plugin` and `workflow` among them. | §1, §2.1, §2.2, §2.3, §2.5 | Versioned with the registry schema. A new NHI `kind`, fingerprint field or optimization `class` is additive; retiring a `kind` needs a migration. Until the schema gains one, an unknown `kind` fails here. |
| [**edge**](v1alpha1/edge.schema.json) | One relationship row: `tenant_id`, `from_id`, `relation`, `to_id`, `attrs`. | §2.4 | Versioned with the registry schema. |
| [**event**](v1alpha1/event.schema.json) | The event envelope every enforcement point and the plugin host emit. | §4 | Consumers must ignore unknown fields, so no object in it is closed. Removing or retyping a field is a new major. |
| [**transaction-token**](v1alpha1/transaction-token.schema.json) | The claims of the transaction token (`typ: txn+jwt`). | §3 | Versioned by `typ` and the `ascc.v` claim. Enforcement points accept the current and the previous minor. Unknown claims are refused. |
| [**plugin-capability-token**](v1alpha1/plugin-capability-token.schema.json) | The claims of the token the plugin host mints for a plugin (`typ: plgcap+jwt`). | §3 | Versioned by `typ` and `ascc.v`. The management API accepts the current and the previous minor. Unknown claims are refused. |
| [**plugin-manifest**](v1alpha1/plugin-manifest.schema.json) | `plugin.yaml` as its JSON data model. | §2.5, FR-PLG-03 | `manifest_version`, default 1. Unknown fields are refused at install, at every level. A new field means a new manifest version. |
| [**pdp-request**](v1alpha1/pdp-request.schema.json) | The body of `POST /pdp/v1/decide`. | §6 | F gives none. Closed, except `context`. |
| [**pdp-response**](v1alpha1/pdp-response.schema.json) | The decision: `allow` or `deny`, and nothing else. | §6 | F gives none. Closed. `obligations` is required, and one outside the `v1alpha1` vocabulary, or without its argument, fails. |

**A schema can't stop a secret being written into a document.** What these do: a credential field takes a
reference id, a digest is `sha256:` and 64 hex characters, a key thumbprint is 43 base64url characters, an
instance key is a public JWK that refuses a private member, and closed objects leave no spare field to put a
value in. What they can't do: tell a secret from any other text in a free-text field (`purpose`, `reason`, a
finding's `evidence`), in a URL's path, in `injection.format` on a credential reference, or in `args_preview`
and `context`, which are free-form by design. Whatever writes those fields has to keep secrets out of them.

Lists that grant something are narrow on purpose. A host is a lowercase DNS name with a dot in it: no scheme,
port, userinfo, path, wildcard or IP address. A plugin scope ends in `@self`. Each such list has no repeats and
at most 64 entries. An agent a vendor hosts and an NHI account can't be labelled `enforced`.

A JSON Schema checks one document. It can't check a token's signature or `typ` header, that `exp` is after
`iat` or within an hour of it, that `ascc.depth` matches the depth of the `act` chain, or that an id points at
something that exists.

## ✅ Run the validator

The same commands work in bash and PowerShell, from the repository root.

```
go run ./src/cmd/ascc-schema list
go run ./src/cmd/ascc-schema validate --schema event schemas/platform/v1alpha1/fixtures/event/valid/mcp-tool-call.json
go run ./src/cmd/ascc-schema validate --schema transaction-token -
```

`-` reads the document from stdin. A manifest written as YAML has to be converted to JSON first.

| Exit code | Meaning |
|:---|:---|
| **0** | The document is valid. |
| **1** | It isn't. Each failure is one line on stderr: the file, the JSON pointer, the rule, and where the rule sits in the schema. The value found there is never printed. A pointer does contain property names. |
| **2** | The validator couldn't run: an unknown schema name, an unreadable file, bad flags, or a document it won't check. That is one that isn't JSON, is larger than 1 MiB, or has the same key twice in one object, since two parsers can read such a document two ways. |

The validator opens the schema file it was asked for and nothing else. A `$ref` that would need a fetch or
another file stops the schema compiling. It is its own binary; `ascc` doesn't include it.

Property names are printed, quoted, because a pointer is made of them. So a secret used as a property name
would be shown.

## 🧪 Fixtures

`v1alpha1/fixtures/<schema>/valid/` holds documents that must pass and `invalid/` documents that must fail.
Each invalid file is named for the one rule it breaks, and `expect.json` beside the two folders gives the JSON
pointer and the keyword it must fail on. `go test ./src/cmd/ascc-schema` walks the folders and holds every
invalid file to its entry, so a new fixture needs a line in `expect.json` and no test change. CI runs it with
the rest of `go test ./...`. Everything in a fixture is made up.

## 🔤 Transaction token claim names

The token keeps F §3's claim names for `v1alpha1`. The table maps them to the nearest names in the IETF
transaction-tokens draft (-11), whose claim names were checked on 2026-11-10
([[V] check results](../../docs/product/plan/k-next-steps.md#11-v-check-results-2026-11-10)). It is informative
only: none of the draft's names is adopted, and a token carrying one fails validation.

| F §3 claim | Nearest draft name |
|:---|:---|
| **`ascc.task_id`** | `txn` |
| **`ascc.scope`** | `scope` |
| **`act.sub`** (the current actor) | `req_wl` |
| **`ascc.purpose`**, **`ascc.session_id`**, **`ascc.budget_ref`**, **`ascc.authority`** | `tctx` |
| *nothing yet* | `rctx` |

<p align="right">(<a href="#schemas-top">back to top</a>)</p>

---

<p align="center">
  <a href="../../README.md">← Repository home</a> ·
  <a href="../../docs/product/plan/f-data-contracts.md">Data contracts →</a>
</p>
