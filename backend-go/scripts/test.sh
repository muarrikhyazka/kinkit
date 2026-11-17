#!/usr/bin/env bash
# Run the Go test suite against a throwaway Postgres container.
#
#   backend-go/scripts/test.sh            # go test ./...
#   backend-go/scripts/test.sh -run Foo   # extra args are passed to go test
#
# Starts postgres:16-alpine on a free local port (default 55432), runs
# `go test ./...` with TEST_DATABASE_URL pointing at it, and always removes
# the container afterwards. Only touches the container it created.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -d "$HOME/.local/go/bin" ]; then
  export PATH="$PATH:$HOME/.local/go/bin"
fi

port_in_use() {
  (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3>&- 3<&-; return 0; }
  return 1
}

PORT="${TEST_PG_PORT:-55432}"
while port_in_use "$PORT"; do PORT=$((PORT + 1)); done

NAME="kinkit-test-pg-$$-$RANDOM"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

echo "starting $NAME on 127.0.0.1:$PORT ..."
docker run -d --rm --name "$NAME" \
  -e POSTGRES_USER=kinkit -e POSTGRES_PASSWORD=kinkit -e POSTGRES_DB=kinkit_test \
  -p "127.0.0.1:$PORT:5432" postgres:16-alpine >/dev/null

for _ in $(seq 1 60); do
  # pg_isready over TCP: the entrypoint's temporary init server listens on
  # the unix socket only, so this waits for the real server.
  if docker exec "$NAME" pg_isready -h 127.0.0.1 -U kinkit -d kinkit_test >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

export TEST_DATABASE_URL="postgres://kinkit:kinkit@127.0.0.1:$PORT/kinkit_test?sslmode=disable"
go test -count=1 "$@" ./...
