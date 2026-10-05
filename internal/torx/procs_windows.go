//go:build windows

package torx

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

// findTorPIDs retorna os PIDs de tor.exe cujo caminho é o binário do fvremote.
func findTorPIDs(bin string) []int {
	ps := `Get-CimInstance Win32_Process -Filter "name='tor.exe'" | ForEach-Object { "$($_.ProcessId)|$($_.ExecutablePath)" }`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		if len(parts) == 2 {
			p := strings.TrimSpace(parts[1])
			if p == "" || !strings.EqualFold(p, bin) {
				continue
			}
		}
		pids = append(pids, pid)
	}
	return pids
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// applyBackground evita que o Tor abra uma janela de console no Windows.
func applyBackground(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
