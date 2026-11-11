<h1 align="center">🧱 The ASCC development stack</h1>

<p align="center">
  <em>One command starts the services the platform is being built on, on your own machine.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-development_only-DC2626?style=for-the-badge" alt="Status: development only">
  <a href="../../docs/ROADMAP.md"><img src="https://img.shields.io/badge/plan-ROADMAP-8B5CF6?style=for-the-badge" alt="Roadmap"></a>
  <a href="../../README.md"><img src="https://img.shields.io/badge/↩-repository_root-6B7280?style=for-the-badge" alt="Repository root"></a>
</p>

---

This is the skeleton the next platform slices plug into: the store, the identity provider, the secrets
backend and the trace viewer the plan names, plus the decision point and a placeholder for the egress proxy.
Most of it is empty on purpose. The `ascc` CLI needs none of it.

**Running a service here doesn't settle the decision to use it.** Postgres (ADR-05), OpenBao (ADR-07) and
Envoy (ADR-08) are still marked Spike in the [ADR table](../../docs/product/plan/e2-adrs.md), waiting for the
owner's sign-off. Nothing in ASCC depends on any of them yet.

> [!WARNING]
> **Never run this anywhere that holds a real secret or faces a network.** It speaks plain HTTP, Keycloak runs
> in development mode, and OpenBao's unseal key and root token sit in the same volume as its data. It is safe
> on one developer's machine because every port is bound to `127.0.0.1` and nothing in it is real.

## Run it

You need Docker Engine 28 or newer, with Compose v2. On Linux, older engines let another machine on the same
network segment reach a port that was published on loopback only. From the repository root:

```bash
docker compose -f deploy/compose/compose.yaml up -d --build --wait
```

The first run downloads about 1 GB of images and builds `ascc-pdp`; after that it takes about a minute, most
of it Keycloak. `--wait` returns when Postgres, Keycloak and OpenBao report healthy, which for OpenBao means unsealed with
both engines mounted.

```bash
docker compose -f deploy/compose/compose.yaml down        # stop, keep the data
docker compose -f deploy/compose/compose.yaml down -v     # stop and delete every volume, secrets included
```

## What you get

| Service | On your machine | What is in it |
|:---|:---|:---|
| **`pdp`** | `http://127.0.0.1:8181` | [`ascc-pdp`](../../docs/guides/pdp-service.md) on the sample registry and policies in `examples/`, mounted read-only. Edit a file there and the next request sees it. |
| **`postgres`** | `127.0.0.1:15432` | PostgreSQL 18 with an empty database `ascc`, user `ascc`. The tables arrive with the registry service. |
| **`keycloak`** | `http://127.0.0.1:18080` | Keycloak 26 in development mode with the realm `ascc-dev` imported from [`keycloak/ascc-dev-realm.json`](keycloak/ascc-dev-realm.json). The realm has no users and no clients yet. |
| **`openbao`** | `http://127.0.0.1:18200` | OpenBao 2.7, initialised and unsealed by [`openbao/dev-unseal.sh`](openbao/dev-unseal.sh), with KV v2 at `secret/` and transit at `transit/`. No key and no secret is created. |
| **`jaeger`** | `http://127.0.0.1:16686` | The Jaeger UI, taking OTLP on `127.0.0.1:4317` and `4318`. Nothing sends it a trace yet. |
| **`egress`** | `http://127.0.0.1:15001` | A placeholder for the egress proxy: Envoy with no upstream, answering 403 to every request and every `CONNECT`. |
| **plugin host** | not running | A commented block in `compose.yaml` and the `sandbox` network it will join, which has no route off the machine. There is no image to run until Slice 8. |

Try the decision point and the proxy:

```bash
curl -s -X POST http://127.0.0.1:8181/pdp/v1/decide -H 'Content-Type: application/json' -d '{
  "tenant_id": "tnt_lab",
  "principal": { "type": "Agent", "id": "dev-agent" },
  "action": "runtime.tool_use",
  "resource": { "type": "Tool", "id": "claude-code/Bash" },
  "context": { "path_in_repo": true, "network": true, "touches_secret_path": true, "touches_ascc_state": false }
}'
curl -s -x http://127.0.0.1:15001 http://paste.example/
```

