package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// BaseDir retorna a pasta exclusiva do fvremote, por sistema operacional.
//
//	Linux/macOS: ~/.config/fvremote
//	Windows:     %APPDATA%\fvremote
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

func EnsureBase() error { return os.MkdirAll(BaseDir(), 0o700) }

// TorDir contém o binário standalone do Tor e suas bibliotecas (uso exclusivo).
func TorDir() string { return filepath.Join(BaseDir(), "tor") }

func TorDataDir() string { return filepath.Join(TorDir(), "data") }

// TorBin retorna o caminho do binário standalone do Tor exclusivo do fvremote.
func TorBin() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(TorDir(), "tor.exe")
	}
	return filepath.Join(TorDir(), "tor")
}

// IdentityPath guarda o código único e a chave derivada.
func IdentityPath() string { return filepath.Join(BaseDir(), "identity.json") }

// StatePath guarda o estado do setup (first-run).
func StatePath() string { return filepath.Join(BaseDir(), "state.json") }

// DownloadsDir é onde arquivos baixados do Cliente são salvos.
func DownloadsDir() string {
	d := filepath.Join(BaseDir(), "downloads")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// ReceivedDir é onde arquivos enviados pelo Suporte são salvos no Cliente.
func ReceivedDir() string {
	d := filepath.Join(BaseDir(), "received")
	_ = os.MkdirAll(d, 0o755)
	return d
}
