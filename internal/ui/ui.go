package ui

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/victorbillyph/fvremote/internal/hub"
	"github.com/victorbillyph/fvremote/internal/identity"
	"github.com/victorbillyph/fvremote/internal/remote"
	"github.com/victorbillyph/fvremote/internal/torx"
)

const version = "0.2.0"

// App mantém o estado do fvremote (é Cliente e Suporte ao mesmo tempo).
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	id   *identity.Identity
	name string
	host string

	tor       *torx.Runner
	hub       *hub.Hub
	httpSrv   *http.Server
	localPort int

	// widgets de setup
	setupMsg *widget.Label
	setupBar *widget.ProgressBar

	// widgets principais
	codeLb      *widget.Label
	statusLb    *widget.Label
	statusDot   *canvas.Circle
	busy        *widget.ProgressBarInfinite
	connectEnt  *widget.Entry
	sessionsBox *fyne.Container
	remoteTabs  *container.AppTabs
	remoteScr   *remoteCanvas
	files       *filesTab

	// estado do Suporte
	rem        *remote.Remote
	sessionID  string
	permission string
	streaming  bool
	remoteW    int
	remoteH    int
}

// Run inicia a aplicação.
func Run() {
	a := app.NewWithID("io.github.victorbillyph.fvremote")
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow("fvremote")
	w.Resize(fyne.NewSize(1060, 720))
	w.CenterOnScreen()

	app := &App{fyneApp: a, win: w}
	app.name, app.host = profile()

	w.SetContent(app.setupContent())
	w.Show()
	go app.bootstrap()
	a.Run()
}

func profile() (name, host string) {
	name = os.Getenv("USER")
	if name == "" {
		name = os.Getenv("USERNAME")
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	host, _ = os.Hostname()
	if name == "" {
		name = host
	}
	return
}

func (a *App) setupContent() fyne.CanvasObject {
	title := canvas.NewText("fvremote", colPrimary)
	title.TextSize = 40
	title.TextStyle = fyne.TextStyle{Bold: true}
	sub := canvas.NewText("acesso remoto descentralizado via Tor", colMuted)
	sub.TextSize = 14

	a.setupMsg = widget.NewLabel("Iniciando…")
	a.setupMsg.Alignment = fyne.TextAlignCenter
	a.setupBar = widget.NewProgressBar()

	box := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(sub),
		widget.NewSeparator(),
		container.NewCenter(a.setupMsg),
		a.setupBar,
	)
	return container.NewCenter(container.NewPadded(box))
}

func (a *App) setSetup(msg string, pct int) {
	fyne.Do(func() {
		a.setupMsg.SetText(msg)
		a.setupBar.SetValue(float64(pct) / 100)
	})
}

func (a *App) fatal(err error) {
	fyne.Do(func() {
		a.setupMsg.SetText("Erro: " + err.Error())
		if a.setupBar != nil {
			a.setupBar.SetValue(0)
		}
		dialog.ShowError(err, a.win)
	})
}

func (a *App) bootstrap() {
	id, err := identity.LoadOrCreate()
	if err != nil {
		a.fatal(err)
		return
	}
	a.id = id

	r, err := torx.Start(func(stage string, pct int) {
		switch stage {
		case "download":
			if pct > 0 {
				a.setSetup(fmt.Sprintf("Baixando Tor (exclusivo do fvremote)… %d%%", pct), pct)
			} else {
				a.setSetup("Baixando Tor (exclusivo do fvremote)…", 0)
			}
		case "bootstrap":
			a.setSetup(fmt.Sprintf("Conectando à rede Tor… %d%%", pct), pct)
		case "ready":
			a.setSetup("Teste de conexão Tor concluído.", 100)
		}
	})
	if err != nil {
		a.fatal(err)
		return
	}
	a.tor = r

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		a.fatal(err)
		return
	}
	a.localPort = ln.Addr().(*net.TCPAddr).Port
	a.hub = hub.New(a.name, a.host, a.id.Code, version)
	a.hub.OnRequest = func(s *hub.Session) { fyne.Do(func() { a.showRequest(s) }) }
	a.hub.OnChange = func() { fyne.Do(a.refreshSessions) }
	a.httpSrv = &http.Server{Handler: a.hub.Handler(), ReadHeaderTimeout: 15 * time.Second}
	go func() { _ = a.httpSrv.Serve(ln) }()

	a.setSetup("Publicando seu endereço .onion…", 100)
	sid, err := a.tor.AddOnion(a.id.TorBlob, 80, a.localPort)
	if err != nil {
		a.fatal(err)
		return
	}
	if want := strings.TrimSuffix(a.id.Onion, ".onion"); sid != want {
		a.fatal(fmt.Errorf("endereço onion divergente: tor=%s esperado=%s", sid, want))
		return
	}

	a.setSetup("Pronto!", 100)
	time.Sleep(500 * time.Millisecond)
	fyne.Do(func() { a.win.SetContent(a.mainContent()) })
}

