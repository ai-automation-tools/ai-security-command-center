<h1 align="center">🚪 Gateway: build or adopt (TKT-001)</h1>

<p align="center">
  <em>Does an existing open-source gateway do enough of what ASCC's MCP gateway must do that we should adopt it and not build one?</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/verdict-agentgateway_meets_the_adopt_rule,_unproven_by_a_run-F59E0B?style=for-the-badge" alt="Verdict: agentgateway meets the adopt rule on a source read; not yet proven by a run">
  <img src="https://img.shields.io/badge/checked-2026--11--10-6B7280?style=for-the-badge" alt="Checked 2026-11-10">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

**Verdict: agentgateway passes four of the five criteria in [ADR-04](../product/plan/e2-adrs.md#adr-04) without
a fork, which is ADR-04's own rule for adopting.** Agent Router (the renamed Envoy AI Gateway) passes none and
ContextForge three. So the default in ADR-04, build the gateway in Go, no longer follows from the evidence.

**This was a source read, not a run.** Each project was cloned at the commit below and its code and in-repo docs
were read; nothing was built or started. The roadmap item says "tested commit hashes", and this report doesn't
meet that: the hashes are the ones read. Before ADR-04 changes, agentgateway has to be run against a stub of
the callout it would use ([what a run must show](#what-a-run-must-show)). Build or adopt is also decision K5 in
[K. Next steps](../product/plan/k-next-steps.md), so it is on the roadmap as an owner item.

## What was read

| Project | Commit | Committed | Language | Licence |
|:--|:--|:--|:--|:--|
| [**agentgateway**](https://github.com/agentgateway/agentgateway) | `2f1e69ed409bf31ada0becffda5681bf08531a3a` | 2026-11-10 | Rust data plane, Go Kubernetes controller | Apache-2.0 (one vendored crate, `crates/pool`, is MIT) |
| [**Agent Router**](https://github.com/envoyproxy/ai-gateway) (was Envoy AI Gateway) | `165733c58c0edcc18fc315229349e8597aaa2c65` | 2026-11-10 | Go, on Envoy Gateway | Apache-2.0 |
| [**ContextForge**](https://github.com/IBM/mcp-context-forge) | `8af8c39310bb7814278a68ebc7e85014f02343f1` | 2026-11-10 | Python (FastAPI) | Apache-2.0 |

## The criteria

ADR-04 lists six things and says "adopt if 4 of 5 pass without forking". This report reads that as five
functional criteria with the licence as a gate: a project with the wrong licence is out whatever it does. All
three pass the gate.

One criterion changed under us. MCP revision 2026-07-28 removed protocol sessions (see the
[[V] check results](../product/plan/k-next-steps.md#11-v-check-results-2026-11-10)), so "session termination API"
is scored as what it was for: can one caller be cut off at once, without a restart.

| # | Criterion | agentgateway | Agent Router | ContextForge |
|:--|:--|:--|:--|:--|
| **1** | Proxies MCP over Streamable HTTP, the 2026-07-28 revision included | **Pass** | Partial | **Pass** |
| **2** | `tools/list` filtered per caller, with a hook for an outside decision | **Pass** | Partial | Partial |
| **3** | Outside authorization per `tools/call`, given the tool name and arguments | **Pass** | Partial | **Pass** |
| **4** | One caller cut off at once | Partial | Fail | Partial |
| **5** | Custom logic added without a fork | **Pass** | Partial | **Pass** |
| | **Passes** | **4 of 5** | **0 of 5** | **3 of 5** |

## Findings

### agentgateway

| Criterion | What the source shows | Where |
|:--|:--|:--|
| **1** | Streamable HTTP, SSE and stdio upstreams. The stateless revision is handled, `server/discover` included; stdio and SSE upstreams still use the older handshake. | `crates/agentgateway/src/mcp/upstream/`, `mcp/handler.rs`, `mcp/README.md` |
| **2** | Each tool in a merged `tools/list` is checked against CEL rules that see the caller's token claims and headers. A remote "guardrail" service is also shown the merged list and may rewrite or reject it. | `mcp/handler.rs` (`merge_tools`), `mcp/rbac.rs`, `crates/protos/proto/ext_mcp.proto` |
| **3** | The `ext_mcp` gRPC callout is sent the MCP method and the request's parameters, so the tool name and arguments, on every `tools/call`. It can pass, change or reject the call, and **its default when the service can't be reached is to fail closed.** | `mcp/guardrails/client.rs`, `mcp/guardrails/mod.rs` |
| **4** | No admin call ends a session or revokes one identity. Authorization and the callout run per request, so a deny from ASCC's PDP stops further calls. Whether a stream already open is torn down wasn't established. | `management/admin.rs`, `mcp/session.rs` |
| **5** | No plugin host and no WebAssembly. Custom logic is CEL in config or a gRPC service in any language (`ext_mcp`, `ext_proc`, `ext_authz`); Go stubs for `ext_mcp` ship in the repo. | `http/ext_proc.rs`, `api/ext_mcp*.go` |

Also: it runs as one binary from a YAML file with no Kubernetes, it proxies A2A, and it has **no pinning of tool
descriptions**. The response side of `ext_mcp` is where ASCC would do that.

### Agent Router

| Criterion | What the source shows | Where |
|:--|:--|:--|
| **1** | Proxies Streamable HTTP and names the 2026-07-28 revision, but on that path only `server/discover` and the list methods are handled: **`tools/call` falls through to "unknown method"**. | `internal/mcpproxy/modern.go` |
| **2** | Tools are filtered per caller by static selectors and in-process rules. No outside decision. | `internal/mcpproxy/handlers.go`, `authorization.go` |
| **3** | Envoy `ext_authz` can sit in front and may be sent the raw body, so a PDP could parse the JSON-RPC itself. The gateway never hands a parsed tool name or arguments to an outside service. Its own rules are CEL and do see them; a rule that errors is a deny. | `api/v1beta1/mcp_route.go`, `authorization.go` |
| **4** | Sessions are client-held encrypted tokens with no server-side record, so there is nothing to revoke short of a new deny rule or rotating the seed, which drops everyone. | `internal/mcpproxy/session.go`, `crypto.go` |
| **5** | No extension point in the MCP proxy. Custom logic means an Envoy filter or a fork. | `internal/mcpproxy/` |

It is configured through Kubernetes resources on Envoy Gateway; a standalone mode starts Envoy as a child
process. A2A is a documentation recipe, not code.

### ContextForge

| Criterion | What the source shows | Where |
|:--|:--|:--|
| **1** | Streamable HTTP and SSE upstreams, with the stateless revision handled in both directions. stdio needs a separate bridge. | `mcpgateway/services/gateway_service.py`, `mcpgateway/transports/streamablehttp_transport.py` |
| **2** | Per-caller filtering by team, visibility and token scope only. No plugin hook on listing. | `mcpgateway/services/tool_service.py` |
| **3** | A `tool_pre_invoke` plugin hook gets the tool name, arguments and headers and can block the call. A Cedar plugin ships. **The sample config ignores plugin errors and waits 120 seconds, and two of the three plugin modes let a call through when the plugin fails.** | `mcpgateway/services/tool_service.py`, `plugins/config.yaml`, `plugins/unified_pdp/` |
| **4** | Tokens can be revoked and users deactivated, checked on each request, with up to 30 seconds of cache unless Redis is there. No call ends one client's session. | `mcpgateway/routers/tokens.py`, `mcpgateway/cache/auth_cache.py` |
| **5** | In-process Python plugins or external ones over MCP. The framework is a separate package, not in the repo. | `plugins/config.yaml` |

It would put Python on the hot path, the reason ADR-04 gives for rejecting LiteLLM, and its defaults fail open.

## What adopting agentgateway would mean

- **ASCC's logic stays in Go.** agentgateway would be the data plane, and one Go service behind `ext_mcp` would
  hold everything ADR-04 calls differentiating: the PDP call per `tools/call`, `tools/list` filtering,
  description pinning and the delegation check. ADR-02 would then cover the code ASCC writes, not the proxy.
- **The trust base grows by a Rust proxy** and its admin surface (a config dump, a config write, a quit
  endpoint). That is a change to T-20 in the [threat model](../product/plan/d-threat-model.md).
- **The kill switch is a PDP deny plus a config push**, not a call the gateway offers. That needs a test that
  an open stream is cut.
- **The LLM side wasn't read.** ADR-04's gateway also carries Anthropic and OpenAI-compatible traffic. Whether
  agentgateway's LLM proxying is enough is outside this report.

## What a run must show

Before ADR-04 is amended from "build" to "adopt", at the commit above or a later tagged release:

1. A `tools/call` is denied when a stub `ext_mcp` service says no, and the client sees the reason.
2. `tools/list` comes back with a tool removed for one identity and present for another.
3. With the stub stopped, the call is refused, not passed.
4. A deny issued while a response stream is open ends that stream.
5. The stub can reject a `tools/list` whose description differs from a pinned one.
6. Added latency per call with the callout in place, against NFR-02.

If any of 1 to 3 fails, agentgateway drops to three passes and ADR-04's build default stands.

## Not covered

- Nothing was run. Every "pass" is a reading of code at one commit.
- Callout latency, the LLM adapters, A2A in depth, each project's dependency licences and its history.
- Whether agentgateway's Windows build works. ASCC's first runtime is Claude Code on Windows.
- LiteLLM and Kong, which ADR-04 already rejects; the [V] checks confirmed their open-core split.

---

<p align="center">
  <a href="2026-claude-code-hook-contract.md">← Claude Code hook contract</a> ·
  <a href="../README.md">Docs home</a> ·
  <a href="2026-proof-of-possession.md">Proof of possession →</a>
</p>
