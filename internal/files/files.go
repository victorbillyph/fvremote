package files

import (
	"io"
	"os"
	"path/filepath"

	"github.com/victorbillyph/fvremote/internal/config"
)

// Save grava um arquivo recebido na pasta de recebidos.
func Save(r io.Reader, name string) (string, error) {
	dst := filepath.Join(config.RecvDir(), filepath.Base(name))
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return dst, nil
}

// List lista o conteúdo da pasta de recebidos.
func List() ([]os.DirEntry, error) {
	return os.ReadDir(config.RecvDir())
}

// Path devolve o caminho absoluto de um arquivo em recebidos, impedindo
// travessia de diretório.
func Path(name string) (string, error) {
	return filepath.Join(config.RecvDir(), filepath.Base(name)), nil
}
