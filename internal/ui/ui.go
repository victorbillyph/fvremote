package ui

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"strings"
	"sync"
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

const version = "0.3.0"

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
	busy        *widget.ProgressBarInfinite
	connectEnt  *widget.Entry
	sessionsBox *fyne.Container
	mainTabs    *container.AppTabs

	fullScreen bool

	mu       sync.Mutex
	sessions map[string]*supportSession
}

// supportSession é uma sessão de suporte (um Cliente sendo assistido).
type supportSession struct {
	app        *App
	rem        *remote.Remote
	id         string
	clientName string
	clientHost string
	clientCode string

	permission string
	streaming  bool
	captured   bool
	remoteW    int
	remoteH    int
	closed     bool

	canvas     *remoteCanvas
	tab        *container.TabItem
	clientLb   *widget.Label
	statusTxt  *canvas.Text
	captureBtn *widget.Button
	fsBtn      *widget.Button
	filesWin   fyne.Window
	files      *filesTab
	pollStop   chan struct{}
}

// Run inicia a aplicação.
func Run() {
	a := app.NewWithID("io.github.victorbillyph.fvremote")
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow("fvremote")
	w.Resize(fyne.NewSize(1100, 740))
	w.CenterOnScreen()

	app := &App{fyneApp: a, win: w, sessions: map[string]*supportSession{}}
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
			a.setSetup(fmt.Sprintf("Baixando Tor (exclusivo do fvremote)… %d%%", pct), pct)
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
	header := container.NewBorder(nil, nil, container.NewVBox(title, sub), nil)

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
	a.connectEnt.SetPlaceHolder("Digite os 19 dígitos do código do Cliente")
	connectBtn := widget.NewButton("Conectar", a.doConnect)
	a.connectEnt.OnSubmitted = func(string) { a.doConnect() }

	a.statusLb = widget.NewLabel("Pronto para conectar.")
	a.busy = widget.NewProgressBarInfinite()
	a.busy.Hide()

	connectTop := container.NewVBox(
		container.NewBorder(nil, nil, nil, connectBtn, a.connectEnt),
		container.NewHBox(a.statusLb, a.busy),
		widget.NewLabelWithStyle("Ao conectar, a sessão abre em uma aba própria com a tela em tela cheia.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)
	connectTab := container.NewBorder(container.NewPadded(connectTop), nil, nil, nil)

	a.mainTabs = container.NewAppTabs(
		container.NewTabItem("Receber suporte", myTab),
		container.NewTabItem("Prestar suporte", connectTab),
	)
	return container.NewBorder(container.NewPadded(header), nil, nil, nil, a.mainTabs)
}

// ---- status ----

func (a *App) setStatus(text, kind string) {
	if a.statusLb == nil {
		return
	}
	a.statusLb.SetText(text)
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
	d.Resize(fyne.NewSize(460, 230))
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
			toggle = widget.NewButton("Voltar para somente leitura", func() { a.hub.SetPermission(s.ID, hub.PermView) })
		} else {
			toggle = widget.NewButton("Dar acesso total", func() { a.hub.SetPermission(s.ID, hub.PermFull) })
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
	code, err := identity.NormalizeCode(strings.TrimSpace(a.connectEnt.Text))
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.mu.Lock()
	if s, ok := a.sessions[code]; ok {
		a.mu.Unlock()
		a.mainTabs.Select(s.tab)
		return
	}
	a.mu.Unlock()

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
		var hello *hub.Hello
		for i := 0; i < 3; i++ {
			hello, err = rem.Hello()
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			a.failConnect(fmt.Errorf("não foi possível alcançar o Cliente (ele pode estar offline)"))
			return
		}

		id, err := rem.Connect(hub.ConnectReq{Name: a.name, Host: a.host, Code: a.id.Code})
		if err != nil {
			a.failConnect(err)
			return
		}

		s := &supportSession{
			app:        a,
			rem:        rem,
			id:         id,
			clientName: hello.Name,
			clientHost: hello.Host,
			clientCode: code,
			permission: hub.PermView,
			remoteW:    hello.Width,
			remoteH:    hello.Height,
			pollStop:   make(chan struct{}),
		}
		s.canvas = newRemoteCanvas(s)

		fyne.Do(func() {
			a.setBusy(false)
			a.setStatus("Pedindo autorização ao Cliente…", "warn")
			s.tab = container.NewTabItem("Cliente: "+hello.Name, s.build())
			a.mainTabs.Append(s.tab)
			a.mainTabs.Select(s.tab)
		})
		a.mu.Lock()
		a.sessions[code] = s
		a.mu.Unlock()
		a.runPoll(s)
	}()
}

func (a *App) failConnect(err error) {
	fyne.Do(func() {
		a.setBusy(false)
		a.setStatus(err.Error(), "err")
	})
}

func (a *App) runPoll(s *supportSession) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.pollStop:
			return
		case <-ticker.C:
		}
		cur, err := s.rem.Poll(s.id)
		if err != nil {
			if !s.closed {
				s.setStatus(err.Error(), "err")
				s.disconnect()
			}
			return
		}
		switch cur.Status {
		case hub.StatusRejected:
			s.setStatus("conexão recusada pelo Cliente", "err")
			s.disconnect()
			return
		case hub.StatusAccepted:
			if cur.Permission != s.permission {
				s.permission = cur.Permission
				fyne.Do(func() { s.applyPermission() })
			}
			if !s.streaming {
				s.streaming = true
				fyne.Do(func() { s.applyPermission() })
				a.startStream(s)
			}
		}
	}
}

