<h1 align="center">⚖️ The PDP as a service</h1>

<p align="center">
  <em>The same decision the Claude Code hook gets, asked for over HTTP by anything on the machine.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-first_platform_service-F59E0B?style=for-the-badge" alt="Status: first platform service">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

`ascc-pdp` is the policy decision point behind an HTTP endpoint. It calls the same `Decide` the hook calls, on
the same registry and policy files, so it can't answer differently from `ascc decide`. It is a binary of its
own: the `ascc` CLI never opens a socket, and nothing in it imports this.

> [!WARNING]
> **It doesn't know who is asking, and it keeps no audit chain.** Any process that can reach the port gets an
> answer, and decisions go to a log on standard output, not to the hash-chained file `ascc` writes. Both come
> with the token service and the event pipeline. Until then it is for a loopback address or a private Compose
> network, and nothing should treat its log as the record.

## Run it

From a folder with a registry and policies (the samples in `examples/` work):

```bash
go build -o bin/ ./src/cmd/ascc-pdp
cd examples
../bin/ascc-pdp
```

| Flag | Default | What it does |
|:---|:---|:---|
| **`--listen`** | `127.0.0.1:8181` | Address to listen on. Anything but a literal loopback address is refused (`localhost` too: it is whatever the hosts file says), |
| **`--allow-remote`** | off | unless this is set. Use it only on a network where every peer may ask. |
| **`--registry`** | `ascc.registry.json` | The agent registry. |
| **`--policies`** | `policies` | The folder of `*.cedar` files. |

It exits 2 if it can't listen or the flags are wrong. It starts even when the registry or policies don't load:
it then denies everything, which is safer than leaving an enforcement point with nothing to ask.

## Ask for a decision

`POST /pdp/v1/decide` as `application/json`, with the request from [F §6](../product/plan/f-data-contracts.md), which
[`pdp-request.schema.json`](../../schemas/platform/v1alpha1/pdp-request.schema.json) describes:

```bash
curl -s -X POST http://127.0.0.1:8181/pdp/v1/decide -H 'Content-Type: application/json' -d '{
  "tenant_id": "tnt_lab",
  "principal": { "type": "Agent", "id": "dev-agent" },
  "action": "runtime.tool_use",
  "resource": { "type": "Tool", "id": "claude-code/Bash" },
  "context": { "path_in_repo": true, "network": true, "touches_secret_path": true, "touches_ascc_state": false }
}'
```

```json
{"decision":"deny","policy_version":"sha256:9e4ce86e6bc72d97","determining_policies":["deny-network-with-secret-path"],"reasons":["forbidden by deny-network-with-secret-path: a shell command may not send a secret file over the network"],"obligations":[],"degraded":false}
```

`policy_version` is a hash of the registry and policy files the answer came from, so it changes when either
does. `obligations` is always empty and `degraded` always false in this version.

`GET /healthz` answers 200 only while the registry and policies load, which is the only time a decision can
be anything but a deny.

## What it does when something is wrong

**On every path but `/healthz` the body is a decision, and it is a deny unless the engine said allow.** An
enforcement point that reads only the body still fails closed.

| What happened | Status | Body |
|:---|:---|:---|
| **Policy says no, or no policy says yes** | 200 | deny, with the policies and the reason |
| **An agent or tool the registry doesn't list, a suspended agent** | 200 | deny |
| **A principal that isn't an `Agent`, a resource that isn't a `Tool`** | 200 | deny: this version knows nothing else |
| **A context attribute the schema lacks, or of the wrong type** | 200 | deny |
| **Not JSON, an unknown field, anything after the document** | 400 | deny |
| **A key used twice, or spelled in another case (`CONTEXT`)** | 400 | deny: a second `context` would otherwise override the first, and another reader of the same bytes could see a different request |
| **Another path, a near miss on this one included** | 404 | deny |
| **Not a `POST`** | 405 | deny |
| **A body over 1 MiB** | 413 | deny |
| **Not sent as `application/json`** | 415 | deny: this keeps a web page in someone's browser from questioning the service |
| **The registry or a policy doesn't load, or a policy file is empty** | 503 | deny, until it is fixed |
| **The service itself fails, or can't write its log line** | 500 | deny: an allow that can't be written down doesn't go out |

**A change on disk applies to the next request.** The files are read on every request, so `ascc kill`, an edit
to the registry or a policy change needs no restart, and a file that stops loading denies everything at once.
Write those files by replacing them whole (write a new file, then rename it over the old one): a request that
arrives while a file is half written sees whatever is there. A file cut off mid-policy or emptied denies
everything. **A policy file that is deleted, or cut off exactly between two policies, still loads, without
the rules it lost.** Nothing here detects that, so keep the policy folder where an agent can't write.

**No context value comes back or is kept.** The context is where a secret could ride along, and neither the
answer nor the log repeats a value or a key from it. Names are different: the agent, tool and action a caller
sent can appear in the reason, and the log line has them (cut at 200 bytes) with the trace id and a SHA-256 of
the request. Don't put a secret in a name.

## How fast

Measured with `go test -run '^$' -bench DecideAtScale ./src/cmd/ascc-pdp`, on 500 policies and 5,000 entities
(1,000 agents, 4,000 tools), on one machine (Ryzen 5 3600, Windows 11):

| | Mean | p99 | Target (NFR-01) |
|:---|:---|:---|:---|
| **The engine alone** | 0.04 ms | 1.0 ms | p99 ≤ 2 ms in process |
| **Through the service's handler** | 1.0 ms | 2 to 6 ms | p95 ≤ 10 ms remote |

The service's extra millisecond is reading and hashing every file on every request. The second row has no
network in it: the handler was called in process. Four idle runs agreed; one run straight after the full test
suite, with the machine busy, gave a p99 of 112 ms, so these are a quiet machine's numbers. The engine's p99
sits at Windows' timer step, so the true figure may be lower.

## What it doesn't cover

- **Who is calling.** No token is checked; `token_jti` and `tenant_id` are accepted and not used.
- **An audit chain.** The log is a log.
- **Anything but agents calling tools.** The model actions, credentials, egress and delegation in the plan's
  policy set arrive with the slices that enforce them.
- **TLS.** It speaks plain HTTP, which is why it stays on loopback or a private network.
- **Obligations, cached answers and tiers,** which wait on decision K3.

---

<p align="center">
  <a href="claude-code-setup.md">← Claude Code setup</a> ·
  <a href="../README.md">Docs home</a> ·
  <a href="../ROADMAP.md">Roadmap →</a>
</p>
