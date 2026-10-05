//go:build !windows

package torx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// findTorPIDs retorna os PIDs cujo executável é exatamente o binário do fvremote.
func findTorPIDs(bin string) []int {
	want, err := filepath.EvalSymlinks(bin)
	if err != nil || want == "" {
		want = bin
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		if r, err := filepath.EvalSymlinks(exe); err == nil {
			exe = r
		}
		if filepath.Clean(exe) == filepath.Clean(want) {
			pid, _ := strconv.Atoi(e.Name())
			pids = append(pids, pid)
		}
	}
	return pids
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// applyBackground não faz nada no Linux (não há janela de console).
func applyBackground(cmd *exec.Cmd) {}
