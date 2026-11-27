---
description: Run the four verify commands plus the WebAssembly build of the core packages
allowed-tools: Bash, PowerShell
---

Run these from the repo root, in order, and report each as pass or fail with the failing output:

1. `gofmt -l .` (pass only if it prints nothing)
2. `go vet ./...`
3. `go test ./...`
4. `go build ./...`
5. The core and the demo for WebAssembly, as CI does: `go build ./src/internal/... ./src/cmd/ascc-wasm` with
   `GOOS=js` and `GOARCH=wasm` set for that one command only. Don't leave them set in the shell.

6. The two spikes, each a module of its own: `go vet ./...` and `go test ./...` run inside
   `src/spikes/pluginhost` and inside `src/spikes/gatewayrun`. The gateway run's test skips unless
   `ASCC_AGENTGATEWAY` names an agentgateway binary; report that skip.

`go test ./...` also runs the development stack's rule test (`TestComposeFileKeepsItsRules` in
`deploy/compose`), which needs `docker compose` and is skipped without it; report that skip too. It doesn't
start the stack: `ASCC_COMPOSE_SMOKE=1 go test -count=1 ./deploy/compose` does, and takes about a minute.

`go test ./...` includes the demo's page test (`TestBuiltPageRendersWithNoNetwork`), which needs `node` on
`PATH`. Without it the test is skipped locally and fails in CI; report a skip as a skip, not a pass.
`TestBuiltPageInABrowser` walks the same page in headless Chromium and skips unless `ASCC_BROWSER_TEST` is
set (setup in `src/cmd/buildsite/testdata/browser/README.md`); CI runs it in its `browser` job.

Don't fix anything unless asked: this command reports. If a test fails, name the test, show its message, and
say which invariant in `AGENTS.md` it guards (the `ascc-fail-closed-review` skill maps tests to invariants).

$ARGUMENTS
