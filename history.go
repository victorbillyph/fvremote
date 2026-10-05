package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/victorbillyph/fvremote/internal/config"
)

// HistoryItem é um Cliente com quem já houve conexão.
type HistoryItem struct {
	Code string    `json:"code"`
	Name string    `json:"name"`
	Host string    `json:"host"`
	Last time.Time `json:"last"`
}

type historyStore struct {
	mu    sync.Mutex
	items []HistoryItem
}

func historyPath() string { return filepath.Join(config.BaseDir(), "history.json") }

func loadHistory() *historyStore {
	h := &historyStore{}
	if b, err := os.ReadFile(historyPath()); err == nil {
		_ = json.Unmarshal(b, &h.items)
	}
	return h
}

func (h *historyStore) save() {
	b, _ := json.MarshalIndent(h.items, "", "  ")
	_ = os.WriteFile(historyPath(), b, 0o600)
}

// record insere/atualiza um cliente no histórico.
func (h *historyStore) record(code, name, host string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.items {
		if h.items[i].Code == code {
			h.items[i].Name = name
			h.items[i].Host = host
			h.items[i].Last = time.Now()
			h.save()
			return
		}
	}
	h.items = append(h.items, HistoryItem{Code: code, Name: name, Host: host, Last: time.Now()})
	h.save()
}

func (h *historyStore) list() []HistoryItem {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HistoryItem, len(h.items))
	copy(out, h.items)
	sort.Slice(out, func(i, j int) bool { return out[i].Last.After(out[j].Last) })
	return out
}

func (h *historyStore) forget(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.items[:0]
	for _, it := range h.items {
		if it.Code != code {
			out = append(out, it)
		}
	}
	h.items = out
	h.save()
}
