#!/bin/sh
# Rebuild-and-restart loop for local development (see docker-compose.override.yml).
# Not used in production: the prod image runs the compiled binary directly.
set -u

pid=""

start() {
    go build -o /tmp/server ./cmd/server || {
        echo "--- build falhou; aguardando a próxima alteração ---" >&2
        return 1
    }
    /tmp/server &
    pid=$!
    echo "--- server iniciado (pid $pid) ---" >&2
}

stop() {
    [ -n "$pid" ] && kill "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    pid=""
}

trap 'stop; exit 0' INT TERM

start || true

while inotifywait -qq -r -e close_write,create,delete,move \
        --include '\.(go|sql)$' /src; do
    echo "--- alteração detectada; recompilando ---" >&2
    stop
    start || true
done
