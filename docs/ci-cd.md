# Publicar uma versão nova

> **Em uma frase:** você envia o código, a máquina testa sozinha e, passando
> tudo, a mesma versão testada vai para o ar.

## O básico, sem termos técnicos

**Antes**, publicar era assim: você rodava um comando e o seu computador montava
o programa e mandava para o servidor.

O problema: o programa era montado na *sua* máquina. Se você tivesse uma versão
diferente de alguma ferramenta, ou tivesse esquecido de salvar um arquivo, o que
ia para o ar não era exatamente o que você testou.

**Agora**, um computador neutro faz isso: monta o programa uma vez, testa, e
guarda essa versão exata. Essa mesma versão — os mesmos bytes — é a que vai
para o servidor. Nada é montado de novo no caminho.

É como a diferença entre mandar a receita do bolo (cada um faz o seu, e sai
diferente) e mandar o bolo pronto (todo mundo recebe o mesmo).

### O comando que você precisa saber

```bash
make push      # na main: testa e, passando, publica no servidor
```

Para **voltar atrás**, é um clique: no GitLab, **Operate → Environments →
staging**, escolha a versão anterior e clique em **Rollback environment**.

---

## Detalhes técnicos

## Por que é assim

O `make deploy` antigo compilava as imagens localmente, salvava em tarball e
copiava para o servidor, identificando tudo por `:latest`. Isso significa que o
que rodava no ar não era necessariamente o que passou nos testes — bastava uma
dependência ter mudado entre um build e outro. E `:latest` pode ser reapontada,
então não havia como saber, olhando o servidor, qual versão estava no ar.

Agora a imagem é construída **uma vez**, testada de pé, publicada no Container
Registry e identificada por **digest** (`repo@sha256:...`), que é o hash do
conteúdo. O servidor recebe esse digest.

## O caminho de uma mudança

```
git push (main)
   │
   ├── validate ──── sintaxe da pipeline e do Ansible (este só se o Ansible mudou)
   ├── lint ──────── go vet · svelte-check · ruff + mypy --strict  ┐
   ├── unit_test ─── testes dos 3 serviços + integração com Postgres │ ao mesmo
   ├── build ─────── constrói as imagens (ainda não publica)         ┘ tempo
   ├── artifact_test  sobe as imagens e exige resposta em /health
   │   └── e2e ──────  o app inteiro pelo navegador, contra essas imagens
   ├── publish ───── só com TUDO acima verde: envia ao Registry e captura o DIGEST
   ├── deploy_staging  Ansible promove o digest → servidor
   └── smoke_test ── o servidor responde, na versão deste commit

Operate → Environments → staging
   └── Rollback environment ── reexecuta o deploy_staging de uma pipeline antiga
```

O build não espera os testes: começa junto com eles, e o e2e roda assim que as
imagens ficam prontas. A garantia está no `publish`, que espera por todos —
validate, lint, testes, artifact_test e e2e. Um teste vermelho não impede o
build, mas impede que a imagem chegue ao Registry, e portanto qualquer deploy.

O job `e2e` é do projeto, não do template (`.gitlab-ci.yml`): roda a mesma
suíte do `make e2e` (`e2e/ci.sh`) com as imagens que o build acabou de fazer, e
o `publish` espera por ele — uma imagem que quebra um fluxo do usuário não
chega ao Registry. O relatório (`e2e/relatorio/`, com o print de cada cenário)
fica como artefato do job por 30 dias, passe ou falhe. A imagem do Playwright
(3,5 GB) é guardada no registry do runner na primeira pipeline de cada versão;
as seguintes a puxam de lá.

### O que o projeto muda no template

O template (`gitlab-ci-templates`) foi escrito para dois ambientes numa VPS. O
`.gitlab-ci.yml` do projeto ajusta:

- **Um ambiente só.** `deploy_production`, `verify` e `rollback_production`
  estão desligados (`when: never`): o servidor é um, e o nome dele na esteira é
  `staging`. Não crie tag de versão — não há para onde ela ir. O `url` do
  ambiente é `https://studygo.d4ffy.com`.
- **Build em paralelo e com cache.** Os builds têm `needs: []` e usam o
  BuildKit com cache de camadas no registry-local
  (`cache/<componente>:buildcache`): uma camada que não mudou — o apt, o
  tesseract e o Claude Code do processador — sai do cache em segundos e com o
  mesmo digest, então o publish não a reenvia nem o servidor a baixa de novo.
  Em compensação, o `needs` do `publish` lista todos os jobs de teste.
