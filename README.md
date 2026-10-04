# fvremote

Acesso remoto via rede **Tor** (hidden service `.onion`): visualizar a tela,
controlar mouse/teclado e transferir arquivos entre máquinas Linux e Windows.

O fvremote **baixa e usa um Tor standalone exclusivo** (pasta própria do app),
sem instalar, configurar ou tocar no Tor do sistema.

## Componentes

- **Servidor** (`cmd/server`): roda na máquina que será acessada. Sobe um
  hidden service Tor e expõe as rotas HTTP em `127.0.0.1:8080` (somente via Tor):
  - `GET  /info`     — resolução da tela e onion
  - `GET  /stream`   — frames JPEG da tela
  - `POST /input`    — eventos de mouse/teclado
  - `GET  /files`    — lista arquivos recebidos
  - `POST /upload`   — envia arquivo para o servidor
  - `GET  /download` — baixa arquivo do servidor
- **Cliente** (`cmd/client`): conecta ao `.onion` via SOCKS5, exibe a tela,
  envia mouse/teclado e gerencia upload/download.

## Onde ficam os dados

| Plataforma | Pasta |
|---|---|
| Linux   | `~/.config/fvremote/` |
| Windows | `%APPDATA%\fvremote\` |

O Tor standalone fica em `.../fvremote/tor/` (binário, dados e hidden service).
Arquivos recebidos ficam em `.../fvremote/received/`.

## Build

Requer Go 1.23+ e CGO habilitado (Fyne, robotgo e screenshot usam CGO).

Linux (X11):

```bash
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev \
  libxtst-dev libx11-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev
make linux
```

Windows (cross-compile a partir do Linux):

```bash
sudo apt-get install -y mingw-w64
make windows
```

Ou compilando nativamente no Windows com MinGW/GCC:

```powershell
go build -o fvremote-server.exe ./cmd/server
go build -o fvremote-client.exe ./cmd/client
```

## Uso

1. Na máquina alvo, rode `fvremote-server`. Na primeira execução o Tor é
   baixado. Aguarde aparecer o endereço `.onion`.
2. Na máquina de controle, rode `fvremote-client`, cole o `.onion` e clique em
   **Conectar**.
3. Use a aba **Tela** para visualizar/controlar e **Arquivos** para
   enviar/baixar.

## Releases

O workflow `release.yml` compila Linux e Windows em runners nativos e publica
um GitHub Release ao empurrar uma tag `v*`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Artefatos: `fvremote-linux-amd64.tar.gz` e `fvremote-windows-amd64.zip`.

## Avisos

- A captura de tela no Linux usa X11. Em Wayland a captura pode não funcionar
  sem um portal compatível.
- Use apenas em máquinas que você possui ou tem autorização para acessar.
