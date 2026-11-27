---
name: ascc-cedar-policy
description: Add, change or debug a Cedar policy in ASCC, together with the schema, registry and fixtures it depends on. Use when writing a permit or forbid, adding a tool or agent group, adding a context or entity attribute, when ascc refuses to load a policy, or when a decision or its reason is wrong.
---

# Cedar policies in ASCC

## Where things live

| Thing | File |
|:---|:---|
| Built-in schema (the source of truth) | `src/internal/pdp/schema.cedar` |
| Loader, validator, `Decide` | `src/internal/pdp/pdp.go` |
| Registry format | `src/internal/pdp/registry.go`, sample in `examples/ascc.registry.json` |
| Sample policies | `examples/policies/allow.cedar`, `examples/policies/deny.cedar` |
| Request fixtures | `examples/requests/*.json` |
| Tests | `src/internal/pdp/pdp_test.go`, `src/cmd/ascc/main_test.go` |
| Why cedar-go behaves as it does | `docs/reports/2026-cedar-go-check.md` |

## Shape of a policy

```cedar
@id("deny-network-with-secret-path")
@reason("a shell command may not send a secret file over the network")
forbid (principal, action == Action::"runtime.tool_use", resource in ToolGroup::"shell")
when { context.network && context.touches_secret_path };
```

- `@id` is required and unique across every file; the loader rejects a missing or duplicate one. Use
  `allow-...` or `deny-...` in kebab-case.
- `@reason` is the sentence shown to Claude and written to the audit log. One line, no jargon, no secret.
- The only action is `Action::"runtime.tool_use"`.

## Steps to add a rule

1. **Does the context already carry what you need?** If not, stop and use the `ascc-hook-feature` skill first: the
   attribute goes into `schema.cedar`, then the parser, then here.
2. **New tool or group?** Add the tool to `tools` in the sample registry with its groups. A tool that isn't
   listed is denied before Cedar runs, MCP tools included (`claude-code/mcp__<server>__<tool>`).
3. **Write the policy** in `allow.cedar` or `deny.cedar`, with a comment saying why it exists.
4. **Add two fixtures** in `examples/requests/`: one the rule decides, and its nearest opposite. Fictional
   agents and hosts only.
5. **Add the test rows** to the `TestDecide` table (and `TestDecideOnTheSamples` if the README shows the run).
   Assert the decision and the determining policy ids.
6. **Run it by hand:**
   ```
   go build -o bin/ ./src/cmd/ascc
   cd examples
   ../bin/ascc decide --request requests/<fixture>.json
   ```
7. **Run the four checks** in `AGENTS.md`, then fix any doc that quotes the policies, the tool list or output.

## Traps

| Trap | What to do |
|:---|:---|
| **A `forbid` that errors is skipped, so it fails open.** | Guard optional attributes: `context has git_push_target && context.git_push_target == "main"`. ascc also denies on any `Diagnostic.Errors` entry; keep both. |
| **An entity Cedar doesn't know makes `in` false, silently.** | ascc denies unknown agents and tools before evaluating. Don't weaken that to make a policy "work". |
| **The validator is experimental** (`x/exp/schema/validate`). | Call it only through `newValidator` and `checkPolicy` in `pdp.go`. |
| **Putting the kill switch in policy.** | Never. `Decide` denies suspended and retired agents before the engine. |
| **A permit on `principal` with no condition.** | Almost always too wide. Scope by tool group and a context condition. |
| **Policy on a score or a model's opinion.** | Not allowed (ADR-14). Policy reads registry facts and parsed features only. |
| **Promising more than the parser sees.** | A rule on `network` or `touches_secret_path` judges command text. Say so in the reason's surrounding docs, not in the reason. |

## Debugging a decision

- `policy_ids` in the output are the determining policies. Empty with a deny means default deny: no permit
  matched. Check the tool's groups and the context values in the request.
- "ascc couldn't load" on exit 2 is a registry typo, a missing `@id`, or an attribute the schema lacks. The
  message names the file and the id.
- To see what the hook would send for a real call, feed a payload from `examples/hook-payloads/` to
  `ascc hook claude-code --agent dev-agent --audit <temp file>`; the audit line holds the parsed features.
