# Gateway run (TKT-001)

A spike, not a product. It runs a real [agentgateway](https://github.com/agentgateway/agentgateway) between a
stub MCP server and a stub `ext_mcp` policy service, and checks the six things the
[gateway report](../../../docs/reports/2026-gateway-build-vs-adopt.md#the-run-2026-11-11) says a run must show
before ADR-04 can change from "build" to "adopt". The results are in that report.

**It is a Go module of its own, on purpose.** gRPC links the network packages and the test starts a process;
the `ascc` binary may do neither, and `go test ./...` at the repo root never builds this folder. Nothing in
`src/cmd` or `src/internal` may import it.

| File | What's inside |
|:---|:---|
| [**upstream.go**](upstream.go) | The smallest MCP server over Streamable HTTP: `read_file`, `delete_repo` and `slow_count`, which holds a stream open for five seconds. It counts the calls that reach it. |
| [**stub.go**](stub.go) | The policy service. Five rules standing in for ASCC's decision point; the comment on `Stub` lists them. |
| [**run_test.go**](run_test.go) | Starts both, starts the gateway twice (with the callout and without), and checks each result. |
| [**extmcp/**](extmcp/) | agentgateway's `ext_mcp` contract and its generated Go code. Not ours; see below. |

## Run it

Download the v1.7.0 release binary for your machine from
[the release page](https://github.com/agentgateway/agentgateway/releases/tag/v1.7.0), check it against the
`.sha256` file published beside it, and from this folder:

```bash
go vet ./...
ASCC_AGENTGATEWAY=/path/to/agentgateway go test -count=1 -v .
```

In PowerShell, set `$env:ASCC_AGENTGATEWAY` first. Without the variable the test skips: this repo doesn't
download a binary behind your back. CI sets it, in a job of its own on Linux, after checking the checksum.

Everything listens on `127.0.0.1` on ports the kernel picks, the gateway included (left alone it listens on
every interface), and its admin and stats listeners are off. The run takes about 13 seconds.

**Each result is pinned to what v1.7.0 does**, the failing one included. If a later release starts cutting an
open stream on a deny, the test fails, and that is the cue to reread the report.

## The `extmcp` folder

`ext_mcp.proto`, `ext_mcp.pb.go` and `ext_mcp_grpc.pb.go` are copied from agentgateway at tag `v1.7.0` (commit
`26f5aebe99a36a037fe54f8080a608aca5547e15`): the proto from `crates/protos/proto/`, the Go files from `api/`.
They are Apache-2.0, copyright the agentgateway authors, and changed only by `gofmt`, which this repo's lint
requires of every Go file. They are copied and not imported because the `api` folder isn't a module that
resolves on its own: asking for it pulls in the whole repository, which needs a newer Go than this one uses.
To move to a later release, copy the three files again and run `gofmt -w extmcp`.
