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
> in development mode, and OpenBao's seal key sits in a file on the same disk as its data. It is safe
> on one developer's machine because every port is bound to `127.0.0.1` and nothing in it is real.

## Run it

You need Docker Engine 28 or newer, with Compose v2. On Linux, older engines let another machine on the same
network segment reach a port that was published on loopback only.

First the secrets. Copy [`.env.example`](../../.env.example) to `.env` at the repository root and give each of
the four names a long random value (letters and digits; `openssl rand -hex 32` makes one, and the seal key
must be exactly that). Then, from the repository root:

```bash
docker compose --env-file .env -f deploy/compose/compose.yaml up -d --build --wait
```

Every command below takes the same `--env-file .env`. With a value missing, Compose stops and names it.

The first run downloads about 1 GB of images and builds `ascc-pdp`; after that it takes about a minute, most
of it Keycloak. `--wait` returns when Postgres, Keycloak and OpenBao report healthy, which for OpenBao means unsealed with
both engines mounted.

```bash
docker compose --env-file .env -f deploy/compose/compose.yaml down        # stop, keep the data
docker compose --env-file .env -f deploy/compose/compose.yaml down -v     # stop and delete every volume
```

## What you get

| Service | On your machine | What is in it |
|:---|:---|:---|
| **`pdp`** | `http://127.0.0.1:8181` | [`ascc-pdp`](../../docs/guides/pdp-service.md) on the sample registry and policies in `examples/`, mounted read-only. Edit a file there and the next request sees it. |
| **`postgres`** | `127.0.0.1:15432` | PostgreSQL 18 with an empty database `ascc`, user `ascc`. The tables arrive with the registry service. |
| **`keycloak`** | `http://127.0.0.1:18080` | Keycloak 26 in development mode with the realm `ascc-dev` imported from [`keycloak/ascc-dev-realm.json`](keycloak/ascc-dev-realm.json). The realm has no users and no clients yet. |
| **`openbao`** | `http://127.0.0.1:18200` | OpenBao 2.7, which unseals itself with the key from `.env` and is initialised by [`openbao/dev-init.sh`](openbao/dev-init.sh), with KV v2 at `secret/` and transit at `transit/`. No key and no secret is created. |
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

**Every secret the stack has is in `.env` at the repository root, and nowhere else.** Git ignores that file,
[`.env.example`](../../.env.example) lists the names, and nothing prints a value or puts one on a command line.

| Name | What it is |
|:---|:---|
| **`ASCC_POSTGRES_PASSWORD`** | The password of the `ascc` user. |
| **`ASCC_KEYCLOAK_ADMIN_PASSWORD`** | The password of the `admin` user. Keycloak marks that user temporary and asks you to make a permanent one. |
| **`ASCC_OPENBAO_SEAL_KEY`** | The key OpenBao unseals itself with: 32 random bytes as 64 hex digits. |
| **`ASCC_OPENBAO_ROOT_TOKEN`** | OpenBao's root token. On the first start `dev-init.sh` replaces the token OpenBao generates with this one and revokes the generated one. |

**Each one is used when its volume is first made.** Change a value in `.env` afterwards and the running data
still expects the old one: Postgres keeps its password, Keycloak its admin, and OpenBao can't be unsealed by a
different key. Run `down -v` and start again.

## The rules the Compose file keeps

`go test ./deploy/compose` reads Compose's own resolved view of the file and fails if one is broken:

- **Every published port is bound to `127.0.0.1`.** Nothing here authenticates a caller well enough to face a
  network. `ascc-pdp` runs with `--allow-remote` because it has to listen on every address inside its
  container to be reachable at all, so every container in this stack may ask it for a decision.
- **Every image is pinned by digest,** the Go image `ascc-pdp` is built with included.
- **Every variable that names a password, token, key or secret takes its value from `.env`,** no URL in the
  file carries a password, `.env.example` holds names only, and with no secret set the file doesn't resolve.
  The test knows those shapes; it can't recognise a secret written any other way.
- **Every service sets `no-new-privileges` and nothing else under `security_opt`;** none is privileged, adds a
  capability, shares a namespace with the host, or mounts anything from outside this repository (the Docker
  socket included).
- **The `sandbox` network is internal,** and every network is a plain bridge.

A second test breaks each rule in turn and checks it is caught.

`ASCC_COMPOSE_SMOKE=1 go test -count=1 ./deploy/compose` starts the whole stack as a second project, checks
that the decision point denies and allows the sample requests, the realm is served, OpenBao is unsealed with
both engines and unseals again after a restart, Postgres, Keycloak and OpenBao each accept the secret it was
given (and Postgres refuses a wrong one), the Jaeger UI answers, the proxy refuses a request and a `CONNECT`,
and no secret is in a log. It makes its own throwaway secrets and never reads your `.env`. Then it removes it. It uses the same ports, so
stop your own stack first. CI runs it on Linux. It doesn't check the OTLP ports or that the realm is empty.

## What it doesn't do

- **It isn't a deployment.** There is no TLS, no backup, no upgrade path and no production profile. The plan's
  production profile needs keys the customer owns, and none of that is here.
- **It doesn't protect its own secrets from someone on the machine.** They are in `.env` and in each
  container's environment, so anyone who can read that file or run `docker inspect` can read them, and
  OpenBao's seal protects nothing while its key sits on the same disk.
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
