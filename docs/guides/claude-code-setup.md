<h1 align="center">🪝 Wire ascc into Claude Code</h1>

<p align="center">
  <em>Register the hook, check it blocks, read the audit log, and stop an agent.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Claude_Code-2.1.295+-D97757?style=for-the-badge" alt="Claude Code 2.1.295 or newer">
  <img src="https://img.shields.io/badge/checked_live-2026--11--09-6B7280?style=for-the-badge" alt="Checked against a live session 2026-11-09">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

`ascc hook claude-code` runs before every tool call Claude Code makes. It asks the policies, blocks what they
deny, and writes one line per decision to a hash-chained log. This guide wires it up for one repo. The contract
behind it is in the [hook contract report](../reports/2026-claude-code-hook-contract.md).

> [!IMPORTANT]
> Run it only on agents and machines you own or are authorized to control. ascc reads its own files and writes
> its registry and audit log; it never makes a network call.

## 1. Build ascc and put it on your PATH

```bash
go build -o bin/ ./src/cmd/ascc   # bin/ascc, or bin/ascc.exe on Windows
```

Claude Code starts the hook without a shell, so `ascc` has to resolve to a real executable on `PATH` (or give
the full path as `command`).

## 2. Give the repo a registry and policies

Copy the samples and edit them:

```bash
cp examples/ascc.registry.json  <your-repo>/ascc.registry.json
cp -r examples/policies         <your-repo>/policies
```

| File | What goes in it |
|:---|:---|
| **`ascc.registry.json`** | Every agent (`id`, `owner`, `runtime`, `risk_tier`, `status`, optional `groups`) and every tool an agent may ask for, with its tool groups. **A tool that isn't listed is denied**, MCP tools included: add `claude-code/mcp__<server>__<tool>` for each one you want policy to consider. |
| **`policies/*.cedar`** | Cedar policies. Each needs an `@id("...")`; `@reason("...")` is the line Claude and the audit log show. Nothing is allowed unless a `permit` says so. |
| **`.ascc/audit.jsonl`** | Written by the hook. Add `.ascc/` to `.gitignore`. |

ascc refuses to start on a registry typo, a policy with no `@id`, or a policy that reads an attribute the
built-in schema doesn't have. While it can't start, the hook blocks every call.

## 3. Register the hook

In the repo's `.claude/settings.json` (or, for real enforcement, in managed settings: see
[What this doesn't cover](#limits)):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "ascc",
            "args": [
              "hook", "claude-code",
              "--agent", "dev-agent",
              "--repo", "${CLAUDE_PROJECT_DIR}",
              "--registry", "${CLAUDE_PROJECT_DIR}/ascc.registry.json",
              "--policies", "${CLAUDE_PROJECT_DIR}/policies",
              "--audit", "${CLAUDE_PROJECT_DIR}/.ascc/audit.jsonl"
            ],
            "timeout": 10,
            "onFailure": "block"
          }
        ]
      }
    ]
  }
}
```

All three settings matter:

| Setting | Why |
|:---|:---|
| **`"matcher": "*"`** | `Bash`, `PowerShell`, Monitor and MCP tools all reach the policies. A `Bash` matcher misses the rest. |
| **`"onFailure": "block"`** | Without it, a missing binary, a crash or a timeout lets the call through. Needs Claude Code 2.1.295 or newer. |
| **`"timeout": 10`** | The default is 600 seconds. A decision takes milliseconds; a hook that hangs should block soon. |

`--agent` is the registry id this registration speaks for. `--repo` is what "inside the repo" means to the
policies; without it, ascc uses the session's working directory.

## 4. Check it

Without Claude Code, feed it a sample payload. From `examples/`:

```bash
ascc hook claude-code --agent dev-agent < hook-payloads/exfil-curl.json
```

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"ascc: forbidden by deny-network-with-secret-path: a shell command may not send a secret file over the network"}}
```

`hook-payloads/read-repo-file.json` prints nothing and exits 0: an allowed call gets no answer from ascc, so
Claude Code's own permission rules and prompts still apply. ascc only ever tightens.

