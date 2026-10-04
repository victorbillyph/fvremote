package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/victorbillyph/fvremote/internal/files"
	"github.com/victorbillyph/fvremote/internal/input"
	"github.com/victorbillyph/fvremote/internal/screen"
)

type Srv struct {
	Onion string
}

func New(onion string) *Srv { return &Srv{Onion: onion} }

// Run sobe o servidor HTTP apenas em localhost (o acesso externo passa pelo Tor).
func (s *Srv) Run(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/info", s.info)
	mux.HandleFunc("/stream", s.stream)
	mux.HandleFunc("/input", s.input)
	mux.HandleFunc("/files", s.filesList)
	mux.HandleFunc("/upload", s.upload)
	mux.HandleFunc("/download", s.download)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Srv) info(w http.ResponseWriter, r *http.Request) {
	wi, h := screen.Size()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"width":  wi,
		"height": h,
		"onion":  s.Onion,
	})
}

// stream envia frames JPEG com prefixo de tamanho (4 bytes big-endian).
func (s *Srv) stream(w http.ResponseWriter, r *http.Request) {
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

func (s *Srv) input(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
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

type fileItem struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

func (s *Srv) filesList(w http.ResponseWriter, r *http.Request) {
	ents, err := files.List()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	out := make([]fileItem, 0, len(ents))
	for _, e := range ents {
		it := fileItem{Name: e.Name(), Dir: e.IsDir()}
		if info, err := e.Info(); err == nil {
			it.Size = info.Size()
		}
		out = append(out, it)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Srv) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseMultipartForm(64 << 20)
	f, h, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer f.Close()
	dst, err := files.Save(f, h.Filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(dst))
}

func (s *Srv) download(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("file")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	p, _ := files.Path(name)
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(name))
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	f, err := os.Open(p)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}
