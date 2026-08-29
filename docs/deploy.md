# Deploy numa VPS

O deploy roda numa VPS Ubuntu (o projeto usa uma Hostinger com Ubuntu 24.04 LTS),
provisionada e atualizada por **Ansible**. As imagens Docker são **buildadas na
sua máquina** e enviadas prontas — o código-fonte nunca vai para a VPS.

```
sua máquina                                   VPS
┌───────────────┐   docker save + scp   ┌──────────────────────────────────┐
│ docker build  │ ────────────────────► │ nginx (borda, HTTPS)             │
│ backend +     │                       │   └─► frontend  (SPA + proxy /api)│
│ frontend      │                       │        └─► backend ─► postgres    │
└───────────────┘                       │             worker ─┘            │
                                        │  certbot renova o cert           │
                                        └──────────────────────────────────┘
```

Tudo mora em `ansible/`. Detalhes de cada playbook e role estão em
[CLAUDE.md](../CLAUDE.md#infrastructure-ansible).

## Pré-requisitos na sua máquina

- **Ansible**
- **Docker** (para buildar as imagens localmente)
- `sshpass` — só no primeiro acesso à VPS
- Uma VPS Ubuntu com IP público e um domínio apontando para ela

## Primeira vez (por servidor)

> A única coisa que você digita à parte é a **senha de root da VPS** (a que o
> provedor gera). Ela é usada só no `bootstrap`/`lockdown` e nunca é salva.

```bash
cd ansible

# 1. chave SSH dedicada ao deploy (pule se já existir — NÃO sobrescreva)
ssh-keygen -t ed25519 -f ~/.ssh/annygo_deploy -N "" -C "annygo-deploy"

# 2. inventário e segredos (arquivos .example → reais; os reais são gitignored)
cp inventory/hosts.ini.example inventory/hosts.ini
#   edite: ansible_host = IP da VPS

cp inventory/group_vars/vps/secrets.yml.example inventory/group_vars/vps/secrets.yml
#   edite: letsencrypt_email, jwt_secret, postgres_password
#   opcional: gemini_api_key (liga a importação de edital por IA)

#   ajuste o domínio em inventory/group_vars/vps/main.yml (app_domain)

# 3. cria o usuário sudo com sua chave (pede a senha de root)
ansible-playbook bootstrap.yml -e ansible_user=root --ask-pass

# 4. desliga o login de root por SSH (última vez que a senha de root é usada)
ansible-playbook lockdown.yml -e ansible_user=root --ask-pass

# 5. provisiona a box: firewall, Docker, nginx, certificado HTTPS
ansible-playbook site.yml

# 6. builda as imagens, envia e sobe o stack
ansible-playbook deploy.yml
```

Ao final, `https://SEU-DOMINIO/health` responde `{"status":"ok"}`.

## Atualizações

`bootstrap.yml` e `lockdown.yml` rodam **uma vez** por servidor. Depois:

```bash
cd ansible
ansible-playbook deploy.yml   # mudou backend ou frontend → nova versão no ar
ansible-playbook site.yml     # mudou infra (nginx, firewall, cert…)
```

`deploy.yml` roda as migrations no boot do backend (advisory-lock, então o
worker pode subir junto sem corrida) e é seguro repetir.

## Segredos e variáveis

| Onde | Arquivo | Contém |
|---|---|---|
| Inventário | `ansible/inventory/hosts.ini` (gitignored) | IP da VPS, usuário, chave |
| Segredos | `ansible/inventory/group_vars/vps/secrets.yml` (gitignored) | `jwt_secret`, `postgres_password`, `letsencrypt_email`, `gemini_api_key` |
| Não-secreto | `ansible/inventory/group_vars/vps/main.yml` (versionado) | `app_domain`, portas, nome do banco |

## Avisos

- **Nunca** rode `ssh-keygen` sobrescrevendo `~/.ssh/annygo_deploy`. O root SSH
  está desativado; se a chave for perdida, a recuperação é só pelo console web do
  provedor.
- O `docker-compose.yml` da raiz é só para desenvolvimento local. Em produção o
  Ansible gera o seu a partir de `ansible/templates/docker-compose.prod.yml.j2`.
