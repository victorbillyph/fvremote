package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/proxy"
)

// Client conecta ao servidor fvremote através do SOCKS5 do Tor.
type Client struct {
	HTTP *http.Client
	Base string
}

// New cria um cliente HTTP que disca via SOCKS5 (Tor local) para o .onion.
func New(socksAddr, onion string) (*Client, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		Dial:                  dialer.Dial,
		DisableKeepAlives:     false,
		MaxIdleConns:          10,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	return &Client{
		HTTP: &http.Client{Transport: tr},
		Base: fmt.Sprintf("http://%s:8080", onion),
	}, nil
}

type Info struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Onion  string `json:"onion"`
}

func (c *Client) Info() (*Info, error) {
	resp, err := c.HTTP.Get(c.Base + "/info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) send(payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Post(c.Base+"/input", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) Move(x, y int) error {
	return c.send(map[string]any{"type": "move", "x": x, "y": y})
}

func (c *Client) Click(left, down bool) error {
	btn := "left"
	if !left {
		btn = "right"
	}
	return c.send(map[string]any{"type": "click", "btn": btn, "down": down})
}

func (c *Client) Scroll(delta int) error {
	return c.send(map[string]any{"type": "scroll", "val": delta})
}

func (c *Client) Key(name string, down bool) error {
	return c.send(map[string]any{"type": "key", "key": name, "down": down})
}

type FileItem struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

func (c *Client) List() ([]FileItem, error) {
	resp, err := c.HTTP.Get(c.Base + "/files")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var items []FileItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) Upload(path string) error {
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

	req, err := http.NewRequest(http.MethodPost, c.Base+"/upload", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) Download(name, dst string) error {
	resp, err := c.HTTP.Get(c.Base + "/download?file=" + name)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download falhou: %s", resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
