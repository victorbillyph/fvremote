# fvremote

Acesso remoto **descentralizado via rede Tor**, em um único aplicativo para
Linux e Windows — **Cliente** e **Suporte** ao mesmo tempo, sem servidor central.
A interface usa o **webview nativo do sistema** (WebView2 no Windows,
WebKitGTK no Linux), **sem OpenGL**.

O fvremote **baixa e usa um Tor standalone exclusivo**, na própria pasta do app,
sem instalar nem tocar no Tor do sistema.

## Como funciona

1. Cada instalação gera um **código numérico de 19 dígitos**.
2. Esse código é transformado, de forma determinística, em um endereço `.onion`
   (chave ed25519 derivada do código via `ADD_ONION` do Tor).
3. Quem tem o código **encontra o Cliente na rede Tor** e pede conexão — sem
   servidor de diretório.
4. O Cliente vê "*Fulano (host) quer se conectar. Aceitar?*".
5. Ao aceitar, o acesso começa como **somente leitura** (apenas ver a tela).
   O Cliente pode liberar **controle total** (teclado/mouse) ou desconectar.

## Interface

- **Abas** no topo: *Receber suporte* e *Prestar suporte*.
- Cada sessão abre em **aba própria**, com a tela ocupando toda a área e uma
  **barra superior** com: status, *Capturar mouse*, *Arquivos*, *Tela cheia* e
  *Desconectar*.
- Com **acesso total**, *Capturar mouse* ativa o **pointer lock** (cursor
  preso/oculto) e envia movimento/clique ao Cliente.
- Navegador de arquivos em painel lateral.

## Onde ficam os dados

| Plataforma | Pasta |
|---|---|
| Linux | `~/.config/fvremote/` |
| Windows | `%APPDATA%\fvremote\` |

- `tor/` — Tor standalone e seu data directory (exclusivo do fvremote).
- `identity.json` — o código único e a chave derivada (permissão `0600`).
- `downloads/` / `received/` — arquivos transferidos.

## Build

Requer Go 1.26+, o CLI do Wails e (no Linux) GTK3 + WebKit2GTK.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
```

Linux (Ubuntu 24.04+/26.04 usa WebKit2GTK 4.1):

```bash
sudo apt-get install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
  libx11-dev libxtst-dev libxrandr-dev libxcursor-dev libxinerama-dev \
  libxi-dev libxfixes-dev
make build          # wails build -s -skipbindings -m -nosyncgomod -tags webkit2_41
# binário em build/bin/fvremote
```

Windows: compile **nativamente** no Windows (o Wails não faz cross-compile):

```powershell
wails build -s -skipbindings -m -nosyncgomod
# binário em build\bin\fvremote.exe
```

O frontend fica em `frontend/dist` (HTML/CSS/JS puro, sem etapa de build npm) e é
embutido no binário.

## Testes

```bash
make test    # unitários (identidade e sessões)
make e2e     # real: sobe Tor e conecta via onion (requer rede)
```

## Releases

O workflow `release.yml` compila Linux e Windows em runners nativos e publica
um GitHub Release ao empurrar uma tag `v*`:

```bash
git tag v0.4.0
git push origin v0.4.0
```

Artefatos: `fvremote-linux-amd64.tar.gz` e `fvremote-windows-amd64.zip`.

## Segurança e avisos

- O código do Cliente **é** a chave do serviço oculto derivado. Quem conhece o
  código pode calcular o `.onion` e *tentar* conectar, mas só entra com a
  **aceitação explícita** do Cliente. Trate o código como segredo; apagar
  `identity.json` gera um novo.
- Linux: a captura de tela usa X11 (Wayland pode exigir portal compatível).
- Windows: requer o **WebView2 Runtime** (já presente no Windows 10/11).
- Use apenas em máquinas que você possui ou tem autorização para acessar.