Then in a Claude Code session in the repo, ask it to read a file outside the repo. It reports the block with
ascc's reason:

```text
PreToolUse:Read hook error: ascc: no policy permits agent "dev-agent" to use "claude-code/Read" (default deny)
```

| The hook | Exit | Claude Code |
|:---|:-:|:---|
| Policy denies the call | 0, with the deny JSON | Blocks it and shows Claude the reason |
| Policy allows the call | 0, no output | Carries on with its own permission rules |
| ascc can't run: bad payload, registry or policy that won't load, audit log that can't be written | 2, reason on stderr | Blocks it |

## 5. Read and verify the audit log

Every decision is one JSON line: the agent, the tool, the decision, the policy ids, the reason, the parsed
features policy saw, and a SHA-256 of the hook payload. **The payload itself is never stored**, so a command
with a token in it leaves no token in the log.

```bash
ascc audit verify
```

```text
audit: ok, 5 entries, head 43e2a90c2e33511f5783d532a37dba9df20dd9d1c350714135394638961f7f30
audit: the head wasn't checked. Lines cut off the end still verify; record the head somewhere else and pass it back with --expect-head.
```

It exits 1 and names the first line that was edited, removed or reordered. **Lines cut off the end are different:**
a shorter chain is still a valid chain. Write the head hash down somewhere the agent can't reach (a ticket, a
second machine) and check against it:

```bash
ascc audit verify --expect-head 43e2a90c2e33511f5783d532a37dba9df20dd9d1c350714135394638961f7f30
```

`ascc decide` is a dry run and records nothing.

## 6. Stop an agent

```bash
ascc kill dev-agent      # every request from dev-agent is denied from the next tool call
ascc revive dev-agent    # policy applies again
```

Both rewrite `ascc.registry.json` (in standard JSON formatting) and add a line to the audit log. A suspended
agent is denied by ascc itself, before any policy runs. A `retired` agent can't be revived: set that status by
hand when an agent is offboarded.

<a id="limits"></a>

## ⚠️ What this doesn't cover

| Limit | What to do |
|:---|:---|
| **The agent id is asserted, not proven.** It comes from `--agent` in the registration. Anyone who can edit that file can claim another id. | Register the hook in managed settings with `allowManagedHooksOnly`. |
| **Whoever can edit the settings can remove the hook**, or set `disableAllHooks`. | Same: managed settings. A hook in project or user settings is best effort. |
| **The sample policies stop the agent editing ascc's files** (`deny-ascc-state-changes`: the registry, policies, audit log, Claude Code settings, and `ascc kill` or `revive`). They do it by reading the command's text, which can be dodged. | Keep the registry, the policies and the audit log where the agent's user can't write them. |
| **Command features are a heuristic.** `network`, `touches_secret_path` and `git_push_target` come from the command text. An encoded, aliased or downloaded command gets past them. | Treat these policies as a guard against accidents and plain injected lines, not a sandbox. |
| **Files pulled in with `@` mentions** never reach the hook. | Set `CLAUDE_CODE_DISABLE_FILE_MENTIONS=1` (Claude Code 2.1.299 or newer). |
| **`git push` with no branch named** is judged by the branch checked out in the session's working directory. When ascc can't read it (a worktree, a detached head), the target is `unknown` and `deny-push-to-main` doesn't fire. | Protect `main` on the remote as well. |
| **Symlinks aren't resolved.** A link inside the repo that points outside it counts as inside. | Don't rely on `path_in_repo` alone for anything a link could reach. |
| **A hook killed mid-write** leaves `.ascc/audit.jsonl.lock` behind. Calls block for up to 30 seconds, then ascc clears it. | Nothing; it recovers. |

---

<p align="center">
  <a href="../reports/2026-claude-code-hook-contract.md">← Hook contract</a> ·
  <a href="../README.md">Docs home</a> ·
  <a href="../ROADMAP.md">Roadmap →</a>
</p>