- **`validate:ansible` enxuto.** Só o syntax-check (o `ansible-lint` do template
  tinha o resultado ignorado) e só quando `ansible/`, a esteira ou a imagem de
  deploy mudam.
- **Cache dos testes.** Módulos do Go, pacotes do npm e do uv ficam no cache do
  runner, com a chave no lockfile de cada serviço.
- **Imagem de deploy pronta.** O `deploy_staging` roda em
  `ci/deploy.Dockerfile` (cliente SSH e Ansible já instalados), guardada no
  registry-local como `ci/deploy-ansible:<DEPLOY_IMAGE_TAG>`. Mudou o
  Dockerfile? Suba `DEPLOY_IMAGE_TAG` no `.gitlab-ci.yml`: o job
  `imagem:deploy` só constrói a tag que ainda não existe.
- **`smoke_test` de verdade.** Exige que o `/health` responda com a `versao`
  deste commit, pelo nginx do servidor (`172.17.0.1:8480`, o caminho do
  deploy) e pelo endereço público, pelo túnel. Um deploy que deixasse o
  container antigo de pé passava no do template.
- **Segurança semanal.** Os jobs `seguranca:*` (o mesmo `make seguranca`) só
  rodam na pipeline agendada, que testa tudo mas não publica nem implanta, e
  avisam sem bloquear. O agendamento é feito uma vez no GitLab: **Build →
  Pipeline schedules → New schedule**, branch `main`, toda semana.

Ao subir o `ref` do template, confira se mudaram o `needs` do `publish`, o
`before_script` do `.deploy_base` ou o script do `.build_template`: o projeto
os sobrescreve.

## Qual versão está no ar

```bash
make servidor-health
# {"status":"ok","versao":"4de881fd","deploy":"2884592321","schema":8}
```

| Campo | O que é |
|---|---|
| `versao` | o commit que foi publicado |
| `deploy` | a pipeline que implantou |
| `schema` | a última migration aplicada no banco |

Os três vêm do deploy, não do build: a mesma imagem sobe como publicações
diferentes. Em desenvolvimento, `versao` é `dev` e `deploy` não aparece.

## Rollback

**É um botão.** No GitLab, **Operate → Environments → staging**. A lista
mostra cada publicação pelo commit; na versão para a qual quer voltar, clique
em **Rollback environment**.

O GitLab reexecuta o `deploy_staging` daquela pipeline antiga, com os digests
que ela publicou. Nada é reconstruído: sobe a mesma imagem que já esteve no ar,
pelo mesmo playbook de sempre — cópia do banco, health check e, se a versão
antiga não responder, a volta automática para a que estava.

O mesmo resultado sai pela pipeline: abra a pipeline desejada e reexecute (↻)
o job `deploy_staging`. Para republicar a versão atual sem mudar nada, é o
**Re-deploy to environment** da mesma página.

Depois, `make servidor-health` deve mostrar o commit antigo.

Duas coisas a saber:

- **O botão só aparece em versões que já foram publicadas com sucesso.** Se
  "Prevent outdated deployment jobs" estiver ligado (Settings → CI/CD →
  General pipelines), mantenha marcado "Allow job retries for rollback
  deployments" — sem ele o GitLab bloqueia a reexecução de um deploy antigo.
- **Os digests vêm dos artifacts do job `publish`**, que vencem em 90 dias. O
  GitLab guarda os da última pipeline bem-sucedida de cada ref ("Keep
  artifacts from most recent successful jobs", em Settings → CI/CD →
  Artifacts); pipelines mais antigas da `main` podem perder os seus.

### E o banco?

**Rollback de código não reverte schema.** O runner só aplica `.up.sql`: a
versão que volta encontra o banco como a outra o deixou. O `schema` do
`make servidor-health` diz onde ele está — se for maior que a última migration
da versão que voltou, o banco está à frente do código.

| A versão que sai trouxe… | O que fazer |
|---|---|
| nenhuma migration | Rollback environment, e acabou |
| migration **aditiva** (tabela, coluna, índice novos) | Rollback environment; o código antigo ignora o que não conhece |
| migration **destrutiva** (tem `-- contract:`) | o botão não basta: restaure a cópia do banco (abaixo) ou siga em frente com uma migration corretiva |

