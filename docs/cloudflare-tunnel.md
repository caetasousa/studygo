# ☁️ Túnel da Cloudflare

> **Em uma frase:** o servidor não tem porta aberta para a internet. Um
> programa dentro dele, o `cloudflared`, abre uma conexão de **saída** até a
> Cloudflare, e é por essa conexão que as visitas chegam.

Este documento registra como o túnel foi montado em 25/09/2026, como ele
funciona hoje e como operá-lo. O servidor em si (a distro `ubuntu-server` do
WSL, o Docker rootless, o Ansible) está em [deploy.md](deploy.md).

---

## 🗺️ Como a visita chega

```
navegador
   │  https://<endereço>
   ▼
Cloudflare  ─── termina o HTTPS (certificado da Cloudflare)
   │
   │  conexão que o próprio servidor abriu (saída, porta 7844)
   ▼
cloudflared  (serviço na ubuntu-server)
   │  http://127.0.0.1:8480
   ▼
nginx  ─── limite de taxa, cabeçalhos de segurança (CSP, HSTS…)
   │  http://127.0.0.1:15173
   ▼
frontend (SPA + proxy /api) ─► backend ─► postgres
```

Por que assim:

- **Nenhuma porta aberta.** O PC está atrás do roteador de casa, sem IP fixo,
  e o WSL nem publica portas na rede local. O túnel dispensa tudo isso: quem
  abre a conexão é o servidor.
- **HTTPS sem certbot.** O certificado é o da Cloudflare; não há Let's Encrypt
  para renovar.
- **O nginx continua na frente da aplicação.** É ele que aplica o limite de
  requisições por visitante e os cabeçalhos de segurança.

---

## 🔌 Os dois túneis

Há dois serviços `cloudflared` no servidor, com papéis diferentes:

| Serviço | Endereço | Situação |
|---|---|---|
| `cloudflared-studygo` | `https://<palavras-aleatórias>.trycloudflare.com` | **em uso**. Quick Tunnel: grátis, sem conta, sem domínio. **Muda a cada reinício** do serviço |
| `cloudflared` | nenhum, por enquanto | túnel **com token**, ligado à conta da Cloudflare, conectado e *Healthy*. Espera um domínio próprio para ter endereço fixo |

O endereço da vez:

```bash
make servidor-endereco
```

