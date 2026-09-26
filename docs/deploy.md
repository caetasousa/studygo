# 🚢 O servidor

![Ansible](https://img.shields.io/badge/Ansible-provisionamento-EE0000?logo=ansible&logoColor=white)
![Ubuntu](https://img.shields.io/badge/Ubuntu-26.04_no_WSL-E95420?logo=ubuntu&logoColor=white)
![Cloudflare](https://img.shields.io/badge/Cloudflare-Tunnel-F38020?logo=cloudflare&logoColor=white)

> **Em uma frase:** este documento é sobre PREPARAR o servidor, não sobre
> publicar o sistema.
>
> Publicar é assunto de [ci-cd.md](ci-cd.md). Aqui está o que se faz uma vez
> por servidor: o acesso por SSH, o Docker, o nginx e o túnel que o põe na
> internet ([cloudflare-tunnel.md](cloudflare-tunnel.md)). Novo por aqui? Leia
> [como-funciona.md](como-funciona.md) antes.

O servidor é próprio: a distro **`ubuntu-server`** do WSL da mesma máquina de
desenvolvimento, publicada por um **túnel da Cloudflare**. As imagens Docker
são construídas **pela pipeline** e o servidor as baixa do Container Registry
por digest — o código-fonte nunca vai para lá.

```
GitLab.com                     esta máquina (Windows + WSL2)
┌─────────────┐   pull por     ┌───────────────────────────────────────────┐
│ Registry    │   digest       │ ubuntu-server                             │
│ imagens     │ ─────────────► │  cloudflared ─► nginx :8480               │
│ testadas    │                │                  └─► frontend ─► backend  │
└─────────────┘                │                    worker ─► postgres     │
       ▲                       │ Ubuntu-24.04 (desenvolvimento)            │
       └── pipeline ◄──────────│  runner da esteira, registry local        │
                               └───────────────────────────────────────────┘
```

Tudo mora em `ansible/`: o `bootstrap-wsl.sh` (o que vem antes de o Ansible
poder entrar), o `site.yml` (a infraestrutura) e o `deploy.yml` (a
aplicação, só pela pipeline), com uma role por peça: `common`,
`docker_rootless`, `nginx` e `cloudflared`.

---

## 🧭 Por que ele é assim

**Todas as distros do WSL2 dividem a mesma rede** — o mesmo IP, as mesmas
portas, o mesmo iptables. A distro de desenvolvimento roda o Docker do dia a
dia e o runner da esteira; o servidor não pode brigar com eles. Daí:

- **Docker rootless, um usuário por projeto.** O Docker do servidor é o do
  usuário `studygo`, com rede, imagens e volumes próprios. Um Docker de
  sistema brigaria com o de desenvolvimento pelo `docker0` e pelo iptables. Um
  próximo projeto no mesmo servidor ganha o seu usuário e não enxerga este.
- **Sem firewall local.** Um ufw com "deny by default" fecharia a rede do
  desenvolvimento e do runner. E não há o que fechar: nenhuma porta precisa
  abrir, porque o túnel só faz conexões de saída.
- **SSH na 2222.** A 22 fica livre na rede compartilhada. O job de deploy roda
  num container do runner e chega ao servidor por `172.17.0.1:2222`.
- **nginx na 8480**, atrás do túnel. Ele lê o IP do visitante do cabeçalho da
  Cloudflare (só vindo de localhost), para o limite de taxa continuar por
  pessoa — detalhes em [cloudflare-tunnel.md](cloudflare-tunnel.md).
- **O deploy confere o `/health` no próprio servidor**, e o `smoke_test` por
  `172.17.0.1:8480`: a esteira não depende de DNS nem do painel da Cloudflare.
- **Só fica no ar com o PC ligado** e a distro de pé.

Os valores ficam no inventário (`ansible/inventory/staging/group_vars/app/main.yml`):
`deploy_user`, `projeto`, `app_dir`, portas, `nginx_porta`, `cloudflared_rapido`.

---

## 1️⃣ Montar do zero

Pré-requisitos na distro de desenvolvimento: `ansible-core` e as coleções
(`ansible-galaxy collection install -r ansible/requirements.yml`), a chave
`~/.ssh/studygo_ci` (a da esteira) e o `ansible/.vault_pass`.

```bash
# 1. a distro do servidor (uma vez), com o systemd ligado em /etc/wsl.conf:
#    [boot]
#    systemd=true

# 2. bootstrap, como root na distro: pacotes, sshd na 2222 só por chave e o
#    usuário de deploy do projeto com a chave da esteira. Idempotente;
#    usuários a remover vão depois da chave.
wsl.exe -d ubuntu-server -u root -- bash -s -- \
  studygo "$(cat ~/.ssh/studygo_ci.pub)" \
  < ansible/bootstrap-wsl.sh

# 3. o inventário local e a chave do host
ssh-keyscan -p 2222 127.0.0.1 >> ~/.ssh/known_hosts
sed 's/SEU_IP_AQUI/127.0.0.1/' ansible/inventory/staging/hosts.ini.example \
  > ansible/inventory/staging/hosts.ini

# 4. provisionar: Docker rootless, nginx, túnel
make provision

# 5. a aplicação sobe pela pipeline — veja ci-cd.md
#    (o deploy.yml exige um digest já publicado e recusa rodar sem ele)
```

No GitLab (Settings → CI/CD → Variables, ambiente `staging`),
`SSH_KNOWN_HOSTS` recebe a saída de `ssh-keyscan -p 2222 172.17.0.1`: é dela
que o job tira o endereço do servidor.

O túnel com token (para um domínio próprio) usa o `cloudflared_token` do
`secrets.yml`; o passo a passo está em [cloudflare-tunnel.md](cloudflare-tunnel.md).

Para o servidor voltar sozinho quando o Windows liga, uma tarefa agendada
mantém a distro de pé (no PowerShell, uma vez):

```powershell
schtasks /Create /TN "studygo-servidor" /SC ONLOGON /RL LIMITED /F `
  /TR "conhost.exe --headless wsl.exe -d ubuntu-server --exec sleep infinity"
```

---

## 🔁 Atualizações

O dia a dia, da raiz do repositório:

```bash
make provision                  # mudou infra (nginx, túnel, Docker…)
make provision tags=nginx       # só uma peça, sem tocar no resto
make servidor-health            # a aplicação responde? que versão está no ar?
make servidor-status            # containers
make servidor-logs svc=backend  # logs
make servidor-endereco          # o endereço público da vez
```

Sem `tags`, o `site.yml` roda tudo, inclusive o `apt upgrade` da role
`common`. Ele é idempotente: rodar de novo num servidor pronto não muda nada.

A **aplicação** não sobe por aqui em nenhuma hipótese: quem publica é a
pipeline, no push da `main` ([ci-cd.md](ci-cd.md)).

O backend roda as migrations no boot (com advisory lock, então o worker pode
subir junto sem corrida), e o `deploy.yml` é seguro de repetir. Antes de subir
as imagens, ele copia o banco para `<app_dir>/backups` e guarda as 5 últimas
cópias — a restauração está em [ci-cd.md](ci-cd.md).

---

## 🔑 Segredos e variáveis

| | Arquivo | Contém |
|---|---|---|
| 🗒️ | `ansible/inventory/staging/hosts.ini` (gitignored) | endereço, porta, usuário e chave — gerado do `.example` |
| 🔐 | `ansible/inventory/staging/group_vars/app/secrets.yml` (gitignored, Vault) | `jwt_secret`, `postgres_password`, `gemini_api_key`, `edital_processor_token`, `cloudflared_token` |
| 📄 | `ansible/inventory/staging/group_vars/app/main.yml` (versionado) | usuário, portas, diretório, banco, túnel |

O `secrets.yml` local é a fonte, mas **a pipeline não o lê**: ela recebe o
conteúdo cifrado pela variável `ANSIBLE_SECRETS` do GitLab. Mudou um segredo
que o deploy usa? `ansible-vault edit`, cole o conteúdo cifrado na variável
(com *Expand variable reference* desmarcado) e rode a pipeline de novo.

---

## ⚖️ Publicar uma lei

A lei não passa pela pipeline como código: ela é **dado**, importado pela
tela depois que a versão do app que a lê já está no ar. Enquanto o app é de
teste, qualquer conta logada importa.

1. Na sua máquina: `make leis-validar` e `make leis-pacote`.
2. No app: **Legislação → Importar lei**, um pacote de
   `conteudo/leis/pacotes/` por vez. Abra a lei, confira um artigo com
   questões e o link direto (`/leis/cf88#art71`).

Importar de novo o mesmo pacote não duplica nada; uma versão nova da lei
preserva as respostas das questões que continuam.

---

## ⚠️ Avisos

> [!CAUTION]
> Não instale ufw, fail2ban nem o Docker de sistema na `ubuntu-server`: a rede
> é a mesma da distro de desenvolvimento, e eles derrubariam o
> desenvolvimento e o runner da esteira.

ℹ️ O `docker-compose.yml` da raiz é só para desenvolvimento local. No servidor
o Ansible gera o seu a partir de `ansible/templates/docker-compose.prod.yml.j2`.
