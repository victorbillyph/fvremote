package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// frameHolder guarda o último frame JPEG de uma sessão.
type frameHolder struct {
	mu   sync.Mutex
	data []byte
	sig  chan struct{}
}

func newFrameHolder() *frameHolder {
	return &frameHolder{sig: make(chan struct{}, 1)}
}

func (h *frameHolder) set(b []byte) {
	h.mu.Lock()
	h.data = b
	h.mu.Unlock()
	select {
	case h.sig <- struct{}{}:
	default:
	}
}

func (h *frameHolder) get() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.data
}

// viewServer serve o vídeo remoto (MJPEG) para a webview, sem base64.
type viewServer struct {
	addr string
	ln   net.Listener
	srv  *http.Server

	mu      sync.Mutex
	holders map[string]*frameHolder
}

func newViewServer() *viewServer {
	v := &viewServer{holders: map[string]*frameHolder{}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return v
	}
	v.ln = ln
	v.addr = ln.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("/view", v.handleView)
	mux.HandleFunc("/frame", v.handleFrame)
	v.srv = &http.Server{Handler: mux}
	go func() { _ = v.srv.Serve(ln) }()
	return v
}

func (v *viewServer) holder(code string) *frameHolder {
	v.mu.Lock()
	defer v.mu.Unlock()
	h := v.holders[code]
	if h == nil {
		h = newFrameHolder()
		v.holders[code] = h
	}
	return h
}

func (v *viewServer) urlFor(code string) string {
	if v.addr == "" {
		return ""
	}
	return "http://" + v.addr + "/view?code=" + url.QueryEscape(code)
}

func (v *viewServer) close() {
	if v.srv != nil {
		_ = v.srv.Close()
	}
}

func (v *viewServer) handleFrame(w http.ResponseWriter, r *http.Request) {
	data := v.holder(r.URL.Query().Get("code")).get()
	if len(data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (v *viewServer) handleView(w http.ResponseWriter, r *http.Request) {
	h := v.holder(r.URL.Query().Get("code"))
	flusher, ok := w.(http.Flusher)
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.sig:
		case <-time.After(3 * time.Second):
		}
		data := h.get()
		if len(data) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(data)); err != nil {
			return
		}
		if _, err := w.Write(data); err != nil {
			return
		}
		if _, err := fmt.Fprint(w, "\r\n"); err != nil {
			return
		}
		if ok {
			flusher.Flush()
		}
	}
}
