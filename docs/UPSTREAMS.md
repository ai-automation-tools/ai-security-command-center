# Upstreams

Every external tool, library, and platform AI Security Command Center depends on, what the code
assumes about each, and where to check whether that assumption still holds. The biweekly
**AI-Security-Command-Center Upstream Check** routine (`upstream/auto-*` PRs) works from this file.
Keep it current by hand too: a new dependency or a new assumption about a tool is a new row.

The `ascc` binary makes no network call and holds no secret, so most upstreams here are *contracts
read from local files* (Claude Code's settings and hook payload) rather than services it calls.

**Last checked** is filled in by the routine, and only with a date it actually read the source.
`—` means never checked.

## Agents it governs

| Upstream | What the code assumes | Code | Check at | Last checked |
|:---|:---|:---|:---|:---|
| **Claude Code PreToolUse hook contract** | stdin JSON carries `hook_event_name` (must be `PreToolUse`), `tool_name`, `tool_input`, `cwd`; a block is `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":...}}` on stdout with exit 0; an allow prints nothing; exit 2 also blocks; the hook is registered with `"onFailure": "block"`; no agent identity is in the payload (so `--agent` is required) | `src/internal/claudecode/claudecode.go`, `src/cmd/ascc/`, `examples/hook-payloads/`, `docs/guides/claude-code-setup.md`, `docs/reports/2026-claude-code-hook-contract.md` | code.claude.com/docs/en/hooks | — |
| **Claude Code settings and MCP config files** | `posture` and `discover` read `managed-settings.json` and `managed-settings.d/*.json`, `~/.claude/settings.json`, `.claude/settings.json`, `.claude/settings.local.json`, `.mcp.json`, `managed-mcp.json`, `.claude.json`; they judge a hook as enforcing only when it is in managed settings, matcher `*`, exec form, `"onFailure": "block"`, not async, and no setting switches hooks off | `src/internal/claudecode/posture.go`, `src/internal/claudecode/discover.go` | code.claude.com/docs/en/settings ; code.claude.com/docs/en/hooks ; code.claude.com/docs/en/mcp | — |
| **Claude Code built-in tool names** | The policies and the request mapper match tool names such as `Bash`, `Read`, `Write`, `Edit`, `WebFetch`, `PowerShell`, `Monitor` and MCP tools named `mcp__<server>__<tool>`; `@` file mentions and some tools need care | `src/internal/claudecode/claudecode.go`, `examples/policies/` | code.claude.com/docs/en/hooks (tool matchers) ; code.claude.com/docs/en/tools-reference | — |

## Policy engine and Go

| Upstream | What the code assumes | Code | Check at | Last checked |
|:---|:---|:---|:---|:---|
| **cedar-go** `github.com/cedar-policy/cedar-go` v1.9.0 | Matches the Rust reference on the sample policies and schema validation; returns determining policy ids; needs Go 1.26; a `forbid` that errors fails open, so the code denies unknown entities and any evaluation error | `go.mod`, `src/internal/pdp/`, `docs/reports/2026-cedar-go-check.md` | github.com/cedar-policy/cedar-go/releases ; docs.cedarpolicy.com | — |
| **Cedar language** | Policy syntax and semantics in `examples/policies/*.cedar` | `examples/policies/`, `src/internal/pdp/` | docs.cedarpolicy.com ; github.com/cedar-policy/cedar/releases | — |
| **Go toolchain** | `go 1.26.0` in every `go.mod`; CI uses `go-version-file: go.mod`; the Compose build image is `golang:1.26.11-alpine`; the demo copies `$GOROOT/lib/wasm/wasm_exec.js` and builds `GOOS=js GOARCH=wasm` | `go.mod`, `src/cmd/buildsite/main.go`, `src/cmd/ascc-wasm/`, `deploy/compose/pdp.Dockerfile`, `.github/workflows/ci.yml` | go.dev/doc/devel/release ; go.dev/wiki/WebAssembly | — |
| **santhosh-tekuri/jsonschema v6** | Test-time validation of the inventory output, platform schemas and fixtures against JSON Schema draft 2020-12 | `go.mod`, `src/cmd/ascc/main_test.go`, `src/cmd/ascc-pdp/main_test.go` | github.com/santhosh-tekuri/jsonschema/releases | — |

## Suite and schemas

| Upstream | What the code assumes | Code | Check at | Last checked |
|:---|:---|:---|:---|:---|
| **Agent Security Suite inventory schema** (org-owned) | `https://ai-automation-tools.dev/schemas/agent-inventory/v0.1.json` is byte-identical to the vendored `schemas/agent-inventory.schema.json`; CI fails on a difference | `schemas/agent-inventory.schema.json`, `.github/workflows/ci.yml` | ai-automation-tools.dev/schemas/agent-inventory/ | — |

## Gateway and platform spikes (not in the shipped binary)

| Upstream | What the code assumes | Code | Check at | Last checked |
|:---|:---|:---|:---|:---|
| **agentgateway** | The gateway run uses release `v1.7.0`, `agentgateway-linux-amd64`, checked against a pinned SHA-256; config `bindAddress` defaults to all interfaces; `ext_mcp` callouts; an open stream isn't ended by a later deny | `src/spikes/gatewayrun/`, `.github/workflows/ci.yml`, `docs/reports/2026-gateway-build-vs-adopt.md` | github.com/agentgateway/agentgateway/releases ; agentgateway.dev/docs | — |
| **gRPC and protobuf for Go** | `google.golang.org/grpc` v1.85.0 and `protobuf` v1.36.12 in the two spike modules only; the `ascc` binary must not link them (a test fails if it links a socket-capable package) | `src/spikes/gatewayrun/go.mod`, `src/spikes/pluginhost/go.mod` | github.com/grpc/grpc-go/releases | — |
| **MCP specification** | The platform design assumes the revision named in the spike reports (2026-07-28 removed sessions and server-initiated requests); no shipped code speaks MCP | `docs/product/plan/`, `docs/reports/2026-gateway-build-vs-adopt.md` | modelcontextprotocol.io/specification | — |

## Demo, CI and dev stack

| Upstream | What the code assumes | Code | Check at | Last checked |
|:---|:---|:---|:---|:---|
| **GitHub Pages and Actions** | `pages.yml` deploys `_site/` with `actions/upload-pages-artifact@v5` and `actions/deploy-pages@v5` (Pages source: workflow); CI uses `actions/checkout@v4` and `actions/setup-go@v5` on `ubuntu-latest` and `windows-latest`; the demo's custom domain and HTTPS are set on the repo's Pages settings | `.github/workflows/pages.yml`, `.github/workflows/ci.yml` | docs.github.com/en/pages ; github.com/actions/deploy-pages/releases ; github.com/actions/setup-go/releases | — |
| **Playwright and Chromium** | `playwright` 1.65.0 pinned exactly in the browser test; `npx playwright install --with-deps chromium` in CI; used only by `TestBuiltPageInABrowser` | `src/cmd/buildsite/testdata/browser/package.json`, `.github/workflows/ci.yml` | playwright.dev/docs/release-notes | — |
| **Docker Engine and Compose** | Docker Engine 28.3.3 or newer (loopback port binding survives a firewalld reload); `docker compose up --wait` with `--env-file .env` | `deploy/compose/README.md`, `deploy/compose/compose.yaml`, `deploy/compose/stack_test.go` | docs.docker.com/engine/release-notes/ | — |
| **Compose stack images** (digest-pinned) | postgres 18.6-alpine, Keycloak 26.8.2, OpenBao 2.7.2, Jaeger 2.23.0, Envoy distroless v1.40.2; Keycloak imports `ascc-dev-realm.json`; OpenBao config in `config.hcl` and `dev-init.sh`; Envoy placeholder config in `envoy.yaml` | `deploy/compose/` | hub.docker.com / each project's release page: postgresql.org/support/versioning ; keycloak.org/docs/latest/release_notes ; github.com/openbao/openbao/releases ; github.com/jaegertracing/jaeger/releases ; envoyproxy.io/docs/envoy/latest/version_history/version_history | — |
