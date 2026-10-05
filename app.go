package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/victorbillyph/fvremote/internal/config"
	"github.com/victorbillyph/fvremote/internal/hub"
	"github.com/victorbillyph/fvremote/internal/identity"
	"github.com/victorbillyph/fvremote/internal/shell"
	"github.com/victorbillyph/fvremote/internal/torx"
	"github.com/victorbillyph/fvremote/internal/update"
	"github.com/victorbillyph/fvremote/internal/tray"
)

const version = "0.7.2"

// App é o backend exposto ao frontend (webview).
type App struct {
	ctx context.Context

	name string
	host string

	id        *identity.Identity
	tor       *torx.Runner
	hub       *hub.Hub
	httpSrv   *http.Server
	view      *viewServer
	localPort int

	mu       sync.Mutex
	sessions map[string]*supportSession

	history *historyStore
	upd     *update.Info
}

// Info descreve este dispositivo.
type Info struct {
	Code    string `json:"code"`
	Onion   string `json:"onion"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Version string `json:"version"`
}

// FileItem é um item de arquivo/diretório remoto.
type FileItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

func NewApp() *App {
	n, h := profile()
	_ = config.EnsureBase()
	return &App{name: n, host: h, sessions: map[string]*supportSession{}, history: loadHistory()}
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

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.bootstrap()
	a.startTray()
}

func (a *App) startTray() {
	go func() {
		// Initialize tray with nil bus - it will create its own session bus on Linux
		t := tray.NewTrayIcon(nil)
		if err := t.Start(a.ctx); err != nil {
			log.Printf("tray start warning: %v", err)
			return
		}

		// Create the right-click menu
		menu := &tray.Menu{
			Items: []tray.MenuItem{
				{Label: "Abrir UI", OnClick: func() {
					a.ShowApp()
				}},
				{Label: "Sair", OnClick: func() {
					a.QuitApp()
				}},
			},
		}

		// Show the menu (the OS will handle displaying it on tray icon right-click)
		t.ShowMenu(menu)
	}()
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	for _, s := range a.sessions {
		s.close()
	}
	a.mu.Unlock()
	if a.tor != nil {
		_ = a.tor.Stop()
	}
	if a.view != nil {
		a.view.close()
	}
}

func (a *App) emit(event string, data ...any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data...)
	}
}

func (a *App) bootstrap() {
	a.emit("setup", stageMsg("identity", 0, "Preparando…"))

	id, err := identity.LoadOrCreate()
	if err != nil {
		a.fatal(err)
		return
	}
	a.id = id

	r, err := torx.Start(func(stage string, pct int) {
		switch stage {
		case "download":
			a.emit("setup", stageMsg("download", pct, fmt.Sprintf("Baixando Tor (exclusivo do fvremote)… %d%%", pct)))
		case "bootstrap":
			a.emit("setup", stageMsg("bootstrap", pct, fmt.Sprintf("Conectando à rede Tor… %d%%", pct)))
		case "ready":
			a.emit("setup", stageMsg("bootstrap", 100, "Teste de conexão Tor concluído."))
		}
	})
	if err != nil {
		a.fatal(err)
		return
	}
	a.tor = r

	a.emit("setup", stageMsg("onion", 0, "Publicando seu endereço .onion…"))

	a.view = newViewServer()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		a.fatal(err)
		return
	}
	a.localPort = ln.Addr().(*net.TCPAddr).Port
	a.hub = hub.New(a.name, a.host, a.id.Code, version)
	a.hub.OnRequest = func(s *hub.Session) { a.emit("incoming:request", incomingView(s)) }
	a.hub.OnChange = func() { a.emit("incoming:changed", a.incomingList()) }
	a.hub.OnMessage = func(id string, m hub.ChatMsg) {
		if m.From == "support" {
			a.emit("incoming:chat", map[string]any{"id": id, "from": m.From, "text": m.Text, "at": m.At})
		}
	}
	a.hub.OnShell = func(id string) {
		s := a.hub.Get(id)
		state := "none"
		if s != nil {
			state = s.Shell
		}
		a.emit("incoming:shell", map[string]any{"id": id, "state": state})
	}
	a.httpSrv = &http.Server{Handler: a.hub.Handler(), ReadHeaderTimeout: 15 * time.Second}
	go func() { _ = a.httpSrv.Serve(ln) }()

	sid, err := a.tor.AddOnion(a.id.TorBlob, 80, a.localPort)
	if err != nil {
		a.fatal(err)
		return
	}
	if want := strings.TrimSuffix(a.id.Onion, ".onion"); sid != want {
		a.fatal(fmt.Errorf("endereço onion divergente: tor=%s esperado=%s", sid, want))
		return
	}

	a.emit("ready", a.GetInfo())
}

func stageMsg(stage string, pct int, msg string) map[string]any {
	return map[string]any{"stage": stage, "pct": pct, "msg": msg}
}

func (a *App) fatal(err error) {
	log.Println("fvremote:", err)
	a.emit("fatal", err.Error())
}

// GetInfo retorna o código e o endereço do dispositivo.
func (a *App) GetInfo() Info {
	info := Info{Name: a.name, Host: a.host, Version: version}
	if a.id != nil {
		info.Code = a.id.Code
		info.Onion = a.id.Onion
	}
	return info
}

// ViewURL retorna a URL local do stream de vídeo da sessão.
func (a *App) ViewURL(code string) string {
	if a.view == nil {
		return ""
	}
	return a.view.urlFor(code)
}

// ToggleFullscreen alterna a tela cheia da janela.
func (a *App) ToggleFullscreen() {
	if a.ctx == nil {
		return
	}
	if runtime.WindowIsFullscreen(a.ctx) {
		runtime.WindowUnfullscreen(a.ctx)
		return
	}
	runtime.WindowFullscreen(a.ctx)
}

// ---- Sessões de suporte (eu assistindo outro Cliente) ----

func (a *App) Connect(codeIn string) error {
	code, err := identity.NormalizeCode(codeIn)
	if err != nil {
		return err
	}
	if a.tor == nil {
		return fmt.Errorf("a rede Tor ainda está iniciando")
	}
	a.mu.Lock()
	if _, ok := a.sessions[code]; ok {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	onion, err := identity.DeriveFromCode(code)
	if err != nil {
		return err
	}

	s := &supportSession{
		app:      a,
		code:     code,
		onion:    onion,
		state:    "finding",
		message:  "Achando o Cliente na rede Tor…",
		holder:   a.view.holder(code),
		pollStop: make(chan struct{}),
	}
	a.mu.Lock()
	a.sessions[code] = s
	a.mu.Unlock()
	a.emit("support:changed", s.view())

	go s.run()
	return nil
}

func (a *App) Disconnect(codeIn string) {
	code, err := identity.NormalizeCode(codeIn)
	if err != nil {
		return
	}
	a.mu.Lock()
	s := a.sessions[code]
	a.mu.Unlock()
	if s != nil {
		s.close()
	}
}

// Sessions lista as sessões de suporte ativas.
func (a *App) Sessions() []SessionView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]SessionView, 0, len(a.sessions))
	for _, s := range a.sessions {
		out = append(out, s.view())
	}
	return out
}

func (a *App) session(codeIn string) *supportSession {
	code, err := identity.NormalizeCode(codeIn)
	if err != nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[code]
}

// Move/Click/Scroll/Key encaminham entrada para o Cliente (somente com acesso total).
func (a *App) Move(code string, x, y int) {
	if s := a.session(code); s != nil && s.canControl() {
		_ = s.rem.Move(s.id, x, y)
	}
}

func (a *App) Click(code string, left, down bool, x, y int) {
	if s := a.session(code); s != nil && s.canControl() {
		_ = s.rem.Click(s.id, left, down, x, y)
	}
}

func (a *App) Scroll(code string, delta int) {
	if s := a.session(code); s != nil && s.canControl() {
		_ = s.rem.Scroll(s.id, delta)
	}
}

func (a *App) Key(code, name string, down bool) {
	if s := a.session(code); s != nil && s.canControl() {
		_ = s.rem.Key(s.id, name, down)
	}
}

// ---- Arquivos ----

func (a *App) ListFiles(code, path string) ([]FileItem, error) {
	s := a.session(code)
	if s == nil || s.rem == nil || s.permission != hub.PermFull {
		return nil, fmt.Errorf("é necessário acesso total")
	}
	if path == "" {
		path, _ = os.UserHomeDir()
	}
	items, err := s.rem.List(s.id, path)
	if err != nil {
		return nil, err
	}
	out := make([]FileItem, 0, len(items))
	for _, it := range items {
		out = append(out, FileItem{Name: it.Name, Path: it.Path, Dir: it.Dir, Size: it.Size})
	}
	return out, nil
}

func (a *App) DownloadFile(code, path string) error {
	s := a.session(code)
	if s == nil || s.rem == nil || s.permission != hub.PermFull {
		return fmt.Errorf("é necessário acesso total")
	}
	dst, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Salvar arquivo",
		DefaultFilename: filepath.Base(path),
	})
	if err != nil || dst == "" {
		return err
	}
	return s.rem.Download(s.id, path, dst)
}

func (a *App) UploadFile(code string) error {
	s := a.session(code)
	if s == nil || s.rem == nil || s.permission != hub.PermFull {
		return fmt.Errorf("é necessário acesso total")
	}
	src, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Enviar arquivo"})
	if err != nil || src == "" {
		return err
	}
	return s.rem.Upload(s.id, src)
}

// ---- Solicitações recebidas (outro Suporte quer me assistir) ----

type IncomingView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Status     string `json:"status"`
	Permission string `json:"permission"`
}

func incomingView(s *hub.Session) IncomingView {
	return IncomingView{ID: s.ID, Name: s.SupportName, Host: s.SupportHost, Status: s.Status, Permission: s.Permission}
}

func (a *App) incomingList() []IncomingView {
	if a.hub == nil {
		return nil
	}
	var out []IncomingView
	for _, s := range a.hub.Active() {
		out = append(out, incomingView(s))
	}
	return out
}

// Incoming lista os suportes conectados a mim.
func (a *App) Incoming() []IncomingView { return a.incomingList() }

func (a *App) AcceptIncoming(id string) {
	if a.hub != nil {
		a.hub.Accept(id)
	}
}

func (a *App) RejectIncoming(id string) {
	if a.hub != nil {
		a.hub.Reject(id)
	}
}

func (a *App) SetIncomingPermission(id, perm string) {
	if a.hub != nil {
		a.hub.SetPermission(id, perm)
	}
}

func (a *App) DisconnectIncoming(id string) {
	if a.hub != nil {
		a.hub.Disconnect(id)
	}
}

// ---- Monitores, chat e terminal (lado Suporte) ----

func (a *App) SetDisplay(code string, index int) {
	if s := a.session(code); s != nil {
		s.setDisplay(index)
	}
}

func (a *App) SendChat(code, text string) {
	if s := a.session(code); s != nil && text != "" {
		s.sendChat(text)
	}
}

// ChatHistory retorna todas as mensagens da sessão de suporte.
func (a *App) ChatHistory(code string) []hub.ChatMsg {
	s := a.session(code)
	if s == nil || s.rem == nil || s.id == "" {
		return nil
	}
	msgs, err := s.rem.ChatFetch(s.id, 0)
	if err != nil {
		return nil
	}
	return msgs
}

func (a *App) RequestShell(code string) {
	if s := a.session(code); s != nil {
		s.requestShell()
	}
}

func (a *App) ShellStatus(code string) string {
	if s := a.session(code); s != nil {
		return s.shellStatus()
	}
	return "none"
}

func (a *App) ShellExec(code, cmd string) (shell.Result, error) {
	if s := a.session(code); s != nil {
		return s.shellExec(cmd)
	}
	return shell.Result{}, fmt.Errorf("sem sessão")
}

// ---- Chat e terminal (lado Cliente) ----

func (a *App) IncomingChat(id string) []hub.ChatMsg {
	if a.hub == nil {
		return nil
	}
	return a.hub.Messages(id, 0)
}

func (a *App) SendIncomingChat(id, text string) {
	if a.hub != nil && text != "" {
		a.hub.AddClientMessage(id, text)
	}
}

func (a *App) AllowShell(id string) {
	if a.hub != nil {
		a.hub.AllowShell(id)
	}
}

func (a *App) DenyShell(id string) {
	if a.hub != nil {
		a.hub.DenyShell(id)
	}
}

// ---- Histórico de clientes ----

func (a *App) History() []HistoryItem { return a.history.list() }

func (a *App) ForgetClient(code string) { a.history.forget(code) }

// ---- Atualizações ----

func (a *App) sendNotification(id, title, body string, categoryID string, data map[string]interface{}) {
	opts := runtime.NotificationOptions{
		ID:         id,
		Title:      title,
		Body:       body,
		CategoryID: categoryID,
		Data:       data,
	}
	_ = runtime.SendNotification(a.ctx, opts)
}

func (a *App) NotifyConnection(code, status, msg string) {
	a.sendNotification(
		"connection",
		"Conexão " + status,
		msg,
		"",
		map[string]interface{}{"code": code, "status": status},
	)
}

func (a *App) NotifyPermission(code string, action string, granted bool, msg string) {
	a.sendNotification(
		"permission",
		"Permissão " + action,
		msg,
		"",
		map[string]interface{}{"code": code, "action": action, "granted": granted},
	)
}

func (a *App) NotifyCmd(code, title, output string) {
	a.sendNotification(
		"cmd",
		"Terminal: "+title,
		output,
		"",
		map[string]interface{}{"code": code, "title": title},
	)
}

func (a *App) NotifyUpdate(available bool, current, latest string) {
	var status string
	if available {
		status = "atualização disponível: " + latest
	} else {
		status = "vocão está na versão mais recente: " + current
	}
	a.sendNotification(
		"update",
		"Atualização",
		status,
		"",
		map[string]interface{}{"current": current, "latest": latest, "available": available},
	)
}

func (a *App) CheckUpdate() (*update.Info, error) {
	socks := ""
	if a.tor != nil {
		socks = a.tor.SocksAddr()
	}
	info, err := update.Check(version, socks)
	if err != nil {
		return nil, err
	}
	a.upd = info
	return info, nil
}

func (a *App) startAutoUpdateChecker() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if a.shouldAutoUpdate() {
				if info, err := a.CheckUpdate(); err == nil && info.Available {
					a.NotifyUpdate(true, info.Current, info.Latest)
					if err := a.DoUpdate(); err != nil {
						a.NotifyUpdate(false, info.Current, info.Latest)
						a.NotifyConnection("", "erro", err.Error())
					}
				}
			}
		}
	}()
}

func (a *App) shouldAutoUpdate() bool {
	a.mu.Lock()
	idle := len(a.sessions) == 0
	if a.hub != nil {
		idle = idle && len(a.hub.Active()) == 0
	}
	a.mu.Unlock()
	return idle
}

func (a *App) ShowApp() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
}

func (a *App) QuitApp() {
	a.shutdown(nil)
	os.Exit(0)
}

func (a *App) DoUpdate() error {
	if a.upd == nil || !a.upd.Available {
		return fmt.Errorf("nenhuma atualização disponível")
	}
	socks := ""
	if a.tor != nil {
		socks = a.tor.SocksAddr()
	}
	if err := update.Apply(a.upd, socks); err != nil {
		return err
	}
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = update.Restart()
	}()
	return nil
}
