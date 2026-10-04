package tor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/victorbillyph/fvremote/internal/config"
)

type Mode int

const (
	ModeServer Mode = iota // expõe hidden service
	ModeClient             // apenas SOCKS5 para discar .onion
)

// Runner gerencia um processo Tor standalone exclusivo do fvremote.
type Runner struct {
	Mode  Mode
	Port  string
	Onion string

	cmd          *exec.Cmd
	bootstrapped chan struct{}
	bootOnce     sync.Once
}

func NewServer() *Runner {
	return &Runner{Mode: ModeServer, Port: "9050", bootstrapped: make(chan struct{})}
}

func NewClient() *Runner {
	return &Runner{Mode: ModeClient, Port: "9050", bootstrapped: make(chan struct{})}
}

func (r *Runner) SocksAddr() string { return "127.0.0.1:" + r.Port }

// Start garante o binário, escreve o torrc e sobe o Tor.
func (r *Runner) Start() error {
	if err := Ensure(); err != nil {
		return err
	}
	_ = os.MkdirAll(config.TorDataDir(), 0o700)

	rc := filepath.Join(config.TorDir(), "fvremote-torrc")
	f, err := os.Create(rc)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "SocksPort 127.0.0.1:%s\n", r.Port)
	fmt.Fprintf(f, "DataDirectory %s\n", config.TorDataDir())
	fmt.Fprintf(f, "Log notice stdout\n")
	fmt.Fprintf(f, "AvoidDiskWrites 1\n")
	if r.Mode == ModeServer {
		_ = os.MkdirAll(config.HsDir(), 0o700)
		fmt.Fprintf(f, "HiddenServiceDir %s\n", config.HsDir())
		fmt.Fprintf(f, "HiddenServicePort 8080 127.0.0.1:8080\n")
	}
	_ = f.Close()

	r.cmd = exec.Command(config.TorBin(), "-f", rc)
	pr, pw := io.Pipe()
	r.cmd.Stdout = pw
	r.cmd.Stderr = pw
	if err := r.cmd.Start(); err != nil {
		return err
	}
	go r.scan(pr)

	if err := r.waitBootstrap(90 * time.Second); err != nil {
		_ = r.Stop()
		return err
	}

	if r.Mode == ModeServer {
		if err := r.readOnion(30 * time.Second); err != nil {
			_ = r.Stop()
			return err
		}
	}
	return nil
}

func (r *Runner) scan(rd io.Reader) {
	s := bufio.NewScanner(rd)
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, "Bootstrapped 100%") {
			r.bootOnce.Do(func() { close(r.bootstrapped) })
		}
	}
}

func (r *Runner) waitBootstrap(d time.Duration) error {
	select {
	case <-r.bootstrapped:
		return nil
	case <-time.After(d):
		return fmt.Errorf("timeout aguardando Tor iniciar")
	}
}

func (r *Runner) readOnion(d time.Duration) error {
	h := filepath.Join(config.HsDir(), "hostname")
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(h); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				r.Onion = s
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("não foi possível ler o hostname do hidden service")
}

func (r *Runner) Stop() error {
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Kill()
	}
	return nil
}
