//go:build windows

package update

import (
	"os"
	"os/exec"
)

// replaceExecutable renomeia o exe em uso e coloca o novo no lugar.
func replaceExecutable(newBin, exe string) error {
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	return os.Rename(newBin, exe)
}

// Restart inicia o novo processo e encerra o atual.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
