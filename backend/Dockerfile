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

FROM gcr.io/distroless/static-debian12

COPY --from=build /out/server /server
COPY --from=build /out/worker /worker

EXPOSE 8080

ENTRYPOINT ["/server"]
