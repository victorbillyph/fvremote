package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/proxy"

	"github.com/victorbillyph/fvremote/internal/hub"
	"github.com/victorbillyph/fvremote/internal/shell"
)

// Remote é o cliente (lado Suporte) que fala com o onion do Cliente.
type Remote struct {
	HTTP  *http.Client
	Base  string
	Onion string
}

// New cria um cliente HTTP que disca via SOCKS5 (Tor local) para o .onion.
func New(socksAddr, onion string) (*Remote, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		Dial:                  dialer.Dial,
		MaxIdleConns:          10,
		ResponseHeaderTimeout: 120 * time.Second,
	}
	return &Remote{
		HTTP:  &http.Client{Transport: tr},
		Base:  "http://" + onion,
		Onion: onion,
	}, nil
}

func (r *Remote) Hello() (*hub.Hello, error) {
	resp, err := r.HTTP.Get(r.Base + "/hello")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var h hub.Hello
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *Remote) Connect(req hub.ConnectReq) (string, error) {
	b, _ := json.Marshal(req)
	resp, err := r.HTTP.Post(r.Base+"/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (r *Remote) Poll(id string) (*hub.Session, error) {
	resp, err := r.HTTP.Get(r.Base + "/poll?id=" + url.QueryEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sessão encerrada (%s)", resp.Status)
	}
	var s hub.Session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Remote) Bye(id string) {
	_, _ = r.HTTP.Get(r.Base + "/bye?id=" + url.QueryEscape(id))
}

// Stream abre o fluxo de frames (JPEG com prefixo de tamanho de 4 bytes)
// do monitor informado.
func (r *Remote) Stream(id string, display int) (io.ReadCloser, error) {
	u := fmt.Sprintf("%s/stream?id=%s&display=%d", r.Base, url.QueryEscape(id), display)
	resp, err := r.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("stream negado (%s)", resp.Status)
	}
	return resp.Body, nil
}

// ChatSend envia uma mensagem de chat ao Cliente.
func (r *Remote) ChatSend(id, text string) error {
	b, _ := json.Marshal(map[string]string{"text": text})
	resp, err := r.HTTP.Post(r.Base+"/chat?id="+url.QueryEscape(id), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ChatFetch busca mensagens a partir de um índice.
func (r *Remote) ChatFetch(id string, since int) ([]hub.ChatMsg, error) {
	u := fmt.Sprintf("%s/chat?id=%s&since=%d", r.Base, url.QueryEscape(id), since)
	resp, err := r.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var msgs []hub.ChatMsg
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// ShellRequest pede ao Cliente permissão para abrir o terminal.
func (r *Remote) ShellRequest(id string) error {
	resp, err := r.HTTP.Post(r.Base+"/shell/request?id="+url.QueryEscape(id), "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ShellStatus retorna o estado do terminal na sessão.
func (r *Remote) ShellStatus(id string) (string, error) {
	resp, err := r.HTTP.Get(r.Base + "/shell/status?id=" + url.QueryEscape(id))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.State, nil
}

// ShellExec executa um comando no Cliente (se permitido).
func (r *Remote) ShellExec(id, cmd string) (shell.Result, error) {
	b, _ := json.Marshal(map[string]string{"cmd": cmd})
	resp, err := r.HTTP.Post(r.Base+"/shell/exec?id="+url.QueryEscape(id), "application/json", bytes.NewReader(b))
	if err != nil {
		return shell.Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return shell.Result{}, fmt.Errorf("terminal não autorizado (%s)", resp.Status)
	}
	var res shell.Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return shell.Result{}, err
	}
	return res, nil
}

func (r *Remote) send(id string, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	resp, err := r.HTTP.Post(r.Base+"/input?id="+url.QueryEscape(id), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (r *Remote) Move(id string, x, y int) error {
	return r.send(id, map[string]any{"type": "move", "x": x, "y": y})
}

func (r *Remote) Click(id string, left, down bool, x, y int) error {
	btn := "left"
	if !left {
		btn = "right"
	}
	return r.send(id, map[string]any{"type": "click", "btn": btn, "down": down, "x": x, "y": y})
}

func (r *Remote) Scroll(id string, delta int) error {
	return r.send(id, map[string]any{"type": "scroll", "val": delta})
}

func (r *Remote) Key(id, name string, down bool) error {
	return r.send(id, map[string]any{"type": "key", "key": name, "down": down})
}

func (r *Remote) List(id, path string) ([]hub.FileItem, error) {
	u := r.Base + "/files?id=" + url.QueryEscape(id)
	if path != "" {
		u += "&path=" + url.QueryEscape(path)
	}
	resp, err := r.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("acesso a arquivos negado (%s)", resp.Status)
	}
	var items []hub.FileItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Remote) Upload(id, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return err
	}
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, r.Base+"/upload?id="+url.QueryEscape(id), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload negado (%s)", resp.Status)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (r *Remote) Download(id, path, dst string) error {
	u := r.Base + "/download?id=" + url.QueryEscape(id) + "&path=" + url.QueryEscape(path)
	resp, err := r.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download negado (%s)", resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