O que mantém a terceira linha vazia é expand/contract: primeiro uma publicação
que para de usar a coluna (mantendo-a), e só numa publicação **posterior** a
migration que a remove — nunca as duas na mesma. Entre esses dois passos,
qualquer rollback é seguro.

O `make check` cobra isso. Uma migration com `DROP TABLE`, `DROP COLUMN`,
`RENAME`, `ALTER COLUMN ... TYPE`, `SET NOT NULL` ou `TRUNCATE` falha o build sem
um marcador que diga quem já parou de usar o que ela tira:

```sql
-- contract: o commit 4de881f parou de ler planos.ciclo
```

O teste não tem como conferir a ordem das publicações — o marcador obriga você a
escrevê-la. `ADD COLUMN ... NOT NULL DEFAULT` é aditivo e passa sem marcador.

### Cópia do banco antes de cada deploy

Todo deploy, rollback incluído, faz um `pg_dump` antes de subir as imagens
novas: `<app_dir>/backups/pre-deploy-<data>-p<pipeline>.sql.gz`, mantidas as 5
últimas. Se a cópia falhar, o deploy é recusado antes de mexer em qualquer
coisa.

A cópia que desfaz uma versão é a que leva no nome o `deploy` que o
`make servidor-health` mostrava **com ela no ar**: foi feita imediatamente
antes daquela pipeline implantar.

Restaurar — no servidor, e só quando a árvore acima mandar:

```bash
ssh -i ~/.ssh/studygo_ci -p 2222 studygo@127.0.0.1
cd /opt/studygo-staging
export DOCKER_HOST=unix:///run/user/$(id -u)/docker.sock   # o Docker rootless

# 1. pare quem escreve no banco
docker compose stop backend worker

# 2. guarde o estado atual — ele tem o que foi escrito DEPOIS do deploy, e a
#    restauração vai descartar isso
docker compose exec -T postgres pg_dump -U studygo_staging studygo_staging \
  | gzip > backups/antes-da-restauracao-$(date +%Y%m%d-%H%M%S).sql.gz

# 3. recrie o banco a partir da cópia
ls -1t backups/
docker compose exec -T postgres psql -U studygo_staging -d postgres -v ON_ERROR_STOP=1 \
  -c 'DROP DATABASE studygo_staging WITH (FORCE)' \
  -c 'CREATE DATABASE studygo_staging OWNER studygo_staging'
gunzip -c backups/pre-deploy-AAAAMMDD-HHMMSS-pNNNN.sql.gz \
  | docker compose exec -T postgres psql -U studygo_staging -d studygo_staging -v ON_ERROR_STOP=1 >/dev/null
```

4. No GitLab, **Rollback environment** para a versão anterior. É ele que sobe o
   backend e o worker de novo, com o código que entende aquele schema.

O que foi escrito entre o deploy e a restauração fica só na cópia do passo 2 —
recuperar isso, se for o caso, é consulta à mão naquele arquivo.

## O ambiente

| | staging |
|---|---|
| Servidor | distro `ubuntu-server` do WSL, usuário `studygo` ([deploy.md](deploy.md)) |
| Endereço | `https://studygo.d4ffy.com`, pelo túnel com token ([cloudflare-tunnel.md](cloudflare-tunnel.md)) |
| Diretório | `/opt/studygo-staging` |
| Banco | `studygo_staging` |
| Deploy | automático, a cada push na main |

## Runners

Rodam na sua máquina (a distro de desenvolvimento do WSL), registrados no
GitLab.com como runners de **grupo**, reusáveis pelos próximos projetos.

| Tag | Faz | Acesso |
|---|---|---|
| `local-build` | lint, testes, build de imagens | Registry |
| `local-deploy` | Ansible | SSH ao servidor; **protegido** |

Separados de propósito: o runner que constrói não tem credencial de servidor, e
o que implanta não constrói.

**Todo job declara sua tag.** Sem isso o job cairia no runner compartilhado do
GitLab.com e consumiria a cota de 400 min/mês. Com runner próprio, o consumo é
zero e não há limite de jobs.

Os jobs rodam em containers do Docker da distro de desenvolvimento e chegam ao
servidor por `172.17.0.1` (o `docker0` dela, na rede que todas as distros
dividem): SSH na 2222, nginx na 8480.

Se o WSL estiver desligado, as pipelines ficam na fila e rodam quando ele voltar.
Nada se perde.

