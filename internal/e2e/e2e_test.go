//go:build e2e

package e2e

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/victorbillyph/fvremote/internal/hub"
	"github.com/victorbillyph/fvremote/internal/identity"
	"github.com/victorbillyph/fvremote/internal/remote"
	"github.com/victorbillyph/fvremote/internal/torx"
)

func TestEndToEnd(t *testing.T) {
	id, err := identity.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("código: %s", id.Code)
	t.Logf("onion:  %s", id.Onion)

	r, err := torx.Start(func(stage string, pct int) { t.Logf("[%s] %d%%", stage, pct) })
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	h := hub.New("Suporte-Test", "host-test", id.Code, "e2e")
	go func() { _ = http.Serve(ln, h.Handler()) }()

	sid, err := r.AddOnion(id.TorBlob, 80, port)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSuffix(id.Onion, ".onion"); sid != want {
		t.Fatalf("ServiceID %s != %s", sid, want)
	}
	t.Log("onion publicado, conectando via SOCKS…")

	rem, err := remote.New(r.SocksAddr(), id.Onion)
	if err != nil {
		t.Fatal(err)
	}

	var hello *hub.Hello
	for i := 0; i < 40; i++ {
		hello, err = rem.Hello()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		t.Fatalf("não alcançou o onion: %v", err)
	}
	t.Logf("hello: %s@%s (%dx%d)", hello.Name, hello.Host, hello.Width, hello.Height)

	sessionID, err := rem.Connect(hub.ConnectReq{Name: "Sup", Host: "h"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := rem.Poll(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != hub.StatusPending {
		t.Fatalf("status inicial = %s", s.Status)
	}

	h.Accept(sessionID)
	time.Sleep(500 * time.Millisecond)
	s, err = rem.Poll(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != hub.StatusAccepted || s.Permission != hub.PermView {
		t.Fatalf("status=%s perm=%s", s.Status, s.Permission)
	}

	h.SetPermission(sessionID, hub.PermFull)
	time.Sleep(300 * time.Millisecond)
	s, _ = rem.Poll(sessionID)
	if s.Permission != hub.PermFull {
		t.Fatalf("permissão não propagou: %s", s.Permission)
	}
	t.Log("E2E OK: onion determinístico + SOCKS + sessão + permissões")
}