func (a *App) startStream(s *supportSession) {
	body, err := s.rem.Stream(s.id)
	if err != nil {
		s.setStatus(err.Error(), "err")
		s.disconnect()
		return
	}
	go s.canvas.readFrames(body)
}

// ---- métodos da sessão ----

func (s *supportSession) canControl() bool {
	return s.permission == hub.PermFull && !s.closed
}

func (s *supportSession) build() fyne.CanvasObject {
	s.clientLb = widget.NewLabel("Cliente: " + s.clientName + " (" + s.clientHost + ")")
	s.statusTxt = canvas.NewText("pedindo autorização…", colWarn)
	s.statusTxt.TextSize = 13

	s.captureBtn = widget.NewButton("Capturar mouse", s.toggleCapture)
	s.captureBtn.Disable()
	filesBtn := widget.NewButton("Arquivos", s.openFiles)
	s.fsBtn = widget.NewButton("Tela cheia", s.toggleFullscreen)
	discBtn := widget.NewButton("Desconectar", s.disconnect)

	left := container.NewHBox(s.clientLb, s.statusTxt)
	right := container.NewHBox(s.captureBtn, filesBtn, s.fsBtn, discBtn)
	bar := container.NewBorder(nil, nil, left, right)

	return container.NewBorder(container.NewPadded(bar), nil, nil, nil, s.canvas)
}

func (s *supportSession) setStatus(text, kind string) {
	if s.statusTxt == nil {
		return
	}
	fyne.Do(func() {
		s.statusTxt.Text = text
		switch kind {
		case "ok":
			s.statusTxt.Color = colOk
		case "warn":
			s.statusTxt.Color = colWarn
		case "err":
			s.statusTxt.Color = colErr
		default:
			s.statusTxt.Color = colMuted
		}
		s.statusTxt.Refresh()
	})
}

func (s *supportSession) applyPermission() {
	if s.statusTxt == nil {
		return
	}
	if s.canControl() {
		s.statusTxt.Text = "controle total"
		s.statusTxt.Color = colOk
		s.captureBtn.Enable()
	} else {
		s.statusTxt.Text = "somente leitura"
		s.statusTxt.Color = colPrimary
		s.captured = false
		s.captureBtn.SetText("Capturar mouse")
		s.captureBtn.Disable()
	}
	s.statusTxt.Refresh()
	s.canvas.Refresh()
}

func (s *supportSession) toggleCapture() {
	if !s.canControl() {
		return
	}
	s.captured = !s.captured
	if s.captured {
		s.captureBtn.SetText("Soltar mouse")
	} else {
		s.captureBtn.SetText("Capturar mouse")
	}
	s.canvas.Refresh()
}

func (s *supportSession) toggleFullscreen() {
	s.app.fullScreen = !s.app.fullScreen
	s.app.win.SetFullScreen(s.app.fullScreen)
	if s.app.fullScreen {
		s.fsBtn.SetText("Sair da tela cheia")
	} else {
		s.fsBtn.SetText("Tela cheia")
	}
}

func (s *supportSession) openFiles() {
	if s.rem == nil {
		return
	}
	if s.filesWin == nil {
		w := s.app.fyneApp.NewWindow("Arquivos — " + s.clientName)
		s.files = newFilesTab(s)
		w.SetContent(s.files.object())
		w.Resize(fyne.NewSize(780, 560))
		s.filesWin = w
	}
	s.files.refresh()
	s.filesWin.Show()
}

func (s *supportSession) disconnect() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.pollStop)
	if s.rem != nil && s.id != "" {
		s.rem.Bye(s.id)
	}
	fyne.Do(func() {
		if s.app.mainTabs != nil && s.tab != nil {
			s.app.mainTabs.Remove(s.tab)
		}
		if s.filesWin != nil {
			s.filesWin.Close()
		}
	})
	s.app.mu.Lock()
	delete(s.app.sessions, s.clientCode)
	s.app.mu.Unlock()
}
