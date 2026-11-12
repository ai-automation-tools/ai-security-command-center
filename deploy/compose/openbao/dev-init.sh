#!/bin/sh
# Development only. Starts OpenBao, which unseals itself with ASCC_OPENBAO_SEAL_KEY (config.hcl), initialises
# it the first time, and makes its root token ASCC_OPENBAO_ROOT_TOKEN. Both come from .env, so `docker compose
# up` gives a usable secrets backend with no step by hand and no secret that exists only in a volume.
#
# A key that sits in a file next to the stack protects nothing from someone on the machine, which is why this
# must never run anywhere that holds a real secret. Neither value is printed or put on a command line.
set -eu
umask 077

export BAO_ADDR=http://127.0.0.1:8200
init=/openbao/file/dev-init.tmp # the first root token, kept only until the one from .env replaces it
ready=/tmp/ascc-ready           # the health check waits for this: unsealed, with both engines mounted
rm -f "$ready"

mkdir -p /openbao/file/data
bao server -config=/ascc/config.hcl &
server=$!
body=/tmp/ascc-token # holds the root token from .env for one command; never outlives this script
trap 'rm -f "$body"' EXIT
trap 'rm -f "$body"; kill "$server" 2>/dev/null || true; wait "$server" 2>/dev/null || true; exit 0' TERM INT

# retry <what> <command...>: once a second for a minute, then give up and stop the container.
retry() {
  what=$1
  shift
  n=0
  until "$@"; do
    n=$((n + 1))
    if [ "$n" -ge 60 ]; then
      echo "dev-init: $what did not happen within 60 seconds" >&2
      exit 1
    fi
    sleep 1
  done
}

# `bao status` exits 0 unsealed, 2 sealed (which is also what it says before the first init), 1 unreachable.
answering() { bao status >/dev/null 2>&1 || [ $? -eq 2 ]; }
retry "OpenBao answering" answering

if ! bao status 2>/dev/null | grep -q '^Initialized  *true'; then
  bao operator init -recovery-shares=1 -recovery-threshold=1 >"$init"
fi

# A first run, or one that was stopped before this finished: swap the generated root token for the one in
# .env, then forget the generated one. The recovery key is dropped; this stack has no use for it.
if [ -s "$init" ]; then
  BAO_TOKEN=$(sed -n 's/^Initial Root Token: //p' "$init")
  export BAO_TOKEN
  active() { bao token lookup >/dev/null 2>&1; }
  retry "OpenBao becoming active" active
  printf '{"id":"%s","policies":["root"],"display_name":"ascc-dev"}' "$ASCC_OPENBAO_ROOT_TOKEN" >"$body"
  bao write auth/token/create-orphan @"$body" >/dev/null || {
    rm -f "$body"
    echo "dev-init: OpenBao refused ASCC_OPENBAO_ROOT_TOKEN; use letters and digits only, with no dot" >&2
    exit 1
  }
  rm -f "$body"
  bao token revoke -self >/dev/null
  rm -f "$init"
fi

# The two engines the plan uses: KV v2 for secrets and transit for signing. No key and no secret is created.
export BAO_TOKEN="$ASCC_OPENBAO_ROOT_TOKEN"
mounted() { mounts=$(bao secrets list 2>/dev/null); }
retry "OpenBao accepting the root token from .env (if .env changed since the first start, run down -v)" mounted
case "$mounts" in *"secret/"*) ;; *) bao secrets enable -path=secret kv-v2 >/dev/null ;; esac
case "$mounts" in *"transit/"*) ;; *) bao secrets enable transit >/dev/null ;; esac
unset BAO_TOKEN mounts

: >"$ready"
echo "dev-init: OpenBao is unsealed, with kv-v2 at secret/ and transit at transit/"
wait "$server"
