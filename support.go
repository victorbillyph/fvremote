package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/victorbillyph/fvremote/internal/hub"
	"github.com/victorbillyph/fvremote/internal/remote"
	"github.com/victorbillyph/fvremote/internal/screen"
	"github.com/victorbillyph/fvremote/internal/shell"
)

// SessionView é o estado de uma sessão de suporte enviado ao frontend.
type SessionView struct {
	Code       string           `json:"code"`
	ClientName string           `json:"clientName"`
	ClientHost string           `json:"clientHost"`
	State      string           `json:"state"`
	Permission string           `json:"permission"`
	Message    string           `json:"message"`
	RemoteW    int              `json:"remoteW"`
	RemoteH    int              `json:"remoteH"`
	Displays   []screen.Display `json:"displays"`
	Display    int              `json:"display"`
	Shell      string           `json:"shell"`
}

type supportSession struct {
	app   *App
	code  string
	onion string

	rem    *remote.Remote
	id     string
	holder *frameHolder

	mu         sync.Mutex
	clientName string
	clientHost string
	state      string
	permission string
	message    string
	remoteW    int
	remoteH    int
	displays   []screen.Display
	display    int
	shell      string
	streaming  bool
	closed     bool
	body       io.ReadCloser

	chatIndex int
	pollStop  chan struct{}
}

func (s *supportSession) view() SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionView{
		Code:       s.code,
		ClientName: s.clientName,
		ClientHost: s.clientHost,
		State:      s.state,
		Permission: s.permission,
		Message:    s.message,
		RemoteW:    s.remoteW,
		RemoteH:    s.remoteH,
		Displays:   s.displays,
		Display:    s.display,
		Shell:      s.shell,
	}
}

func (s *supportSession) setState(state, msg, perm string) {
	s.mu.Lock()
	if state != "" {
		s.state = state
	}
	if msg != "" {
		s.message = msg
	}
	if perm != "" {
		s.permission = perm
	}
	s.mu.Unlock()
	s.app.emit("support:changed", s.view())
}

func (s *supportSession) canControl() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.permission == hub.PermFull && !s.closed && s.rem != nil && s.id != ""
}

func (s *supportSession) fail(err error) {
	s.setState("error", err.Error(), "")
}

func (s *supportSession) run() {
	rem, err := remote.New(s.app.tor.SocksAddr(), s.onion)
	if err != nil {
		s.fail(err)
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
		s.fail(fmt.Errorf("não foi possível alcançar o Cliente (ele pode estar offline)"))
		return
	}

	s.mu.Lock()
	s.rem = rem
	s.clientName = hello.Name
	s.clientHost = hello.Host
	s.remoteW = hello.Width
	s.remoteH = hello.Height
	s.displays = hello.Displays
	s.display = 0
	s.mu.Unlock()
	s.setState("connecting", "Conectando…", "view")

	id, err := rem.Connect(hub.ConnectReq{Name: s.app.name, Host: s.app.host, Code: s.app.id.Code})
	if err != nil {
		s.fail(err)
		return
	}
	s.mu.Lock()
	s.id = id
	s.mu.Unlock()
	s.setState("waiting", "Pedindo autorização ao Cliente…", "view")
	s.poll()
}

func (s *supportSession) poll() {
	s.mu.Lock()
	id := s.id
	s.mu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.pollStop:
			return
		case <-ticker.C:
		}
		cur, err := s.rem.Poll(id)
		if err != nil {
			s.fail(err)
			return
		}
		switch cur.Status {
		case hub.StatusRejected:
			s.setState("rejected", "Conexão recusada pelo Cliente.", "")
			return
		case hub.StatusAccepted:
			if cur.Permission != s.view().Permission {
				if cur.Permission == hub.PermFull {
					s.setState("accepted", "Controle total liberado pelo Cliente.", hub.PermFull)
				} else {
					s.setState("accepted", "Conectado — somente leitura.", hub.PermView)
				}
			} else if s.view().State != "accepted" {
				s.setState("accepted", "Conectado — somente leitura.", hub.PermView)
			}
			s.mu.Lock()
			start := !s.streaming
			s.streaming = true
			name := s.clientName
			host := s.clientHost
			s.mu.Unlock()
			if start {
				s.app.history.record(s.code, name, host)
				go s.runStream()
			}
			s.fetchChat()
		}
	}
}

func (s *supportSession) fetchChat() {
	msgs, err := s.rem.ChatFetch(s.id, s.chatIndex)
	if err != nil || len(msgs) == 0 {
		return
	}
	s.chatIndex += len(msgs)
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]any{"from": m.From, "text": m.Text, "at": m.At})
	}
	s.app.emit("support:chat", map[string]any{"code": s.code, "messages": out})
}

func (s *supportSession) setDisplay(index int) {
	s.mu.Lock()
	s.display = index
	s.remoteW, s.remoteH = screen.SizeOf(index)
	if s.body != nil {
		_ = s.body.Close()
		s.body = nil
	}
	s.mu.Unlock()
	s.app.emit("support:changed", s.view())
	s.mu.Lock()
	running := s.streaming && !s.closed
	s.mu.Unlock()
	if running {
		go s.runStream()
	}
}

func (s *supportSession) runStream() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	id := s.id
	display := s.display
	s.mu.Unlock()

	body, err := s.rem.Stream(id, display)
	if err != nil {
		s.fail(err)
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		body.Close()
		return
	}
	s.body = body
	s.mu.Unlock()
	defer body.Close()

	br := bufio.NewReader(body)
	var hdr [4]byte
	for {
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n == 0 || n > 32<<20 {
			return
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(br, data); err != nil {
			return
		}
		s.holder.set(data)
	}
}

// -- Ações expostas à UI (Suporte) --

func (s *supportSession) sendChat(text string) {
	if s.rem == nil || s.id == "" {
		return
	}
	_ = s.rem.ChatSend(s.id, text)
	s.fetchChat()
}

func (s *supportSession) requestShell() {
	if s.rem == nil || s.id == "" {
		return
	}
	_ = s.rem.ShellRequest(s.id)
	s.mu.Lock()
	s.shell = "pending"
	s.mu.Unlock()
	s.app.emit("support:changed", s.view())
}

func (s *supportSession) shellStatus() string {
	if s.rem == nil || s.id == "" {
		return "none"
	}
	st, err := s.rem.ShellStatus(s.id)
	if err != nil {
		return s.view().Shell
	}
	s.mu.Lock()
	s.shell = st
	s.mu.Unlock()
	s.app.emit("support:changed", s.view())
	return st
}

func (s *supportSession) shellExec(cmd string) (shell.Result, error) {
	if s.rem == nil || s.id == "" {
		return shell.Result{}, fmt.Errorf("sem sessão")
	}
	return s.rem.ShellExec(s.id, cmd)
}

func (s *supportSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	rem := s.rem
	id := s.id
	if s.body != nil {
		_ = s.body.Close()
	}
	s.mu.Unlock()

	close(s.pollStop)
	if rem != nil && id != "" {
		rem.Bye(id)
	}
	s.app.mu.Lock()
	delete(s.app.sessions, s.code)
	s.app.mu.Unlock()
	s.app.emit("support:removed", s.code)
}
