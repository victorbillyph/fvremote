//go:build !windows

package shell

import "os/exec"

func setBackground(cmd *exec.Cmd) {}
