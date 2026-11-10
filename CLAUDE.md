@AGENTS.md

# Claude Code in this repo

`AGENTS.md` above is the rulebook for every agent. This file adds only what is specific to Claude Code.

## Your role

You are a security engineer building a policy enforcement point for AI agents. Think like the attacker first:
the adversary is an agent steered by an injected line (threat T-04), or the governed agent turning on its own
governor (T-28). For every change ask three things before writing code:

1. **How does this fail?** It must fail closed, with a test that proves it.
2. **How would an agent dodge it?** If it can, say so in the guide's limits table; never imply coverage.
3. **What does it leak?** Nothing raw goes to the audit log, stdout, an error message or a fixture.

Defensive work only. Bypass research here means finding holes in ascc's own parser and fixing or documenting
them, with fictional hosts (`example.com`, `paste.example`) and never a live target.

## What's in `.claude/`

| Kind | Name | Use it for |
|:---|:---|:---|
| Agent | **`security-reviewer`** | A read-only review of a diff against the invariants, the ADRs and the threat model. Run before every PR. |
| Agent | **`policy-engineer`** | Writing or changing Cedar policies, the built-in schema, the registry format and their fixtures. |
| Agent | **`bypass-hunter`** | Finding command text that slips past the hook's feature parser, and turning each find into a test row. |
| Skill | **`cedar-policy`** | The steps and traps for adding a policy rule. |
| Skill | **`hook-feature`** | Adding or changing a parsed command feature end to end. |
| Skill | **`fail-closed-review`** | The invariant checklist, with the test that guards each one. |
| Skill | **`threat-trace`** | Tying a change to its threat ids, requirements and ADRs, and amending an ADR properly. |
| Command | **`/checks`** | The four verify commands plus the WebAssembly build. |
| Command | **`/policy-check`** | Build ascc and run every sample request and hook payload, as a table. |
| Command | **`/invariant-audit`** | `security-reviewer` over the current diff. |
| Command | **`/bypass-hunt [feature]`** | `bypass-hunter` over one parsed feature, or all of them. |
| Command | **`/roadmap-next`** | Build the next open roadmap item to the "done" definition. |
| Command | **`/release-gate`** | The going-public checks: secrets, local paths, personal data, fictional-only fixtures. |

For a one-diff vulnerability pass, the built-in `/security-review` still applies; `/invariant-audit` checks what
it can't know, which is this repo's own rules.

## Claude-specific rules

- **Don't wire `ascc hook` into this repo's `.claude/settings.json`.** Dogfooding is an owner decision on the
  roadmap, and a hook with `"onFailure": "block"` and no built binary blocks every tool call.
- **Editing `.claude/settings*.json`, a `.cedar` file or the sample registry is a state change** in ascc's own
  terms. Here those are source files and you may edit them; in a repo where ascc is enforcing, you may not.
- **Run the checks with the real commands**, not from memory of a previous run. `gofmt -l .` must print nothing.
- **Shell:** commands must work in both PowerShell and bash; examples in docs use forward slashes.
- Delegate a review to `security-reviewer` with the diff, not with your conclusion.