> [!WARNING]
> O Chrome e o Edge costumam marcar endereços `*.trycloudflare.com` como
> **perigosos**: o serviço é gratuito e anônimo, e golpistas o usam muito. Não
> é defeito do servidor — o HTTPS é válido. A correção é um domínio próprio
> (ver [Endereço fixo](#-endereço-fixo-com-domínio-próprio)).

---

## 🛠️ Como foi montado

### 1. Conta e túnel no painel da Cloudflare

1. Conta criada em <https://dash.cloudflare.com> (plano Free).
2. **Zero Trust → Networks → Tunnels → Create a tunnel**, tipo
   **Cloudflared**, com um nome (ex.: `studygo`).
3. Na tela de instalação, o painel só oferece **Debian**: serve para o
   Ubuntu, que é derivado dele e usa o mesmo pacote. Escolheu-se Debian,
   64-bit — mas **os comandos da tela não foram rodados**: quem instala é o
   Ansible (passo 3). Da tela só se aproveitou o **token**, a parte que começa
   com `eyJ…` no fim do comando `cloudflared service install eyJ…`.

### 2. O token no Vault

O token é uma credencial: quem o tem liga um conector ao seu túnel. Ele foi
gravado cifrado no `secrets.yml` do servidor, que não é versionado:

```bash
cd ansible
ansible-vault edit inventory/staging/group_vars/app/secrets.yml
# cloudflared_token: "eyJ..."
```

(Na montagem original, o token foi colado num arquivo `~/.cloudflared-token`,
copiado para o Vault sem ser exibido, e o arquivo pode ser apagado.)

### 3. O Ansible instala e liga

```bash
make provision tags=cloudflared
```

O papel `ansible/roles/cloudflared`:

1. adiciona o repositório de pacotes da Cloudflare (`pkg.cloudflare.com`,
   distribuição `any`) e instala o `cloudflared`;
2. com `cloudflared_token` definido, registra o túnel com token como o
   serviço de sistema `cloudflared` (`cloudflared service install <token>`,
   com `no_log`, para o token não aparecer na saída);
3. com `cloudflared_rapido: true` no inventário, cria o serviço
   `cloudflared-<projeto>` — o Quick Tunnel —, que roda
   `cloudflared tunnel --url http://127.0.0.1:8480`.

### 4. O nginx atrás do túnel

No vhost (`ansible/templates/app.conf.j2`):

- escuta na porta **8480** (`nginx_porta`), em todas as interfaces: o túnel
  chega por `127.0.0.1` e o `smoke_test` da esteira, num container do runner,
  por `172.17.0.1`. O WSL em modo NAT não publica essa porta na rede local;
- aceita **qualquer nome** (`server_name _`): só o túnel chega aqui, e quem
  decide que endereço aponta para ele é a Cloudflare;
- lê o IP do visitante do cabeçalho **`CF-Connecting-IP`**, e só quando a
  conexão vem de localhost (`set_real_ip_from 127.0.0.1`). Sem isso, toda
  visita chegaria como 127.0.0.1 — e o limite de tentativas de login valeria
  para o site inteiro de uma vez. Um container que chame por 172.17.0.1 não
  consegue se passar por outro IP;
- repassa o `X-Forwarded-Proto` que a Cloudflare informa (`https`).

### 5. A esteira não depende do túnel

O deploy confere o `/health` **no próprio servidor**, pelo nginx em
localhost, e o `smoke_test` por `172.17.0.1:8480`. Um problema na Cloudflare
ou um endereço novo do Quick Tunnel não derruba a pipeline.

---

## 👀 O que o painel mostra

Em **Zero Trust → Networks → Tunnels**, o túnel com token aparece assim:

| Campo | Significado |
|---|---|
| **Status: Healthy** | o conector está ligado |
| **Active replicas: 1** | um servidor conectado — a `ubuntu-server` |
| **Hostname** | o nome da máquina onde o conector roda (o PC) |
| **Edge Locations** (`gru…`) | as conexões com a Cloudflare, em São Paulo |
| **Routes: 0** | nenhum endereço aponta para ele ainda — esperado, sem domínio |

O Quick Tunnel **não aparece no painel**: é anônimo, não pertence à conta.

---

## 🌐 Endereço fixo com domínio próprio

Quando houver um domínio:

1. **Domínio na Cloudflare.** Comprado nela (Domain Registration → Register
   Domains; já nasce na conta), ou trazido de outro registrador (**Add a
   site**, plano Free, e trocar os nameservers no registrador pelos dois que
   ela indicar). Prefira `.com` ou `.com.br`: extensões muito baratas
   (`.xyz`, `.top`) também caem em aviso de site perigoso.
2. **Rota no túnel com token.** Zero Trust → Networks → Tunnels → o túnel →
   aba **Routes** (nas versões antigas do painel, *Public Hostname*) → novo
   endereço: o domínio (ou um subdomínio), tipo **HTTP**, URL
   **`localhost:8480`**. A Cloudflare cria o DNS sozinha.
3. **No repositório**, em `ansible/inventory/staging/group_vars/app/main.yml`:
   `app_domain` com o domínio novo e `cloudflared_rapido: false`; depois
   `make provision tags=cloudflared` desliga o Quick Tunnel.

---

## 🩺 Quando algo não funciona

| Sintoma | Causa provável | O que fazer |
|---|---|---|
| Página de erro 502 da Cloudflare | o túnel chegou ao nginx, mas a aplicação não está de pé | `make servidor-status`; se vazio, a pipeline ainda não implantou ou falhou |
| O endereço de ontem não abre | o Quick Tunnel reiniciou e trocou de endereço | `make servidor-endereco` |
| Nenhum endereço responde | a `ubuntu-server` está parada (PC reiniciado, WSL desligado) | `wsl.exe -l -v`; abra a distro |
| Túnel com token *Inactive* no painel | o serviço `cloudflared` caiu ou o token mudou | `sudo systemctl status cloudflared` no servidor; se o token mudou, atualize o Vault e rode `make provision tags=cloudflared` |
| "Site perigoso" no navegador | reputação do `trycloudflare.com` | domínio próprio |

Os logs, no servidor (`ssh -i ~/.ssh/studygo_ci -p 2222 studygo@127.0.0.1`):

```bash
sudo journalctl -u cloudflared-studygo -n 50   # Quick Tunnel (e o endereço)
sudo journalctl -u cloudflared -n 50           # túnel com token
sudo tail -f /var/log/nginx/access.log         # quem está chegando, com o IP real
```

---

## 🔐 Segurança

- **O token é segredo.** Fica só no Vault (e na unidade systemd do servidor).
  Vazou? Painel → o túnel → *Refresh token* (ou apague e recrie o túnel),
  atualize o Vault e rode o `make provision tags=cloudflared`.
- **Nenhuma porta de entrada.** Não há firewall local de propósito — a rede
  do WSL2 é a mesma da distro de desenvolvimento, e um "deny by default"
  fecharia as duas. O servidor só aceita o que chega pelo túnel ou de dentro
  da própria máquina.
- **O IP do visitante** só é aceito do cabeçalho da Cloudflare quando a
  conexão vem de localhost, onde só o túnel chega.
