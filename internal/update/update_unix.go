//go:build !windows

package update

import (
	"os"
	"syscall"
)

// replaceExecutable troca o binário atual (permitido no Linux mesmo em execução).
func replaceExecutable(newBin, exe string) error {
	return os.Rename(newBin, exe)
}

// Restart reinicia o app com o novo binário.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}
