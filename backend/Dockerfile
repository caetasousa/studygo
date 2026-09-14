FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker
# Diretório do volume de provas. Ver a nota no estágio final.
RUN mkdir -p /out/lib/provas && chmod 2770 /out/lib/provas

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
# O mesmo diretório de provas da imagem final: se este container criar o volume
# antes do processador, o volume não pode nascer root:root 755.
RUN mkdir -p /var/lib/provas && chown 65532:65532 /var/lib/provas && chmod 2770 /var/lib/provas

EXPOSE 8080
CMD ["/usr/local/bin/dev-run.sh"]

# :nonroot, e não a tag padrão: a imagem padrão do distroless roda como uid 0.
# O binário é estático, só escreve no volume de provas e escuta numa porta alta
# — nada aqui precisa de root, e um processo que não precisa não deve ter. O
# edital-processor já fazia assim (uid 10001); esta era a exceção.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server
COPY --from=build /out/worker /worker
# O volume de provas nasce com o dono e o modo deste diretório quando é este
# container que o cria: grupo 65532 com setgid, o mesmo grupo "provas" em que o
# edital-processor põe o usuário dele. Assim backend e processador gravam e
# leem os arquivos um do outro, seja qual for o que subir primeiro.
#
# Copia-se o PAI (/out/lib/): COPY de um diretório leva só o conteúdo, e o
# destino criado nasce 0755 — sem escrita para o grupo, o processador não
# conseguia gravar os recortes.
COPY --from=build --chown=65532:65532 /out/lib/ /var/lib/

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/server"]
