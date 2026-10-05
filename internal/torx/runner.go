package torx

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/victorbillyph/fvremote/internal/config"
)

var bootstrapRe = regexp.MustCompile(`Bootstrapped (\d+)%`)

// Runner gerencia um processo Tor standalone exclusivo do fvremote, já com o
// control port autenticado para criação de onion services determinísticos.
type Runner struct {
	SocksPort   int
	ControlPort int

	cmd          *exec.Cmd
	ctrl         *control
	serviceID    string
	bootstrapped chan struct{}
	once         sync.Once
	progress     func(stage string, pct int)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Start baixa (se necessário), sobe o Tor e aguarda o bootstrap completo.
// onProgress reporta a etapa atual ("download", "bootstrap", "ready").
func Start(onProgress func(stage string, pct int)) (*Runner, error) {
	if onProgress == nil {
		onProgress = func(string, int) {}
	}
	onProgress("download", 0)
	if err := Ensure(func(written, total int64) {
		if total > 0 {
			onProgress("download", int(written*100/total))
		}
	}); err != nil {
		return nil, err
	}

	// Encerra um Tor remanescente deste app antes de subir um novo.
	killLeftoverTor()

	socks, err := freePort()
	if err != nil {
		return nil, err
	}
	ctrlPort, err := freePort()
	if err != nil {
		return nil, err
	}

	rc := filepath.Join(config.TorDir(), "fvremote-torrc")
	f, err := os.Create(rc)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "SocksPort 127.0.0.1:%d\n", socks)
	fmt.Fprintf(f, "ControlPort 127.0.0.1:%d\n", ctrlPort)
	fmt.Fprintf(f, "CookieAuthentication 1\n")
	fmt.Fprintf(f, "DataDirectory %s\n", config.TorDataDir())
	fmt.Fprintf(f, "Log notice stdout\n")
	fmt.Fprintf(f, "AvoidDiskWrites 1\n")
	_ = f.Close()

	cmd := exec.Command(config.TorBin(), "-f", rc)
	cmd.Env = os.Environ()
	switch runtime.GOOS {
	case "linux":
		cmd.Env = append(cmd.Env, "LD_LIBRARY_PATH="+config.TorDir())
	case "darwin":
		cmd.Env = append(cmd.Env, "DYLD_LIBRARY_PATH="+config.TorDir())
	}
	applyBackground(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	r := &Runner{
		SocksPort:    socks,
		ControlPort:  ctrlPort,
		cmd:          cmd,
		bootstrapped: make(chan struct{}),
		progress:     onProgress,
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if cmd.Process != nil {
		writePIDFile(cmd.Process.Pid)
	}
	go r.scan(pr)
	go func() { _ = cmd.Wait() }()

	onProgress("bootstrap", 0)
	select {
	case <-r.bootstrapped:
		onProgress("bootstrap", 100)
	case <-time.After(360 * time.Second):
		_ = r.Stop()
		return nil, fmt.Errorf("timeout ao inicializar o Tor")
	}

	ctrl, err := dialControl(fmt.Sprintf("127.0.0.1:%d", ctrlPort), filepath.Join(config.TorDataDir(), "control_auth_cookie"))
	if err != nil {
		_ = r.Stop()
		return nil, err
	}
	r.ctrl = ctrl
	onProgress("ready", 100)
	return r, nil
}

func (r *Runner) scan(rd io.Reader) {
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		line := s.Text()
		if os.Getenv("FVREMOTE_TOR_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "[tor] "+line)
		}
		if m := bootstrapRe.FindStringSubmatch(line); m != nil {
			if pct, err := strconv.Atoi(m[1]); err == nil {
				if r.progress != nil {
					r.progress("bootstrap", pct)
				}
				if pct >= 100 {
					r.once.Do(func() { close(r.bootstrapped) })
				}
			}
		}
	}
}

// SocksAddr devolve o endereço SOCKS5 local para discar .onion.
func (r *Runner) SocksAddr() string { return fmt.Sprintf("127.0.0.1:%d", r.SocksPort) }

// AddOnion registra um serviço oculto determinístico e retorna o ServiceID.
func (r *Runner) AddOnion(blobB64 string, virtPort, targetPort int) (string, error) {
	if r.ctrl == nil {
		return "", fmt.Errorf("control port não conectado")
	}
	cmd := fmt.Sprintf("ADD_ONION ED25519-V3:%s Port=%d,127.0.0.1:%d", blobB64, virtPort, targetPort)
	resp, err := r.ctrl.cmd(cmd)
	if err != nil {
		return "", err
	}
	if !strings.Contains(resp, "250") {
		return "", fmt.Errorf("ADD_ONION falhou: %s", strings.TrimSpace(resp))
	}
	id := ""
	for _, line := range strings.Split(resp, "\n") {
		if strings.Contains(line, "ServiceID=") {
			idx := strings.Index(line, "ServiceID=")
			id = strings.TrimSpace(line[idx+len("ServiceID="):])
			break
		}
	}
	if id == "" {
		return "", fmt.Errorf("ServiceID ausente na resposta: %s", strings.TrimSpace(resp))
	}
	r.serviceID = id
	return id, nil
}

// DelOnion remove o serviço oculto criado.
func (r *Runner) DelOnion(id string) error {
	if r.ctrl == nil || id == "" {
		return nil
	}
	_, err := r.ctrl.cmd("DEL_ONION " + id)
	return err
}

// Stop encerra o Tor.
func (r *Runner) Stop() error {
	if r.ctrl != nil {
		if r.serviceID != "" {
			_, _ = r.ctrl.cmd("DEL_ONION " + r.serviceID)
		}
		r.ctrl.close()
		r.ctrl = nil
	}
	if r.cmd != nil && r.cmd.Process != nil {
		pid := r.cmd.Process.Pid
		_ = r.cmd.Process.Kill()
		_ = killPID(pid) // garante o encerramento mesmo se Kill falhar
		removePIDFile()
	}
	return nil
}
