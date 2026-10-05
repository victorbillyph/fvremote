//go:build !windows

package torx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/victorbillyph/fvremote/internal/config"
)

func TestFindAndKillLeftoverTor(t *testing.T) {
	if _, err := os.Stat(config.TorBin()); err != nil {
		t.Skip("binário do tor ainda não baixado")
	}
	dir := t.TempDir()
	torrc := filepath.Join(dir, "torrc")
	_ = os.WriteFile(torrc, []byte(
		"SocksPort 0\nDataDirectory "+filepath.Join(dir, "data")+"\nLog notice stdout\n"), 0o600)

	cmd := exec.Command(config.TorBin(), "-f", torrc)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+config.TorDir())
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	defer func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}()

	time.Sleep(700 * time.Millisecond)

	found := false
	for _, p := range findTorPIDs(config.TorBin()) {
		if p == pid {
			found = true
		}
	}
	if !found {
		t.Fatalf("findTorPIDs não encontrou o tor %d (achou %v)", pid, findTorPIDs(config.TorBin()))
	}

	killLeftoverTor()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("o Tor não foi encerrado por killLeftoverTor")
	}
}
