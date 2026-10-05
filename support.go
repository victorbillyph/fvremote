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
)

// SessionView é o estado de uma sessão de suporte enviado ao frontend.
type SessionView struct {
	Code       string `json:"code"`
	ClientName string `json:"clientName"`
	ClientHost string `json:"clientHost"`
	State      string `json:"state"`
	Permission string `json:"permission"`
	Message    string `json:"message"`
	RemoteW    int    `json:"remoteW"`
	RemoteH    int    `json:"remoteH"`
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
	streaming  bool
	closed     bool

	pollStop chan struct{}
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
			s.mu.Unlock()
			if start {
				go s.runStream()
			}
		}
	}
}

func (s *supportSession) runStream() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	id := s.id
	s.mu.Unlock()

	body, err := s.rem.Stream(id)
	if err != nil {
		s.fail(err)
		return
	}
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

func (s *supportSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	rem := s.rem
	id := s.id
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
