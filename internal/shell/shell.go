package shell

import (
	"os/exec"
	"runtime"
	"time"
)

// Result é o resultado de um comando executado.
type Result struct {
	Output string `json:"output"`
	Error  string `json:"error"`
	Code   int    `json:"code"`
}

// Exec roda um comando no shell do sistema (sh -c ou cmd /C) e devolve a saída.
func Exec(command string) Result {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	done := make(chan Result, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		res := Result{Output: string(out)}
		if err != nil {
			res.Error = err.Error()
		}
		if cmd.ProcessState != nil {
			res.Code = cmd.ProcessState.ExitCode()
		}
		done <- res
	}()

	select {
	case res := <-done:
		return res
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return Result{Error: "tempo esgotado (30s)", Code: -1}
	}
}

// Name devolve o nome do shell padrão da plataforma.
func Name() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}
