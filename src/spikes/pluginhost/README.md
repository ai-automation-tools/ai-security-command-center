# Plugin host prototype (TKT-031)

A spike, not a product. It proves the plugin model in ADR-15: a connector runs as its own process, reaches its
platform only through the host, and is refused when it calls outside what was approved. What it found is in
[the report](../../../docs/reports/2026-plugin-contract-spike.md).

**It is a Go module of its own, on purpose.** gRPC links the network packages and the host starts processes;
the `ascc` binary may do neither, and `go test ./...` at the repo root never builds this folder. Nothing in
`src/cmd` or `src/internal` may import it.

| Folder | What's inside |
|:---|:---|
| [**proto/**](proto/plugin.proto) | The contract, `ascc.plugin.v1alpha1`. Edit this, then regenerate. |
| [**pluginv1alpha1/**](pluginv1alpha1/) | Generated Go code. Don't edit. |
| [**CONTRACT.md**](CONTRACT.md) | Generated reference. Don't edit. |
| [**host/**](host/) | The host: starts a plugin, mints its capability token, checks every call, writes every event. |
| [**sdk/**](sdk/) | What a connector imports. |
| [**cmd/hello-connector/**](cmd/hello-connector/) | The smallest connector. |

## Run it

From this folder:

```bash
go vet ./...
go test ./...
go test -run '^$' -bench HostRoundTrip ./host    # the NFR-20 timing
```

The tests build the hello connector and run it as a child process.

## Regenerate after changing the proto

With `protoc` 36, `protoc-gen-go`, `protoc-gen-go-grpc` and `protoc-gen-doc` on `PATH`:

```bash
protoc -I proto --go_out=pluginv1alpha1 --go_opt=paths=source_relative --go-grpc_out=pluginv1alpha1 --go-grpc_opt=paths=source_relative --doc_out=. --doc_opt=markdown,CONTRACT.md proto/plugin.proto
```

Field numbers are never reused. A field that goes is marked `reserved`.
