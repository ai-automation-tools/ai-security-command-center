# Contributing

ASCC decides whether an AI agent may run a tool. A change that lets one call through that policy denies is the
worst bug it can have, so the bar is the same for every change: it fails closed, a test proves it, and nothing
raw reaches the audit log or an error message.

A security problem goes through [SECURITY.md](SECURITY.md), never a public issue or pull request.

## Run the checks

You need Go 1.26 or newer. `node` on `PATH` is optional locally: without it the demo's page test is skipped,
and CI runs it.

From the repo root, in PowerShell or bash:

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go build ./...
```

`go test ./...` also checks the rules of the development stack's Compose file. That test needs `docker compose`
and is skipped without it. The stack itself is optional, for work on the platform services:
[`deploy/compose/README.md`](deploy/compose/README.md) says how to start it and where its secrets go (`.env`,
which git ignores).

All four must pass before a pull request opens. CI runs them on Linux and Windows, plus a WebAssembly build of
the core packages, so use `path/filepath` and never a hard-coded separator.

## Add a policy, with its allow and deny fixtures

Every policy rule ships with two cases: a request the rule decides, and its nearest opposite. Both are rows in
the `TestDecide` table; a case the README or the CLI test runs is a file in `examples/requests/` too. A rule
with one case isn't done.

1. **Check the context carries what the rule needs.** The attributes a policy can read are in
   [`src/internal/pdp/schema.cedar`](src/internal/pdp/schema.cedar). A new one is added there first, then to
   the command parser in `src/internal/claudecode` with a table-test row for `Bash` and one for `PowerShell`,
   and only then used in a policy.
2. **Register the tool.** A tool that isn't under `tools` in
   [`examples/ascc.registry.json`](examples/ascc.registry.json) is denied before Cedar runs, MCP tools
   included (`claude-code/mcp__<server>__<tool>`).
3. **Write the rule** in [`examples/policies/allow.cedar`](examples/policies/allow.cedar) or
   [`deny.cedar`](examples/policies/deny.cedar), with a comment saying why it exists:

   ```cedar
   @id("deny-network-with-secret-path")
   @reason("a shell command may not send a secret file over the network")
   forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
   when { context.network && context.touches_secret_path };
   ```

   `@id` is required and unique across every file (`allow-...` or `deny-...`, kebab-case). `@reason` is the one
   line the agent sees and the audit log keeps: no jargon, no secret. Guard an optional attribute with `has`
   (`context has git_push_target && ...`). Cedar skips a policy that errors, which would let a `forbid` fail
   open, so ascc refuses to load one that reads an optional attribute without `has`, and every call is blocked
   until it is fixed.
4. **Write the two cases,** one that the rule allows or denies and one just outside it. As a file in
   [`examples/requests/`](examples/requests/) a case looks like this:

   ```json
   {
     "principal": "dev-agent",
     "action": "runtime.tool_use",
     "resource": "claude-code/Bash",
     "context": { "path_in_repo": true, "network": true, "touches_secret_path": true, "touches_ascc_state": false }
   }
   ```

   Fictional agents, repos and hosts only (`example.com`, `paste.example`). Never a real hook payload, token,
   name or local path.
5. **Add the test rows.** Both cases go in the `TestDecide` table in
   [`src/internal/pdp/pdp_test.go`](src/internal/pdp/pdp_test.go), asserting the decision and the determining
   policy ids. A request file can also go in `TestDecideOnTheSamples` in
   [`src/cmd/ascc/main_test.go`](src/cmd/ascc/main_test.go) so the CLI runs it, but only one that exactly one
   policy decides: that test expects a single policy id, so a default deny doesn't belong there.
6. **Run it by hand:**

   ```bash
   go build -o bin/ ./src/cmd/ascc
   cd examples
   ../bin/ascc decide --request requests/<fixture>.json
   ```

   `policy_ids` holds the policies that decided. A deny with none is the default deny: no permit matched.
7. **Run the four checks,** then fix any doc that quotes the policies, the tool list or the output
   (`README.md`, `docs/guides/claude-code-setup.md`).

## What a policy can't do

- **Grant on a score or a model's opinion.** Policy reads registry facts and parsed features, nothing else.
- **Stand in for the kill switch.** A suspended or retired agent is denied before any policy runs. Don't move
  that into a `.cedar` file.
- **Promise more than the parser sees.** A rule on `network` or `touches_secret_path` judges command text, and
  command text can be dodged. If your rule has a known way round it, add it to the
  [limits table](docs/guides/claude-code-setup.md#limits) in the setup guide, and to the README's
  [coverage section](README.md#-coverage-what-the-hook-sees-and-what-it-doesnt) if it changes the summary.

## Pull requests

- Branch from `main`; never push to it.
- Commit messages are plain imperative sentences: "Add the decide command". No `feat:` prefixes.
- A new dependency needs a line in the PR body saying why. One that links `net`, `net/http`, `crypto/tls` or
  `os/exec` into the `ascc` binary can't be added: a test fails on it.
- Add a line to [`docs/CHANGELOG.md`](docs/CHANGELOG.md) for anything a user would notice.

The full rulebook, with every invariant, is [AGENTS.md](AGENTS.md).
