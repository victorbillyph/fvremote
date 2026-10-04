# fvremote

Acesso remoto **descentralizado via rede Tor**, em um único aplicativo para
Linux e Windows. Cada instalação é, ao mesmo tempo, **Cliente** (pode ser
assistida) e **Suporte** (pode assistir), sem nenhum servidor central.

O fvremote **baixa e usa um Tor standalone exclusivo**, na própria pasta do app,
sem instalar nem tocar no Tor do sistema.

## Como funciona

1. Cada instalação gera um **código único** (`XXXX-XXXX-...`).
2. Esse código é transformado, de forma determinística, em um endereço
   `.onion` (chave ed25519 derivada do código via `ADD_ONION` do Tor).
3. Quem tem o código consegue **encontrar o Cliente na rede Tor** e pedir
   conexão — sem servidor de diretório.
4. O Cliente vê um diálogo "*Fulano (host) quer se conectar. Aceitar?*".
5. Ao aceitar, o acesso começa como **somente leitura** (apenas ver a tela).
   O Cliente pode, a qualquer momento, liberar **controle total**
   (teclado e mouse) ou desconectar.

## Fluxo de uso

| Papel | Ação |
|---|---|
| **Cliente** (quem recebe) | Abre o app, informa o **código** ao Suporte. |
| **Suporte** (quem presta) | Aba *Prestar suporte*, cola o código e clica **Conectar**. |
| Suporte | Vê os estados: *Achando → Conectando → Pedindo autorização → Conectado*. |
| Cliente | Aceita/Rejeita e depois pode dar **controle total**. |

## Recursos

- Descoberta **descentralizada** por código (onion derivado, sem servidor).
- Visualização da tela em tempo real.
- Controle de mouse e teclado quando liberado (acesso total).
- Transferência de arquivos (envio/baixo) com acesso total.
- Setup automático na primeira execução: baixa o Tor, conecta e testa a rede.
- Interface escura, animada e responsiva (Fyne), em um binário único.

## Onde ficam os dados

| Plataforma | Pasta |
|---|---|
| Linux | `~/.config/fvremote/` |
| Windows | `%APPDATA%\fvremote\` |

- `tor/` — Tor standalone e seu data directory (exclusivo do fvremote).
- `identity.json` — o código único e a chave derivada (permissão `0600`).
- `downloads/` — arquivos baixados pelo Suporte.
- `received/` — arquivos enviados pelo Suporte.

## Build

Requer Go 1.26+ e CGO (Fyne, robotgo e screenshot usam CGO).

Linux (X11):

```bash
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev \
  libxkbcommon-x11-dev libxtst-dev libx11-dev libxrandr-dev libxcursor-dev \
  libxinerama-dev libxi-dev libxfixes-dev libwayland-dev wayland-protocols libdecor-0-dev
make linux
```

Windows (cross-compile a partir do Linux, via mingw-w64) ou nativo no Windows:

```bash
make windows
```

## Testes

```bash
make test          # testes unitários (identidade e sessões)
make e2e           # teste real: sobe Tor e conecta via onion (requer rede)
```

## Releases

O workflow `release.yml` compila Linux e Windows em runners nativos e publica
um GitHub Release ao empurrar uma tag `v*`:

```bash
git tag v0.2.0
git push origin v0.2.0
```

Artefatos: `fvremote-linux-amd64.tar.gz` e `fvremote-windows-amd64.zip`.

## Segurança e avisos

- O código do Cliente **é** a chave do serviço oculto derivado. Quem conhece o
  código pode calcular o endereço `.onion` e *tentar* conectar — mas a conexão
  só é efetivada com a **aceitação explícita** do Cliente. Trate o código como
  segredo e regenere-o apagando `identity.json` se necessário.
- A captura de tela no Linux usa X11. Em Wayland pode não funcionar sem um
  portal compatível.
- Use apenas em máquinas que você possui ou tem autorização para acessar.