Os jobs trocam as imagens por um registry na distro de desenvolvimento, o
`registry-local` (porta 5000, dados no volume `registry-data`). Parado, os
builds falham com "o registry local não respondeu": `docker start
registry-local`. Se o container sumiu, recrie-o com o mesmo volume — e com o
delete ligado, sem o qual a limpeza não funciona:

```bash
docker run -d --name registry-local --restart unless-stopped \
  -p 5000:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true \
  -v registry-data:/var/lib/registry registry:2.8.3
```

Ali uma imagem só serve durante a própria pipeline: o deploy e o rollback
puxam do GitLab, pelo digest. `make registry-limpar` apaga as com mais de 2
dias (`DIAS=n` muda; `SIMULAR=1` só lista), roda o garbage-collect e reinicia
o registry; guarda `cache/*` (cache de camadas e Playwright) e `ci/*` (a imagem
de deploy, que um rollback de pipeline antiga pede). Recusa-se a rodar com job
em andamento. Em 12/10/2026 levou o volume de 12,1 GB a 1,3 GB.

No servidor, o próprio deploy apaga as imagens que nenhum container usa e que
têm mais de uma semana (`ansible/deploy.yml`, última tarefa).

## Variáveis exigidas

Settings → CI/CD → Variables, todas **protegidas** e (exceto `SSH_KNOWN_HOSTS`)
**mascaradas**:

| Variável | O que é |
|---|---|
| `CI_SSH_PRIVATE_KEY` | a chave privada `~/.ssh/studygo_ci`, do usuário `studygo` no servidor |
| `SSH_KNOWN_HOSTS` | saída de `ssh-keyscan -p 2222 172.17.0.1` — não mascarar (multilinha); o job tira dela o endereço do servidor |
| `CI_DEPLOY_USER` / `CI_DEPLOY_PASSWORD` | deploy token do Registry |
| `ANSIBLE_VAULT_PASSWORD` | senha do Ansible Vault |
| `ANSIBLE_SECRETS` | o `secrets.yml` do servidor, cifrado |

## Não existe deploy à mão

Todo deploy passa por aqui. Não há atalho, e não é para criar um.

O playbook até recusa sozinho o que não for rastreável — sem digest, ou com
`:latest`, ele falha na verificação inicial. Mas a recusa é a última linha de
defesa, não o procedimento: rodar o Ansible da sua máquina implanta um artefato
que ninguém testou naquela combinação e pula o `smoke_test`.

Precisa voltar atrás rápido? **Rollback environment**, em Operate →
Environments, promove um digest anterior sem reconstruir nada.

## Arquitetura

```mermaid
flowchart LR
    subgraph SUA["Sua máquina (WSL2)"]
        subgraph DEVD["Ubuntu-24.04 (desenvolvimento)"]
            DEV[git push]
            RB["runner local-build<br/>testes · imagens"]
            RD["runner local-deploy<br/>Ansible · protegido"]
        end
        subgraph SRV["ubuntu-server"]
            STG["studygo<br/>/opt/studygo-staging<br/>nginx :8480"]
            CF[cloudflared]
        end
    end

    subgraph GL["GitLab.com (Free)"]
        REPO[(repositório)]
        PIPE[pipeline]
        REG[("Container Registry<br/>imagens por digest")]
    end

    GH[(GitHub<br/>espelho ~5 min)]
    CFE(("Cloudflare"))

    DEV --> REPO --> PIPE
    PIPE -.jobs.-> RB
    PIPE -.jobs.-> RD
    RB -- push por digest --> REG
    REPO -.mirror.-> GH
    RD -- "SSH 172.17.0.1:2222" --> STG
    REG -.pull digest.-> STG
    CF -- túnel de saída --> CFE
    CF --> STG
```

O runner vive na sua máquina; o GitLab.com nunca inicia conexão para cá — é o
runner que busca os jobs. E o túnel também é de saída. Por isso nada precisa
ser exposto na sua rede.

## Portas, volumes e dependências

**No servidor**

| Serviço | Porta | Exposição |
|---|---|---|
| sshd | 2222 | rede do WSL (só chave) |
| nginx | 8480 | rede do WSL; público só pelo túnel |
| backend | 18080 | loopback |
| frontend | 15173 | loopback |
| postgres | — | só rede do compose |

**Volumes** (nomeados pelo diretório do projeto): `postgres_data`,
`edital_work`.

**Dependências externas**: GitLab.com (repositório, Registry e pipeline),
Cloudflare (túnel e HTTPS) e Google Gemini (opcional, importação de edital).
