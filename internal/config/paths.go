package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// BaseDir retorna a pasta exclusiva do fvremote, por sistema operacional.
// Linux/macOS: ~/.config/fvremote
// Windows:     %APPDATA%\fvremote
func BaseDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		app := os.Getenv("APPDATA")
		if app == "" {
			app = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(app, "fvremote")
	default:
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		return filepath.Join(cfg, "fvremote")
	}
}

func TorDir() string     { return filepath.Join(BaseDir(), "tor") }
func TorDataDir() string { return filepath.Join(TorDir(), "data") }
func HsDir() string      { return filepath.Join(TorDir(), "hs") }

// TorBin retorna o caminho do binário standalone do Tor exclusivo do fvremote.
func TorBin() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(TorDir(), "tor.exe")
	}
	return filepath.Join(TorDir(), "tor")
}

// RecvDir é a pasta onde arquivos enviados pelo cliente são salvos.
func RecvDir() string {
	d := filepath.Join(BaseDir(), "received")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func EnsureBase() error {
	return os.MkdirAll(BaseDir(), 0o700)
}