func (a *App) mainContent() fyne.CanvasObject {
	title := canvas.NewText("fvremote", colFg)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	sub := canvas.NewText("acesso remoto descentralizado via Tor", colMuted)
	sub.TextSize = 12
	a.statusDot = canvas.NewCircle(colOk)
	a.statusDot.Resize(fyne.NewSize(10, 10))
	online := widget.NewLabel("online")
	header := container.NewBorder(nil, nil, container.NewVBox(title, sub),
		container.NewHBox(a.statusDot, online))

	// --- Aba: Receber suporte ---
	a.codeLb = widget.NewLabel(a.id.Code)
	a.codeLb.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	a.codeLb.Alignment = fyne.TextAlignCenter

	copyBtn := widget.NewButton("Copiar código", func() {
		a.fyneApp.Clipboard().SetContent(a.id.Code)
		a.setStatus("Código copiado para a área de transferência.", "ok")
	})

	onionLb := widget.NewLabel(a.id.Onion)
	onionLb.TextStyle = fyne.TextStyle{Monospace: true}
	onionLb.Wrapping = fyne.TextWrapBreak

	myTop := container.NewVBox(
		widget.NewLabelWithStyle("Seu código único", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		a.codeLb,
		container.NewCenter(copyBtn),
		widget.NewLabelWithStyle("Compartilhe este código com quem vai prestar o suporte.\nEle encontra você na rede Tor, sem servidor central.", fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Endereço .onion derivado", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
		onionLb,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Suportes conectados", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	a.sessionsBox = container.NewVBox()
	myTab := container.NewVScroll(container.NewVBox(myTop, a.sessionsBox))

	// --- Aba: Prestar suporte ---
	a.connectEnt = widget.NewEntry()
	a.connectEnt.SetPlaceHolder("Cole o código do Cliente (ex.: XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX)")
	connectBtn := widget.NewButton("Conectar", a.doConnect)
	disconnectBtn := widget.NewButton("Desconectar", a.doDisconnect)

	a.statusLb = widget.NewLabel("Pronto para conectar.")
	a.busy = widget.NewProgressBarInfinite()
	a.busy.Hide()

	connectTop := container.NewVBox(
		container.NewBorder(nil, nil, nil, connectBtn, a.connectEnt),
		container.NewHBox(a.statusLb, a.busy, disconnectBtn),
	)

	a.remoteScr = newRemoteCanvas(a)
	a.files = newFilesTab(a)
	a.remoteTabs = container.NewAppTabs(
		container.NewTabItem("Tela", a.remoteScr),
		container.NewTabItem("Arquivos", a.files.object()),
	)
	connectTab := container.NewBorder(container.NewPadded(connectTop), nil, nil, nil, a.remoteTabs)

	tabs := container.NewAppTabs(
		container.NewTabItem("Receber suporte", myTab),
		container.NewTabItem("Prestar suporte", connectTab),
	)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, tabs)
}

// ---- status ----

func (a *App) setStatus(text, kind string) {
	if a.statusLb == nil {
		return
	}
	a.statusLb.SetText(text)
	if a.statusDot == nil {
		return
	}
	switch kind {
	case "ok":
		a.statusDot.FillColor = colOk
	case "warn":
		a.statusDot.FillColor = colWarn
	case "err":
		a.statusDot.FillColor = colErr
	default:
		a.statusDot.FillColor = colPrimary
	}
	a.statusDot.Refresh()
}

func (a *App) setBusy(b bool) {
	if a.busy == nil {
		return
	}
	if b {
		a.busy.Show()
		a.busy.Start()
	} else {
		a.busy.Stop()
		a.busy.Hide()
	}
}

// ---- solicitações recebidas (lado Cliente) ----

func (a *App) showRequest(s *hub.Session) {
	msg := widget.NewLabel(fmt.Sprintf("%s (%s) quer se conectar ao seu computador.\n\nO acesso inicia como somente leitura; você pode liberar o controle total depois.", s.SupportName, s.SupportHost))
	msg.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm("Solicitação de conexão", "Aceitar", "Rejeitar", msg, func(accept bool) {
		if accept {
			a.hub.Accept(s.ID)
			a.setStatus("Suporte conectado — somente leitura.", "ok")
		} else {
			a.hub.Reject(s.ID)
			a.setStatus("Solicitação recusada.", "warn")
		}
	}, a.win)
	d.Resize(fyne.NewSize(440, 220))
	d.Show()
}

func (a *App) refreshSessions() {
	if a.sessionsBox == nil || a.hub == nil {
		return
	}
	active := a.hub.Active()
	a.sessionsBox.RemoveAll()
	if len(active) == 0 {
		a.sessionsBox.Add(widget.NewLabelWithStyle("Nenhum suporte conectado.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}
	for _, s := range active {
		s := s
		title := widget.NewLabelWithStyle(fmt.Sprintf("%s (%s)", s.SupportName, s.SupportHost), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		perm := widget.NewLabel("Acesso: " + permPortuguese(s.Permission))
		var toggle *widget.Button
		if s.Permission == hub.PermFull {
			toggle = widget.NewButton("Voltar para somente leitura", func() {
				a.hub.SetPermission(s.ID, hub.PermView)
			})
			perm.SetText("Acesso: controle total")
		} else {
			toggle = widget.NewButton("Dar acesso total", func() {
				a.hub.SetPermission(s.ID, hub.PermFull)
			})
		}
		disconnect := widget.NewButton("Desconectar", func() { a.hub.Disconnect(s.ID) })
		card := container.NewVBox(title, perm, container.NewHBox(toggle, disconnect), widget.NewSeparator())
		a.sessionsBox.Add(container.NewPadded(card))
	}
	a.sessionsBox.Refresh()
}

func permPortuguese(p string) string {
	if p == hub.PermFull {
		return "controle total"
	}
	return "somente leitura"
}

// ---- fluxo do Suporte ----

func (a *App) doConnect() {
	if a.tor == nil {
		dialog.ShowInformation("fvremote", "A rede Tor ainda está iniciando.", a.win)
		return
	}
	code := strings.TrimSpace(a.connectEnt.Text)
	if code == "" {
		dialog.ShowError(fmt.Errorf("informe o código do Cliente"), a.win)
		return
	}
	onion, err := identity.DeriveFromCode(code)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.setBusy(true)
	a.setStatus("Achando o Cliente na rede Tor…", "info")

	go func() {
		rem, err := remote.New(a.tor.SocksAddr(), onion)
		if err != nil {
			a.failConnect(err)
			return
		}
		var h *hub.Hello
		for i := 0; i < 3; i++ {
			h, err = rem.Hello()
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			a.failConnect(fmt.Errorf("não foi possível alcançar o Cliente (ele pode estar offline)"))
			return
		}
		a.rem = rem
		a.remoteW, a.remoteH = h.Width, h.Height

		fyne.Do(func() { a.setStatus("Conectando…", "info") })
		id, err := rem.Connect(hub.ConnectReq{Name: a.name, Host: a.host, Code: a.id.Code})
		if err != nil {
			a.failConnect(err)
			return
		}
		a.sessionID = id
		a.permission = hub.PermView
		fyne.Do(func() { a.setStatus("Pedindo autorização ao Cliente…", "warn") })
		a.poll()
	}()
}

func (a *App) failConnect(err error) {
	fyne.Do(func() {
		a.setBusy(false)
		a.setStatus(err.Error(), "err")
	})
	a.rem = nil
	a.sessionID = ""
	a.streaming = false
}

func (a *App) poll() {
	id := a.sessionID
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if a.sessionID != id || a.rem == nil {
			return
		}
		s, err := a.rem.Poll(id)
		if err != nil {
			a.failConnect(err)
			return
		}
		switch s.Status {
		case hub.StatusRejected:
			a.failConnect(fmt.Errorf("conexão recusada pelo Cliente"))
			return
		case hub.StatusAccepted:
			if s.Permission != a.permission {
				a.permission = s.Permission
				fyne.Do(func() { a.applyPermission(s.Permission) })
			}
			if !a.streaming {
				a.streaming = true
				fyne.Do(func() {
					a.setBusy(false)
					a.applyPermission(s.Permission)
					a.remoteTabs.SelectIndex(0)
				})
				a.startStream()
			}
		}
	}
}

func (a *App) applyPermission(perm string) {
	if perm == hub.PermFull {
		a.setStatus("Conectado — controle total liberado pelo Cliente.", "ok")
	} else {
		a.setStatus("Conectado — somente leitura (peça ao Cliente para liberar o controle).", "ok")
	}
}

func (a *App) startStream() {
	body, err := a.rem.Stream(a.sessionID)
	if err != nil {
		a.failConnect(err)
		return
	}
	go a.remoteScr.readFrames(body)
}

func (a *App) doDisconnect() {
	if a.rem != nil && a.sessionID != "" {
		a.rem.Bye(a.sessionID)
	}
	a.rem = nil
	a.sessionID = ""
	a.permission = ""
	a.streaming = false
	a.setBusy(false)
	a.setStatus("Desconectado.", "info")
}
