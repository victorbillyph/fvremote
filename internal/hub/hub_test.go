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
