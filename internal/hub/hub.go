package hub

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/victorbillyph/fvremote/internal/config"
	"github.com/victorbillyph/fvremote/internal/input"
	"github.com/victorbillyph/fvremote/internal/screen"
)

const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"

	PermView = "view"
	PermFull = "full"
)

// Session representa uma tentativa de conexão de um Suporte.
type Session struct {
	ID          string    `json:"id"`
	SupportName string    `json:"support_name"`
	SupportHost string    `json:"support_host"`
	Status      string    `json:"status"`
	Permission  string    `json:"permission"`
	Created     time.Time `json:"created"`
}

type Hello struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Code    string `json:"code"`
	Version string `json:"version"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type ConnectReq struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Code string `json:"code"`
}

// Hub é o servidor HTTP que roda atrás do onion service do Cliente.
type Hub struct {
	Name    string
	Host    string
	Code    string
	Version string

	mu       sync.Mutex
	sessions map[string]*Session

	// OnRequest é chamado quando chega uma nova solicitação de conexão.
	OnRequest func(*Session)
	// OnChange é chamado quando o estado de uma sessão muda.
	OnChange func()
}

func New(name, host, code, version string) *Hub {
	return &Hub{
		Name:     name,
		Host:     host,
		Code:     code,
		Version:  version,
		sessions: map[string]*Session{},
	}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Handler monta as rotas do servidor.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", h.hello)
	mux.HandleFunc("/connect", h.connect)
	mux.HandleFunc("/poll", h.poll)
	mux.HandleFunc("/bye", h.bye)
	mux.HandleFunc("/stream", h.stream)
	mux.HandleFunc("/input", h.input)
	mux.HandleFunc("/files", h.filesList)
	mux.HandleFunc("/download", h.download)
	mux.HandleFunc("/upload", h.upload)
	return mux
}

func (h *Hub) session(id string) *Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[id]
}

// requireAccepted retorna a sessão se ela existir e estiver aceita.
func (h *Hub) requireAccepted(w http.ResponseWriter, id string) *Session {
	s := h.session(id)
	if s == nil || s.Status != StatusAccepted {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}
	return s
}

// requireFull exige acesso total (controle de teclado/mouse e arquivos).
func (h *Hub) requireFull(w http.ResponseWriter, id string) *Session {
	s := h.requireAccepted(w, id)
	if s == nil {
		return nil
	}
	if s.Permission != PermFull {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}
	return s
}

func (h *Hub) hello(w http.ResponseWriter, r *http.Request) {
	wi, he := screen.Size()
	writeJSON(w, Hello{Name: h.Name, Host: h.Host, Code: h.Code, Version: h.Version, Width: wi, Height: he})
}

func (h *Hub) connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req ConnectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s := &Session{
		ID:          newID(),
		SupportName: req.Name,
		SupportHost: req.Host,
		Status:      StatusPending,
		Permission:  PermView,
		Created:     time.Now(),
	}
	h.mu.Lock()
	h.sessions[s.ID] = s
	h.mu.Unlock()
	if h.OnRequest != nil {
		h.OnRequest(s)
	}
	writeJSON(w, map[string]string{"id": s.ID})
}

func (h *Hub) poll(w http.ResponseWriter, r *http.Request) {
	s := h.session(r.URL.Query().Get("id"))
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, s)
}

func (h *Hub) bye(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.Disconnect(id)
	w.WriteHeader(http.StatusOK)
}

// ---- Ações disparadas pela UI ----

func (h *Hub) Pending() []*Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*Session
	for _, s := range h.sessions {
		if s.Status == StatusPending {
			out = append(out, s)
		}
	}
	return out
}

func (h *Hub) Active() []*Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*Session
	for _, s := range h.sessions {
		if s.Status == StatusAccepted {
			out = append(out, s)
		}
	}
	return out
}

func (h *Hub) Accept(id string) {
	h.mu.Lock()
	if s := h.sessions[id]; s != nil {
		s.Status = StatusAccepted
		s.Permission = PermView
	}
	h.mu.Unlock()
	h.changed()
}

func (h *Hub) Reject(id string) {
	h.mu.Lock()
	if s := h.sessions[id]; s != nil {
		s.Status = StatusRejected
	}
	h.mu.Unlock()
	h.changed()
}

func (h *Hub) SetPermission(id, perm string) {
	h.mu.Lock()
	if s := h.sessions[id]; s != nil {
		s.Permission = perm
	}
	h.mu.Unlock()
	h.changed()
}

func (h *Hub) Disconnect(id string) {
	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
	h.changed()
}

func (h *Hub) changed() {
	if h.OnChange != nil {
		h.OnChange()
	}
}

// ---- Streaming / controle ----

func (h *Hub) stream(w http.ResponseWriter, r *http.Request) {
	if h.requireAccepted(w, r.URL.Query().Get("id")) == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream não suportado", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	for {
		if r.Context().Err() != nil {
			return
		}
		var buf bytes.Buffer
		if err := screen.WriteJPEG(&buf, 70); err != nil {
			return
		}
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(buf.Len()))
		if _, err := w.Write(hdr[:]); err != nil {
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			return
		}
		flusher.Flush()
		time.Sleep(40 * time.Millisecond)
	}
}

type event struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Btn  string `json:"btn"`
	Down bool   `json:"down"`
	Key  string `json:"key"`
	Val  int    `json:"val"`
}

func (h *Hub) input(w http.ResponseWriter, r *http.Request) {
	if h.requireFull(w, r.URL.Query().Get("id")) == nil {
		return
	}
	var e event
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch e.Type {
	case "move":
		input.Move(e.X, e.Y)
	case "click":
		input.Click(e.Btn != "right", e.Down)
	case "scroll":
		input.Scroll(e.Val)
	case "key":
		input.Key(e.Key, e.Down)
	}
	w.WriteHeader(http.StatusOK)
}

// ---- Arquivos ----

type FileItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

func (h *Hub) filesList(w http.ResponseWriter, r *http.Request) {
	if h.requireFull(w, r.URL.Query().Get("id")) == nil {
		return
	}
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	out := make([]FileItem, 0, len(ents))
	for _, e := range ents {
		it := FileItem{Name: e.Name(), Path: filepath.Join(dir, e.Name()), Dir: e.IsDir()}
		if info, err := e.Info(); err == nil {
			it.Size = info.Size()
		}
		out = append(out, it)
	}
	writeJSON(w, out)
}

func (h *Hub) download(w http.ResponseWriter, r *http.Request) {
	if h.requireFull(w, r.URL.Query().Get("id")) == nil {
		return
	}
	p := r.URL.Query().Get("path")
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(p))
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	f, err := os.Open(p)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

func (h *Hub) upload(w http.ResponseWriter, r *http.Request) {
	if h.requireFull(w, r.URL.Query().Get("id")) == nil {
		return
	}
	_ = r.ParseMultipartForm(256 << 20)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer f.Close()
	dst := filepath.Join(config.ReceivedDir(), filepath.Base(hdr.Filename))
	out, err := os.Create(dst)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(dst))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
