package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionFlow(t *testing.T) {
	h := New("Suporte", "pc", "CODE", "test")
	h.OnRequest = func(*Session) {}

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// hello
	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	var hello Hello
	_ = json.NewDecoder(resp.Body).Decode(&hello)
	resp.Body.Close()
	if hello.Name != "Suporte" {
		t.Fatalf("hello.name = %q", hello.Name)
	}

	// connect
	body, _ := json.Marshal(ConnectReq{Name: "João", Host: "host-j"})
	resp, err = http.Post(srv.URL+"/connect", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if out.ID == "" {
		t.Fatal("id vazio")
	}

	// input antes de aceitar -> 403
	r, _ := http.Post(srv.URL+"/input?id="+out.ID, "application/json", strings.NewReader(`{"type":"move","x":1,"y":1}`))
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("input antes de aceitar = %d, quer 403", r.StatusCode)
	}
	r.Body.Close()

	h.Accept(out.ID)

	// input em modo view -> 403
	r, _ = http.Post(srv.URL+"/input?id="+out.ID, "application/json", strings.NewReader(`{"type":"move","x":1,"y":1}`))
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("input em view = %d, quer 403", r.StatusCode)
	}
	r.Body.Close()

	h.SetPermission(out.ID, PermFull)
	if len(h.Active()) != 1 {
		t.Fatalf("esperava 1 sessão ativa")
	}

	h.Disconnect(out.ID)
	if len(h.Active()) != 0 {
		t.Fatalf("esperava 0 sessões ativas")
	}
}

func TestChatAndShell(t *testing.T) {
	h := New("S", "h", "C", "t")
	h.OnRequest = func(*Session) {}
	h.OnMessage = func(string, ChatMsg) {}
	h.OnShell = func(string) {}

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	// conecta e aceita
	resp, _ := http.Post(srv.URL+"/connect", "application/json", strings.NewReader(`{"name":"João","host":"pc"}`))
	var out struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	h.Accept(out.ID)

	// chat: suporte envia, cliente lê
	resp, _ = http.Post(srv.URL+"/chat?id="+out.ID, "application/json", strings.NewReader(`{"text":"olá"}`))
	resp.Body.Close()
	resp, _ = http.Get(srv.URL + "/chat?id=" + out.ID + "&since=0")
	var msgs []ChatMsg
	_ = json.NewDecoder(resp.Body).Decode(&msgs)
	resp.Body.Close()
	if len(msgs) != 1 || msgs[0].Text != "olá" || msgs[0].From != "support" {
		t.Fatalf("chat inesperado: %+v", msgs)
	}

	// terminal: antes de permitir -> 403
	resp, _ = http.Post(srv.URL+"/shell/exec?id="+out.ID, "application/json", strings.NewReader(`{"cmd":"echo oi"}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("exec sem permissão = %d, quer 403", resp.StatusCode)
	}
	resp.Body.Close()

	// pedido e permissão
	resp, _ = http.Post(srv.URL+"/shell/request?id="+out.ID, "application/json", nil)
	resp.Body.Close()
	h.AllowShell(out.ID)

	resp, _ = http.Post(srv.URL+"/shell/exec?id="+out.ID, "application/json", strings.NewReader(`{"cmd":"echo fvremote-teste"}`))
	var res struct {
		Output string `json:"output"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if !strings.Contains(res.Output, "fvremote-teste") {
		t.Fatalf("saída do terminal: %q", res.Output)
	}
}
