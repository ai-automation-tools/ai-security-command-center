<h1 align="center">🧩 Plugin contract and host prototype (TKT-031)</h1>

<p align="center">
  <em>Can a plugin run as its own process, do its job through the host alone, and be refused the moment it asks for more than was approved?</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/verdict-the_model_holds;_a_laptop_plugin_isn't_confined-F59E0B?style=for-the-badge" alt="Verdict: the model holds; a plugin run as a laptop process isn't confined">
  <img src="https://img.shields.io/badge/checked-2026--11--10-6B7280?style=for-the-badge" alt="Checked 2026-11-10">
  <a href="../README.md"><img src="https://img.shields.io/badge/↩-docs_home-6B7280?style=for-the-badge" alt="Back to docs"></a>
</p>

---

**Verdict: the model in [ADR-15](../product/plan/e2-adrs.md#adr-15) works.** A connector built on the contract
runs as a separate process under a prototype host, reads a platform and reports identities without holding a
credential, and is refused and recorded when it calls outside its grant. **One thing ADR-15 assumes doesn't
hold on a laptop:** a plugin run as a plain process can open any connection its user can, so the host there
checks what the plugin *asks* for and can't limit what it *does*. A container gives that limit; a process
doesn't.

The code is [`src/spikes/pluginhost/`](../../src/spikes/pluginhost/), a Go module of its own. It is a
prototype: about 650 lines, no install gate, no restart, no manifest. ADR-15 stays open on decision K7, which
is the owner's.

## What was built

| Piece | Where | What it is |
|:--|:--|:--|
| **Contract `v1alpha1`** | [`proto/plugin.proto`](../../src/spikes/pluginhost/proto/plugin.proto) | Three gRPC services. `Plugin` (`Describe`, `Health`) and `Connector` (`Sync`) are served by the plugin. `Host` (`UpsertNhi`, `ReportFinding`, `PlatformCall`) is served by the host. |
| **Reference** | [`CONTRACT.md`](../../src/spikes/pluginhost/CONTRACT.md) | Generated from the proto's comments by `protoc-gen-doc`. |
| **Host** | [`host/`](../../src/spikes/pluginhost/host/) | Starts the plugin, mints its capability token, serves `Host` to that one plugin, checks every call and writes every event. |
| **Go SDK skeleton** | [`sdk/`](../../src/spikes/pluginhost/sdk/) | `sdk.Serve(connector)`: a connector implements four methods and never touches a socket or the token. |
| **Hello connector** | [`cmd/hello-connector/`](../../src/spikes/pluginhost/cmd/hello-connector/) | 60 lines. Lists the apps of a fictional platform through the host, reports each one, and reports a finding for the one with no owner. |

## The ticket's acceptance test

`go test ./...` in the spike folder builds the hello connector, runs it as a child process and checks:

| Claim | Test | Result |
|:--|:--|:--|
| A hello-world connector runs under the host | `TestHelloConnectorRunsUnderTheHost` | Two identities reported, one finding, health answered |
| It is refused an out-of-scope call | `TestOutOfScopeCallIsRefused` | Without `findings:write@self` the finding is refused, recorded as `plugin.scope_denied`, and the identities still arrive |
| | `TestPlatformCallOutsideTheGrantNeverReachesTheBroker` | A host or credential kind that wasn't approved is refused before any platform call is made |
| Its events carry `plugin_id` | `TestHelloConnectorRunsUnderTheHost` | Every event has the plugin id, the connector id and the contract version, set by the host |

It was run on Windows 11 with Go 1.26, and CI ran it on Linux and Windows on the pull request that added it.

## What fails closed

Each of these has a test in `host/host_test.go`; the refusal is the default, not a special case.

| Input | Outcome |
|:--|:--|
| No token, two tokens, garbage, `alg: none`, another token type with a good signature, a token signed by another key, claims swapped under a good signature | Refused, `plugin.token_rejected` |
| The plugin's own token, before it has described itself as the approved plugin | Refused: the token comes into force only once `Describe` matches |
| Another plugin's token, or another instance of the same plugin's | Refused: each plugin has its own endpoint and only its own token works there |
| Expired, not yet valid, another audience, another contract version | Refused |
| A token without the scope the method needs | Refused, `plugin.scope_denied` naming the scope |
| A method the host hasn't mapped to a scope, and any streaming call | Refused: a new call is closed until someone decides what it needs |
| A platform call that isn't `GET`, isn't `https`, has no host, names another host or port, carries a user part, or hides the approved host as a prefix | Refused. The broker is given the URL as it was checked, not as it was sent |
| A plugin that describes itself as a different plugin, another type or another contract version | Stopped, with its token never in force and the socket folder removed |
| A panic while serving a plugin's call | That call fails; the host and its other plugins carry on |
| A plugin binary that isn't there | An error, and nothing recorded as started |
| The plugin's process dies | Its token stops working, `plugin.exited` is recorded, and the host runs the next plugin |
| `Stop` | The token is revoked first, then the process ended and the socket folder removed |

No error or event repeats a token (`TestNoEventOrErrorRepeatsTheToken`), and an event's detail is a method, a
scope or a reason, never content a plugin sent.

## What the prototype taught

1. **The contract needs both directions.** The host must call the plugin (`Describe`, `Sync`) and the plugin
   must call the host (to report, and to reach its platform). The prototype uses two Unix sockets in a private
   folder: one the plugin serves, one the host serves **to that plugin only.** A per-plugin endpoint means a
   token can't be replayed on another plugin's socket.
2. **Who is asking comes from the token, never from the message.** No message in the contract has a
   `plugin_id` or `connector_id` field for a plugin to fill in, so there is nothing to forge. The host stamps
   both on what it stores and records.
3. **No call returns the host's credential, but a platform's answer can be a secret.** Nothing in the contract
   hands a plugin the credential the host uses. `PlatformCall` does return the platform's answer whole, and
   the prototype checks the host of the URL, not the path. Reading an OpenBao KV value is a `GET`, so a
   connector approved for that host can read whatever its platform credential can. FR-PLG-06 therefore rests
   on the credential's own scope until the grant carries path limits (TKT-032).
4. **The plugin's environment is built, not inherited.** It gets two socket paths (and `SystemRoot` on Windows,
   which sockets need). The token arrives on standard input, not in the environment, where another process of
   the same user could read it.
