#!/bin/sh
# Development only. Starts OpenBao, initialises it the first time with one unseal key, and unseals it on
# every start, so `docker compose up` gives a usable secrets backend with no step by hand.
#
# The unseal key and the root token are kept in the data volume, next to the data they protect. That is
# exactly what sealing exists to prevent, and it is why this script must never run anywhere that holds a real
# secret. Neither value is printed or put on a command line: the key goes to the API in a request body and the
# token to `bao` in its environment.
set -eu
umask 077

export BAO_ADDR=http://127.0.0.1:8200
keys=/openbao/file/dev-keys
ready=/tmp/ascc-ready # the health check waits for this, so "healthy" means unsealed with both engines mounted
rm -f "$ready"

mkdir -p /openbao/file/data
bao server -config=/ascc/config.hcl &
server=$!
trap 'kill "$server" 2>/dev/null || true; wait "$server" 2>/dev/null || true; exit 0' TERM INT

# `bao status` exits 0 unsealed, 2 sealed, 1 when it can't be reached.
state() {
  if bao status >/dev/null 2>&1; then echo 0; else echo $?; fi
}

# retry <what> <command...>: once a second for a minute, then give up and stop the container.
retry() {
  what=$1
  shift
  n=0
  until "$@"; do
    n=$((n + 1))
    if [ "$n" -ge 60 ]; then
      echo "dev-unseal: $what did not happen within 60 seconds" >&2
      exit 1
    fi
    sleep 1
  done
}

answering() { [ "$(state)" != 1 ]; }
retry "OpenBao answering" answering

# A stop between the init and the rename below leaves the keys under the temporary name. Take them.
if [ ! -s "$keys" ] && grep -q '^Initial Root Token: ' "$keys.tmp" 2>/dev/null; then
  mv "$keys.tmp" "$keys"
fi
if [ ! -s "$keys" ]; then
  # Fails, and stops the container, if OpenBao is already initialised and the keys are gone. There is no way
  # back from that but `docker compose down -v`.
  bao operator init -key-shares=1 -key-threshold=1 >"$keys.tmp"
  mv "$keys.tmp" "$keys"
fi

if [ "$(state)" = 2 ]; then
  # `bao operator unseal` takes the key only as an argument or from a terminal, and an argument is in the
  # process list. So the key goes to the API in a request body, built by the shell's own printf.
  body=/tmp/ascc-unseal
  printf '{"key":"%s"}' "$(sed -n 's/^Unseal Key 1: //p' "$keys")" >"$body"
  wget -q -O /dev/null --header 'Content-Type: application/json' --post-file "$body" "$BAO_ADDR/v1/sys/unseal" || {
    rm -f "$body"
    echo "dev-unseal: OpenBao refused the unseal key" >&2
    exit 1
  }
  rm -f "$body"
fi

# The two engines the plan uses: KV v2 for secrets and transit for signing. No key and no secret is created.
BAO_TOKEN=$(sed -n 's/^Initial Root Token: //p' "$keys")
export BAO_TOKEN
mounted() { mounts=$(bao secrets list 2>/dev/null); }
retry "OpenBao becoming active" mounted
case "$mounts" in *"secret/"*) ;; *) bao secrets enable -path=secret kv-v2 >/dev/null ;; esac
case "$mounts" in *"transit/"*) ;; *) bao secrets enable transit >/dev/null ;; esac
unset BAO_TOKEN mounts

: >"$ready"
echo "dev-unseal: OpenBao is unsealed, with kv-v2 at secret/ and transit at transit/"
wait "$server"
