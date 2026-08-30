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

# Wait for either a source change or the server dying. Without the second case
# a crash (Postgres not up yet, a panic on boot) left the container alive with
# nothing serving until the next save — which looks exactly like a broken build.
while :; do
    if inotifywait -qq -r -t 5 -e close_write,create,delete,move \
            --include '\.(go|sql)$' /src; then
        echo "--- alteração detectada; recompilando ---" >&2
        stop
        start || true
    elif [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
        echo "--- server caiu; reiniciando ---" >&2
        pid=""
        start || true
    fi
done
