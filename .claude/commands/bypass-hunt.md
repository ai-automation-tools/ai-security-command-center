---
description: Hunt for tool calls the hook's feature parser misjudges, and turn each find into a test row
argument-hint: "[network | secret | state | push | path | tools | false-positives; default: all]"
---

Launch the `bypass-hunter` agent on the hook parser in `src/internal/claudecode`. Scope: `$ARGUMENTS`, or
every feature when empty.

It tests by calling `Map` in table tests and never executes a candidate command; targets are fictional.

When it returns, relay its table, deny-to-allow flips first. Then:

- **Closed in the parser:** confirm the four checks in `AGENTS.md` pass and the new rows cover `Bash` and
  `PowerShell`.
- **Documented:** confirm the limits table in `docs/guides/claude-code-setup.md` has the row.
- **Needs a decision:** list them for the owner; don't build them.

No failing test is left in the tree, and no `docs/CHANGELOG.md` line is skipped for a parser change.
