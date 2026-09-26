# 🚢 Deploy numa VPS

![Ansible](https://img.shields.io/badge/Ansible-deploy-EE0000?logo=ansible&logoColor=white)
![Ubuntu](https://img.shields.io/badge/Ubuntu-24.04_LTS-E95420?logo=ubuntu&logoColor=white)
![nginx](https://img.shields.io/badge/nginx-TLS-009639?logo=nginx&logoColor=white)


> **Em uma frase:** este documento é sobre PREPARAR o servidor, não sobre
> publicar o sistema.
>
> Publicar é assunto de [ci-cd.md](ci-cd.md). Aqui está o que se faz uma única
> vez por servidor: instalar o Docker, configurar o endereço do site e o
> cadeado de segurança (HTTPS). Novo por aqui? Leia
> [como-funciona.md](como-funciona.md) antes.
O deploy roda numa VPS Ubuntu (o projeto usa uma Hostinger com Ubuntu 24.04 LTS),
provisionada por **Ansible**. As imagens Docker são construídas **pela pipeline**
e a VPS as baixa do Container Registry por digest — o código-fonte nunca vai
para a VPS.

```
GitLab.com                                    VPS
┌───────────────┐    docker pull        ┌──────────────────────────────────┐
│ Registry      │    (por digest)       │ nginx (borda, HTTPS)             │
│ imagens       │ ────────────────────► │   └─► frontend  (SPA + proxy /api)│
│ testadas      │                       │        └─► backend ─► postgres    │
└───────────────┘                       │             worker ─┘            │
                                        │  certbot renova o cert           │
                                        └──────────────────────────────────┘
```

> [!IMPORTANT]
> Este documento cobre o **provisionamento** do servidor (`bootstrap`,
> `lockdown`, `site`). A publicação da aplicação é da pipeline: veja
> [ci-cd.md](ci-cd.md). O `deploy.yml` não constrói imagem nenhuma — ele recusa
> rodar sem receber um digest já publicado.

Tudo mora em `ansible/`: um playbook por tarefa (`bootstrap`, `lockdown`,
`site`, `deploy`) e uma role por peça da infra (`common`, `docker`, `nginx`,
`certbot`, e, para o servidor no WSL, `docker_rootless` e `cloudflared`).

---

## 🖥️ Servidor no WSL (atual, desde 25/09/2026)

A VPS foi suspensa. O ambiente da esteira roda na distro **`ubuntu-server`**
do WSL da própria máquina de desenvolvimento, publicado por um **túnel da
Cloudflare**:

```
navegador ─► Cloudflare (HTTPS) ─► cloudflared ─► nginx :8480
                                   (ubuntu-server)   └─► frontend ─► backend ─► postgres
```

Por que é diferente da VPS — **todas as distros do WSL2 dividem a mesma
rede** (mesmo IP, mesmas portas, mesmo iptables):

- **Docker rootless**, do usuário `studygo` — um usuário por projeto, cada
  um com o próprio daemon e os próprios volumes. Um segundo Docker comum brigaria
  com o da distro de desenvolvimento pelo `docker0` e pelo iptables. O
  rootless tem rede própria e só publica as portas pedidas, em localhost.
- **Sem ufw nem fail2ban** (`firewall_local: false`): um "deny by default"
  fecharia a rede do desenvolvimento e do runner. Nenhuma porta precisa abrir:
  o cloudflared só faz conexão de saída.
- **SSH na 2222.** O job de deploy roda num container do runner desta mesma
  máquina e chega ao servidor por `172.17.0.1:2222`.
- **O nginx lê o IP do visitante em `CF-Connecting-IP`** (aceito só de
  localhost); sem isso todo mundo contaria como 127.0.0.1 no limite de taxa.
- **Só no ar com o PC ligado** e a distro de pé.
- **Endereço temporário.** Sem domínio próprio, o público entra por um Quick
  Tunnel (`*.trycloudflare.com`, serviço `cloudflared-studygo`), que muda a
  cada reinício — `make servidor-endereco` mostra o da vez. O túnel com token
  (`cloudflared_token`) já está conectado e espera um domínio na Cloudflare
  apontado para `localhost:8480`.
- **O deploy confere o `/health` no próprio servidor** (nginx em localhost),
  não pelo domínio: não depende de DNS nem do painel da Cloudflare.

Tudo isso está no inventário (`inventory/staging/group_vars/app/main.yml`:
`docker_rootless`, `borda: cloudflare`, `firewall_local`, `nginx_porta`) e o
`site.yml` escolhe os papéis por ele.

### Montar do zero

```bash
# 1. como root na distro (o "root por SSH" que o provedor dava na VPS):
#    pacotes, sshd na 2222 só por chave, e o usuário de deploy do projeto com
#    a chave da esteira. Idempotente; usuários a remover vão no fim.
wsl.exe -d ubuntu-server -u root -- bash -s -- \
  studygo "$(cat ~/.ssh/studygo_ci.pub)" \
  < ansible/bootstrap-wsl.sh

# 2. provisionar, desta distro
cd ansible
ssh-keyscan -p 2222 127.0.0.1 >> ~/.ssh/known_hosts
sed 's/SEU_IP_AQUI/127.0.0.1/' inventory/staging/hosts.ini.example > inventory/staging/hosts.ini
ansible-playbook site.yml -i inventory/staging/hosts.ini

# 3. túnel com token (para um domínio próprio): cloudflared_token no
#    secrets.yml (Vault), e o playbook de novo
ansible-vault edit inventory/staging/group_vars/app/secrets.yml
ansible-playbook site.yml -i inventory/staging/hosts.ini --tags cloudflared
```

No GitLab (Settings → CI/CD → Variables, ambiente `staging`),
`SSH_KNOWN_HOSTS` recebe a saída de `ssh-keyscan -p 2222 172.17.0.1`: é dela
que o job tira o host.

---

## 📋 Pré-requisitos na sua máquina

- **Ansible**
- `sshpass` — só no primeiro acesso à VPS
- Uma VPS Ubuntu com IP público e um domínio apontando para ela

---

## 1️⃣ Primeira vez (por servidor)

> [!NOTE]
> A única coisa que você digita à parte é a **senha de root da VPS** (a que o
> provedor gera). Ela é usada só no `bootstrap`/`lockdown` e nunca é salva.

```bash
cd ansible

# 1. chave SSH dedicada ao deploy (pule se já existir — NÃO sobrescreva)
# O nome do arquivo é anterior ao rename do projeto e é o que a VPS já
# autoriza — não troque para studygo_deploy sem reprovisionar o servidor.
ssh-keygen -t ed25519 -f ~/.ssh/annygo_deploy -N "" -C "annygo-deploy"

# 2. inventário e segredos, um por ambiente (staging e production); os
#    arquivos .example viram os reais, que são gitignored
cp inventory/production/hosts.ini.example inventory/production/hosts.ini
#   edite: ansible_host = IP da VPS

cp inventory/production/group_vars/app/secrets.yml.example \
   inventory/production/group_vars/app/secrets.yml
#   edite: letsencrypt_email, jwt_secret, postgres_password
#   opcional: gemini_api_key + edital_processor_token (liga a importação de edital por IA)

#   ajuste o domínio em inventory/production/group_vars/app/main.yml (app_domain)

# 3. cria o usuário sudo com sua chave (pede a senha de root)
ansible-playbook bootstrap.yml -e ansible_user=root --ask-pass

# 4. desliga o login de root por SSH (última vez que a senha de root é usada)
ansible-playbook lockdown.yml -e ansible_user=root --ask-pass

# 5. provisiona a box: firewall, Docker, nginx, certificado HTTPS
ansible-playbook site.yml

# 6. a aplicação sobe pela pipeline — veja ci-cd.md
#    (o deploy.yml exige um digest já publicado e recusa rodar sem ele)
```

Ao final, `https://SEU-DOMINIO/health` responde `{"status":"ok", ...}`, com a
versão, o deploy e o schema que estão no ar.

---

## 🔁 Atualizações

`bootstrap.yml` e `lockdown.yml` rodam **uma vez** por servidor. O dia a dia
depois disso é pelo `make`, da raiz do repositório:

```bash
make provision env=staging               # mudou infra (nginx, firewall, cert…)
make provision env=production            # o mesmo, no ambiente real
make provision env=production tags=nginx # só uma peça, sem tocar no resto
make health env=production               # confirma que respondeu
```

`env` é obrigatório: os dois ambientes dividem a mesma VPS, e um padrão
silencioso convidava a acertar produção sem querer. `tags` limita o alcance —
sem ela, `site.yml` roda tudo, inclusive o `apt upgrade` da role `common`.

A **aplicação** não sobe por aqui em nenhuma hipótese: quem publica é a
pipeline, staging primeiro e produção depois ([ci-cd.md](ci-cd.md)).

Publicar a aplicação é da pipeline: push na `main` implanta em staging, e
`make release go=1` cria a tag `v*` que libera o botão manual de produção. Veja
[ci-cd.md](ci-cd.md). O fluxo completo (desenvolver, verificar, commitar,
publicar) está em [fluxo-de-trabalho.md](fluxo-de-trabalho.md).

`deploy.yml` roda as migrations no boot do backend (advisory-lock, então o
worker pode subir junto sem corrida) e é seguro repetir. Antes de subir as
imagens, ele copia o banco para `<app_dir>/backups` e guarda as 5 últimas
cópias — a restauração está em [ci-cd.md](ci-cd.md#cópia-do-banco-antes-de-cada-deploy).

---

## 🗄️ Estrear esta versão num servidor que já rodou outra

O banco de produção veio da linhagem anterior do projeto (`annyGo`), cujo
modelo é outro: o cronograma materializado, o registro por atividade e o
caderno não existiam naquele schema.

A numeração das migrations recomeçou aqui, então aquele banco tem a **versão 1
registrada** em `schema_migrations` sem nunca ter visto esta baseline. O runner
concluiria que não há nada a aplicar e o backend subiria contra o schema
errado — e como `/health` só dá ping no banco, o deploy passaria verde com
todas as telas quebradas.

Por isso o runner recusa: quando as migrations constam como aplicadas e a
tabela `atividades` não existe, ele falha com `banco de outra linhagem`, o
health não responde e o `deploy.yml` restaura a versão anterior sozinho.

Para estrear, o volume antigo sai e o Postgres cria o banco do zero — mesmos
nomes, dados novos. **Uma vez, na VPS, e nunca pela pipeline:**

```bash
ssh annyGo@SEU_IP
cd /opt/annygo

# 1. leve um dump antes, mesmo que os dados não sirvam mais aqui:
#    é o que permite consultar o que existia se faltar alguma coisa depois.
docker compose exec -T postgres pg_dumpall -U annygo > ~/annygo-$(date +%F).sql

# 2. derruba a aplicação e apaga SÓ o volume do banco
docker compose down
docker volume rm annygo_postgres_data
```

O volume `annygo_edital_work` é cache do processador de editais e pode ficar.
Na próxima tag, `deploy_production` sobe a aplicação, o Postgres cria o banco
vazio e a baseline aplica.

Os dados voltam pelo CSV: exporte no app antigo antes de derrubá-lo e importe
em **Ajustes → Importar de uma planilha** depois que a conta estiver criada. O
que o CSV não leva é a conta em si (cadastre de novo).

---

## 🔑 Segredos e variáveis

| | Onde | Arquivo | Contém |
|---|---|---|---|
| 🗒️ | Inventário | `ansible/inventory/<ambiente>/hosts.ini` (gitignored) | IP da VPS, usuário, chave |
| 🔐 | Segredos | `ansible/inventory/<ambiente>/group_vars/app/secrets.yml` (gitignored) | `jwt_secret`, `postgres_password`, `letsencrypt_email`, `gemini_api_key`, `edital_processor_token` |
| 📄 | Não-secreto | `ansible/inventory/<ambiente>/group_vars/app/main.yml` (versionado) | `app_domain`, portas, nome do banco |

`<ambiente>` é `staging` ou `production`: cada um tem inventário e segredos
próprios, e nenhum comando escolhe um deles por omissão.

## ⚖️ Publicar uma lei

A lei não passa pela pipeline como código: ela é **dado**, importado pela
tela depois que a versão do app que a lê já está no ar. Enquanto o app é de
teste, qualquer conta logada importa (decisão de 25/09/2026).

1. Na sua máquina: `make leis-validar` e `make leis-pacote`.
2. Staging: **Legislação → Importar lei**, um pacote de
   `conteudo/leis/pacotes/` por vez. Abra a lei, confira um artigo com
   questões e o link direto (`/leis/cf88#art71`).
3. Produção: o mesmo, depois do `make release` da versão que trouxe a
   migration 000008.

Importar de novo o mesmo pacote não duplica nada; uma versão nova da lei
preserva as respostas das questões que continuam.

---

## ⚠️ Avisos

> [!CAUTION]
> **Nunca** rode `ssh-keygen` sobrescrevendo `~/.ssh/annygo_deploy`. O root SSH
> está desativado; se a chave for perdida, a recuperação é só pelo console web do
> provedor.

ℹ️ O `docker-compose.yml` da raiz é só para desenvolvimento local. Em produção o
Ansible gera o seu a partir de `ansible/templates/docker-compose.prod.yml.j2`.
