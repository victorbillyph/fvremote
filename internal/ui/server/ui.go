package serverui

import (
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/victorbillyph/fvremote/internal/config"
	"github.com/victorbillyph/fvremote/internal/net/server"
	"github.com/victorbillyph/fvremote/internal/tor"
)

func Start() {
	a := app.New()
	w := a.NewWindow("fvremote — Servidor")
	w.Resize(fyne.NewSize(460, 260))

	onion := widget.NewLabel("Onion: —")
	onion.Wrapping = fyne.TextWrapBreak
	status := widget.NewLabel("Status: parado")

	tr := tor.NewServer()
	var srv *server.Srv

	btnStart := widget.NewButton("Iniciar servidor", func() {
		go func() {
			fyne.Do(func() { status.SetText("Status: baixando/iniciando Tor…") })
			if err := tr.Start(); err != nil {
				fyne.Do(func() {
					status.SetText("Status: erro")
					dialog.ShowError(err, w)
				})
				return
			}
			srv = server.New(tr.Onion)
			go func() { _ = srv.Run("127.0.0.1:8080") }()
			fyne.Do(func() {
				onion.SetText("Onion: " + tr.Onion)
				status.SetText("Status: online")
			})
		}()
	})

	btnOpen := widget.NewButton("Abrir pasta de recebidos", func() {
		openFolder(config.RecvDir())
		dialog.ShowInformation("Pasta de recebidos", config.RecvDir(), w)
	})

	btnStop := widget.NewButton("Parar", func() {
		_ = tr.Stop()
		status.SetText("Status: parado")
		onion.SetText("Onion: —")
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("fvremote — servidor via Tor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		onion,
		status,
		container.NewHBox(btnStart, btnStop),
		btnOpen,
	)
	w.SetContent(content)
	w.ShowAndRun()
}

func openFolder(path string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("explorer", path).Start()
	case "darwin":
		_ = exec.Command("open", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}