The first answers a deny from `deny-network-with-secret-path`; the second, `ascc egress: denied, nothing is
allowed out yet`.

## The secrets

**No password, token or key is in this repository.** Each service makes its own from the kernel's random
source the first time it starts and keeps it in its own volume, where no other service can read it. Nothing
prints one or puts one on a command line. To read one, ask for it:

```bash
docker compose -f deploy/compose/compose.yaml exec postgres cat /run/ascc/password
docker compose -f deploy/compose/compose.yaml exec keycloak cat /opt/keycloak/data/admin-password
docker compose -f deploy/compose/compose.yaml exec openbao cat /openbao/file/dev-keys
```

The Keycloak user is `admin`; Keycloak marks it temporary and asks you to make a permanent one. The OpenBao
file holds the one unseal key and the root token. `down -v` deletes all of them, and the next `up` makes new
ones.

## The rules the Compose file keeps

`go test ./deploy/compose` reads Compose's own resolved view of the file and fails if one is broken:

- **Every published port is bound to `127.0.0.1`.** Nothing here authenticates a caller well enough to face a
  network. `ascc-pdp` runs with `--allow-remote` because it has to listen on every address inside its
  container to be reachable at all, so every container in this stack may ask it for a decision.
- **Every image is pinned by digest,** the Go image `ascc-pdp` is built with included.
- **No variable that names a password, token, key or secret is set in the file,** and no URL in it carries a
  password. The test knows those two shapes; it can't recognise a secret written any other way.
- **Every service sets `no-new-privileges` and nothing else under `security_opt`;** none is privileged, adds a
  capability, shares a namespace with the host, or mounts anything from outside this repository (the Docker
  socket included).
- **The `sandbox` network is internal,** and every network is a plain bridge.

A second test breaks each rule in turn and checks it is caught.

`ASCC_COMPOSE_SMOKE=1 go test -count=1 ./deploy/compose` starts the whole stack as a second project, checks
that the decision point denies and allows the sample requests, the realm is served, OpenBao is unsealed with
both engines and unseals again after a restart, the Jaeger UI and the `ascc` database answer, the proxy
refuses a request and a `CONNECT`, and no secret is in a log. Then it removes it. It uses the same ports, so
stop your own stack first. CI runs it on Linux. It doesn't check the OTLP ports or that the realm is empty.

## What it doesn't do

- **It isn't a deployment.** There is no TLS, no backup, no upgrade path and no production profile. The plan's
  production profile needs keys the customer owns, and none of that is here.
- **It doesn't protect its own secrets from someone on the machine.** Anyone who can run `docker` can read
  them, and OpenBao's seal protects nothing while its key sits beside it.
- **The egress proxy is not in anyone's path.** Nothing is routed through it yet, so it stops nothing but the
  requests you send it by hand.
- **`ascc-pdp` here is as in its guide:** it doesn't know who is asking and keeps no audit chain. Its policies
  are the samples in `examples/`, which anything that can write to this repository can change, an agent
  working in it included. Its guide says to keep real policies where an agent can't write.
- **Only `pdp` and `egress` are on a read-only file system, and Postgres keeps the default capabilities** its
  image needs to start as root and drop to its own user. Every service but the future plugin host can reach
  the internet.
- **Keycloak keeps its data in its own embedded database,** not in Postgres.
- **Image updates are by hand.** A pinned digest never moves, so a fix in a newer image arrives only when
  someone changes the tag and digest together.
- **Ports are fixed.** If one is taken, change the left-hand number in `compose.yaml`.

---

<p align="center">
  <a href="../../docs/guides/pdp-service.md">← The PDP as a service</a> ·
  <a href="../../docs/README.md">Docs home</a> ·
  <a href="../../docs/ROADMAP.md">Roadmap →</a>
</p>
