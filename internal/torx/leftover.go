package torx

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/victorbillyph/fvremote/internal/config"
)

func pidFilePath() string { return filepath.Join(config.TorDir(), "tor.pid") }

// killLeftoverTor encerra qualquer Tor deste app que tenha ficado de uma
// execução anterior (crash) antes de subirmos um novo.
func killLeftoverTor() {
	if b, err := os.ReadFile(pidFilePath()); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			_ = killPID(pid)
		}
		_ = os.Remove(pidFilePath())
	}
	for _, pid := range findTorPIDs(config.TorBin()) {
		_ = killPID(pid)
	}
}

func writePIDFile(pid int) {
	_ = os.WriteFile(pidFilePath(), []byte(strconv.Itoa(pid)), 0o600)
}

func removePIDFile() { _ = os.Remove(pidFilePath()) }
