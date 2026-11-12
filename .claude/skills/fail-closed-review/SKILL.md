---
name: fail-closed-review
description: Check a change in ASCC against its security invariants (default deny, fail closed, no secret at rest, zero network, the audit chain, the kill switch, self-protection, honest coverage), each with the test that guards it and a command to confirm. Use before opening a PR, when reviewing a diff, when adding an error path, a dependency or a command, and when a test in this list fails.
---

# Fail-closed review

Each row is an invariant from `AGENTS.md`, the test that guards it, and what to check in a diff. If a change
weakens a row, the fix is in the code, never in the test.

## The invariants

| Invariant | Guarded by | Check in the diff |
|:---|:---|:---|
| **Default deny.** No permit, unknown agent, unknown tool: deny with a reason. | `TestDecide`, `TestDecideFailsClosed` | Every new branch in `Decide` ends in an explicit decision. No early `return` with a zero `Decision`. |
| **Load fails closed.** Bad registry, missing `@id`, duplicate id, undeclared attribute: ascc won't start. | `TestLoadFailsClosed`, `TestParseRequestRejectsUnknownFields` | New registry or request fields are parsed strictly (`strictUnmarshal`), not ignored. |
| **A Cedar evaluation error is a deny.** | `TestEvaluationErrorIsDeny` (R8, R9) | `Diagnostic.Errors` is still read. A new optional attribute is guarded with `has` in every policy. |
| **A panic is a deny.** | `TestDecideFailsClosed`, `TestHookFailsClosed` | `Decide` and `run` keep their named results and `recover`. |
| **Hook contract.** Deny is exit 0 with the `hookSpecificOutput` JSON; an internal error is stderr and exit 2, never 1; allow prints nothing. | `TestHookBlocksAndPasses`, `TestHookFailsClosed` | No new `os.Exit(1)`, no output on allow, no `"permissionDecision": "allow"`. |
| **A call that can't be recorded doesn't happen.** | `TestHookBlocksWhenTheLogCantBeWritten` | The append happens before the answer is written; its error is exit 2. |
| **One chain under parallel hooks, and no kill lost to another.** | `TestParallelHooksKeepOneChain`, `TestParallelKillsAllLand` | Locking, appending and registry rewrites stay in `src/cmd/ascc/store.go`. A registry rewrite holds the audit log's lock from its read to its audit line. |
| **The chain names its first bad line; truncation changes the head.** | `TestVerifyNamesTheFirstBadLine`, `TestTruncationChangesTheHead`, `TestAuditVerifyCommand` | A new `audit.Entry` field is inside the hashed bytes. Field order and encoding stay stable. |
| **No secret at rest.** The log holds a SHA-256 of the input, not the input. | `TestAuditHoldsNoRawInput` | No new field carries a command, a path's contents, an env value or a raw payload. Same for error strings. |
| **Zero network.** | `TestBinaryCantOpenASocket` | No import of `net`, `net/http`, `crypto/tls` or `os/exec`, directly or through a dependency. |
| **Core packages do no I/O** and build for WebAssembly. | `TestPackageDoesNoIO`, the CI wasm step | `pdp`, `audit`, `claudecode` and `demo` take bytes and return values. No `os`, no `time.Now`. |
| **The kill switch is not a policy.** | `TestKillAndRevive`, `TestRetiredAgentIsDenied` | The status check is in `Decide`, before the engine. `revive` still refuses a retired agent. |
| **An agent can't switch ascc off.** | `TestAgentCantChangeAsccState` | A new state file or state-changing command is covered by `touches_ascc_state`. |
| **Coverage is computed from evidence.** | `TestCoverageIsComputedFromEvidence`, `TestInventoryLabelsComeFromEvidence`, `TestCheckPosture`, `TestPostureCommand` | No label is read from the registry. No evidence means `unmonitored`; a settings file ascc can't read fails the posture check. A new posture condition only ever removes a pass. |
| **Discovery reads names, never values**, and says what it couldn't read. | `TestDiscover`, `TestDiscoverCommand`, `TestDiscoverSaysWhatItCouldntRead` | A new field read from a config file is a key or a name, never a value, and has a planted value in the test. An unreadable file reaches `unreadable`. |
| **Identities are declared or imported, never trusted.** | `TestIdentitiesFailClosed`, `TestInventoryImport`, `TestInventoryImportCommand`, `TestNHIFindingRules` | A new identity field is checked in `checkIdentity` against the suite schema and has a bad-value row. An imported `coverage:`, `imported-from:` or `used-by:local:agent/` tag is dropped. No error repeats a value from the entry. |
| **The demo is the CLI's code and makes no network call.** | `TestBuiltPageRendersWithNoNetwork`, `TestBuiltPageInABrowser` (Chromium, with `ASCC_BROWSER_TEST` set), `TestHookFailsClosed` and `TestPackageDoesNoIO` in `src/internal/demo` | `src/internal/demo` adds no decision logic. A new page feature has a step in `src/cmd/buildsite/testdata/render.js` and in `testdata/browser/page.test.mjs`. Nothing in `site/` loads from another host or is an inline script, and anything drawn from fixture, policy or log text goes through `esc`. |
| **Policy decides, nothing else** (ADR-14). | review | No score, model call or heuristic sits between the request and the decision. |

## Confirm

```
gofmt -l .                                         # prints nothing
go vet ./...
go test ./...
go build ./...
GOOS=js GOARCH=wasm go build ./src/internal/... ./src/cmd/ascc-wasm    # PowerShell: $env:GOOS='js'; $env:GOARCH='wasm'; go build ./src/internal/... ./src/cmd/ascc-wasm; then clear both
go list -deps ./src/cmd/ascc                       # scan for net, net/http, crypto/tls, os/exec
```

## Questions to ask of any new error path

1. What does the caller see: a deny with a reason, or exit 2 with one line on stderr? Anything else is a bug.
2. Is the message one line a person can act on, with no stack trace and no echoed input?
3. Is there a test that takes this exact path? "Every fail-closed path has a test" is a rule, not a hope.
4. If this path fires on every call, does the session stall with a clear reason, or silently?

## New dependency

Stop unless it is unavoidable. It needs a line in the PR body saying why, must not link a network or exec
package, must build for `js/wasm` if a core package imports it, and must carry a licence compatible with MIT.