5. **A process is not a sandbox.** The host refuses a `PlatformCall` to a host that wasn't approved, but
   nothing stops the plugin opening its own connection, reading the user's files or starting another program.
   ADR-15's "confines egress to the manifest's hosts" needs a container or an OS sandbox under it. For a laptop
   that means running plugins in a container there too, or an OS mechanism per platform; that is a new cost
   ADR-15 doesn't count.
6. **A one-hour token with no renewal ends a long-running plugin.** The prototype has no renew call. The host
   either renews before expiry or restarts the plugin; pick one in TKT-032.
7. **Unix sockets work on Windows** through Go's standard library, so no loopback TCP port is needed and no
   other user can connect. On Windows the folder's protection is the temp folder's access list, not the mode
   bits the code sets.

## go-plugin or our own host

ADR-15 asks whether to depend on [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) for the
handshake and health checks. **Recommendation: write the small host.**

| | go-plugin | Own host |
|:--|:--|:--|
| **Starts a plugin and connects** | Yes, with a handshake line on standard output | About 60 lines here |
| **Calls in both directions** | Through its broker, an extra protocol every non-Go SDK must implement | Two plain gRPC endpoints; any gRPC library works |
| **Plugins in containers** | It starts child processes; attaching to a container is a side path | The contract is plain gRPC, so the same plugin runs either way |
| **Licence and maintenance** | MPL-2.0, v1.8.0 in 2026-04, active | Ours to maintain |

go-plugin was not prototyped, so this compares its documented design with code that runs. The deciding point
is the second row: FR-PLG-08 wants Python and TypeScript SDKs on the same contract.

## Overhead (NFR-20)

`BenchmarkHostRoundTrip` measures one `UpsertNhi` from a client to the host over the socket: gRPC, the token
check and the event. On one machine (Ryzen 5 3600, Windows 11), 5,000 calls: **mean 0.34 ms, p95 1.0 ms**,
against NFR-20's 2 ms p95. One machine, one call type, no load; it says the design isn't far off, no more.

## Connector scopes ([V], recorded for TKT-036 and TKT-037)

| Platform | Read | What it needs | Checked |
|:--|:--|:--|:--|
| **GitHub** | App installations and their permissions | Organization "Administration" read | Documentation, 2026-11-10 |
| | Deploy keys | Repository "Administration" read | Documentation |
| | Actions secret names | Organization and repository "Secrets" read | Documentation |
| | Organization PATs and requests | A **GitHub App** with "Personal access tokens" read; a PAT can't call these | Documentation |
| | The PAT policy setting | No read endpoint found | Not readable |
| **OpenBao** | KV v2 paths, AppRoles, transit keys | The list and read endpoints exist in OpenBao's API | Documentation; the policy paths aren't written or tested |
| | Leases | `sys/leases/lookup`; **listing needs `sudo` on that path** | Documentation |

Two things for the connector tickets. The GitHub connector authenticates as an App, so `credential_kinds` for
it is `github_app` alone. And the OpenBao connector's "read-only" policy includes a `sudo` capability, on the
lease-lookup path only; the scope list has to say that plainly, because an approver reading "read-only" won't
expect it.

## Not covered

- **Confinement:** no network, file, CPU or memory limit on the plugin.
- **The install gate:** no manifest, signature or approval. The grant is handed to the host in code.
- **Lifecycle:** no heartbeat, restart or token renewal; `Stop` is the first step of `disable_plugin`, not all
  of it. A call already past the check when `Stop` runs still finishes, `Stop` ends the plugin's process but
  not one it started, and `Health` and `Sync` have no deadline but the caller's.
- **Other processes of the same user.** One can reach a plugin's host socket and, with no token, make the host
  record refusals under that plugin's id; nothing caps them. One can also call the plugin's own socket, which
  has no authentication. Neither gets a call allowed.
- **Rate limits:** a plugin can call its platform, and fill the event record, as fast as it likes.
- **Path limits on platform calls,** as finding 3 says.
- **The real broker and the management API:** platform calls go to a function the test supplies, and reported
  identities are counted, not stored.
- **Other SDKs, other plugin types, streaming,** and a second machine for the timing.

---

<p align="center">
  <a href="2026-proof-of-possession.md">← Proof of possession</a> ·
  <a href="../README.md">Docs home</a> ·
  <a href="../ROADMAP.md">Roadmap →</a>
</p>
