---
name: hook-feature
description: Add or change a parsed command feature in ASCC's Claude Code hook (the context attributes path_in_repo, network, touches_secret_path, touches_ascc_state, git_push_target, or a new one), or change how a tool's payload is mapped to a request. Use when the hook misjudges a command, when a policy needs a fact the context doesn't carry, or when a new Claude Code tool needs mapping.
---

# Changing what the hook sees

A feature is a fact about a tool call that Cedar can read. It crosses four layers, and they change in this order.

## Order of work

1. **Schema.** Declare the attribute in the `context` record of `src/internal/pdp/schema.cedar`. Required
   attributes are set on every call; mark one optional (`name?: Type`) only when it has no honest default, as
   `git_push_target` does, and then every policy must guard it with `has`.
2. **Parser.** Set it in `Map` in `src/internal/claudecode/claudecode.go`. This package does no file, network
   or clock access: anything the parser can't know from the payload arrives through `Options` (as `Branch` and
   `Protected` do), read by `src/cmd/ascc`.
3. **Table test.** Add rows to `TestMapFeatures` in `claudecode_test.go`. The `want` string is
   `"in_repo network secret state [push target]"`; extend that helper if you add a feature. Every behaviour
   gets a row for `Bash` **and** for `PowerShell`, plus at least one row that must stay false.
4. **Fixtures.** Add the attribute to every file in `examples/requests/` (the PDP rejects a context that
   doesn't fit the schema) and a payload in `examples/hook-payloads/` if the README or guide will show it.
5. **Policy.** If a sample policy should use it, follow the `cedar-policy` skill.
6. **Audit.** The audit line stores parsed features. Confirm the new one holds no raw input: a Bool or a short
   normalised string, never a command, a path's contents or a token. `TestAuditHoldsNoRawInput` guards this.
7. **Docs.** Update the feature list in `AGENTS.md`, and add what the feature can't see to the limits table in
   `docs/guides/claude-code-setup.md`.
8. **Checks.** The four commands in `AGENTS.md`, then `GOOS=js GOARCH=wasm go build ./src/internal/...`.

## Parser rules

- **Normalise before comparing.** Backslashes to forward slashes, lower case, `..` resolved (`norm`). Windows
  payloads arrive in any case with either separator.
- **Compare folders with `under`,** not `strings.HasPrefix`: `/home/dev/repo-evil` is not under
  `/home/dev/repo`.
- **Split on shell separators first** (`&&`, `||`, `;`, `|`, `&`, newline), then tokenise each segment. A
  feature must hold for a command that comes after another one.
- **Match the program, not the token:** `program()` strips the folder and `.exe`.
- **When in doubt, set the flag.** A false positive costs a prompt; a false negative costs an allow. But a
  feature that fires on ordinary work gets the hook switched off, so test the harmless neighbours too.
- **Unknown is a value.** If the parser can't tell (no branch readable), emit `"unknown"` and say in the guide
  what that means for policy. Don't guess.
- **A tool ascc can't see inside is the worst case.** MCP tools and `WebFetch` get `network = true`.
- **Any parse error returns an error,** and the caller blocks the call. Never return a zero-value context.

## It is a heuristic, and the docs say so

Command text can be encoded, aliased or downloaded past any pattern. A feature narrows accidents and plain
injected lines; it is not a sandbox. Never write a reason, a README line or a coverage label that implies
otherwise. For a gap worth knowing, add a row to the guide's limits table with what the reader should do
instead.

To look for gaps on purpose, run the `bypass-hunter` agent on the feature.
