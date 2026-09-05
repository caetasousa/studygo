# 🚢 Deploy numa VPS

![Ansible](https://img.shields.io/badge/Ansible-deploy-EE0000?logo=ansible&logoColor=white)
![Ubuntu](https://img.shields.io/badge/Ubuntu-24.04_LTS-E95420?logo=ubuntu&logoColor=white)
![nginx](https://img.shields.io/badge/nginx-TLS-009639?logo=nginx&logoColor=white)

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
`certbot`).

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

# 2. inventário e segredos (arquivos .example → reais; os reais são gitignored)
cp inventory/hosts.ini.example inventory/hosts.ini
#   edite: ansible_host = IP da VPS

cp inventory/group_vars/vps/secrets.yml.example inventory/group_vars/vps/secrets.yml
#   edite: letsencrypt_email, jwt_secret, postgres_password
#   opcional: gemini_api_key + edital_processor_token (liga a importação de edital por IA)

#   ajuste o domínio em inventory/group_vars/vps/main.yml (app_domain)

# 3. cria o usuário sudo com sua chave (pede a senha de root)
ansible-playbook bootstrap.yml -e ansible_user=root --ask-pass

# 4. desliga o login de root por SSH (última vez que a senha de root é usada)
ansible-playbook lockdown.yml -e ansible_user=root --ask-pass

# 5. provisiona a box: firewall, Docker, nginx, certificado HTTPS
ansible-playbook site.yml

# 6. a aplicação sobe pela pipeline — veja ci-cd.md
#    (o deploy.yml exige um digest já publicado e recusa rodar sem ele)
```

Ao final, `https://SEU-DOMINIO/health` responde `{"status":"ok"}`.

---

## 🔁 Atualizações

`bootstrap.yml` e `lockdown.yml` rodam **uma vez** por servidor. O dia a dia
depois disso é pelo `make`, da raiz do repositório:

```bash
make provision  # mudou infra (nginx, firewall, cert…)
make health     # confirma que respondeu
```

Publicar a aplicação é da pipeline: push na `main` implanta em staging, e uma
tag `v*` libera o botão manual de produção. Veja [ci-cd.md](ci-cd.md). O fluxo
completo (desenvolver, verificar, commitar, publicar) está em
[fluxo-de-trabalho.md](fluxo-de-trabalho.md).

`deploy.yml` roda as migrations no boot do backend (advisory-lock, então o
worker pode subir junto sem corrida) e é seguro repetir.

---

## 🔑 Segredos e variáveis

| | Onde | Arquivo | Contém |
|---|---|---|---|
| 🗒️ | Inventário | `ansible/inventory/hosts.ini` (gitignored) | IP da VPS, usuário, chave |
| 🔐 | Segredos | `ansible/inventory/group_vars/vps/secrets.yml` (gitignored) | `jwt_secret`, `postgres_password`, `letsencrypt_email`, `gemini_api_key`, `edital_processor_token` |
| 📄 | Não-secreto | `ansible/inventory/group_vars/vps/main.yml` (versionado) | `app_domain`, portas, nome do banco |

---

## ⚠️ Avisos

> [!CAUTION]
> **Nunca** rode `ssh-keygen` sobrescrevendo `~/.ssh/annygo_deploy`. O root SSH
> está desativado; se a chave for perdida, a recuperação é só pelo console web do
> provedor.

ℹ️ O `docker-compose.yml` da raiz é só para desenvolvimento local. Em produção o
Ansible gera o seu a partir de `ansible/templates/docker-compose.prod.yml.j2`.
