FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

# ---- dev: rebuild and restart on any .go change ----
# Used only by docker-compose.override.yml, which bind-mounts ./backend over
# /src. inotify-tools is the whole dependency: no air/reflex to pin and keep
# up to date, and the loop is short enough to read.
FROM golang:1.27-alpine AS dev

RUN apk add --no-cache inotify-tools

WORKDIR /src
COPY --from=build /go/pkg/mod /go/pkg/mod
COPY dev-run.sh /usr/local/bin/dev-run.sh
RUN chmod +x /usr/local/bin/dev-run.sh

EXPOSE 8080
CMD ["/usr/local/bin/dev-run.sh"]

# :nonroot, e não a tag padrão: a imagem padrão do distroless roda como uid 0.
# O binário é estático, não escreve em disco e escuta numa porta alta — nada
# aqui precisa de root, e um processo que não precisa não deve ter. O
# edital-processor já fazia assim (uid 10001); esta era a exceção.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server
COPY --from=build /out/worker /worker

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/server"]
