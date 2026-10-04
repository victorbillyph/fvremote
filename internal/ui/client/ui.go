package clientui

import (
	"fmt"
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/victorbillyph/fvremote/internal/net/client"
	"github.com/victorbillyph/fvremote/internal/tor"
)

// remoteCanvas exibe o stream da tela remota e envia eventos de mouse.
type remoteCanvas struct {
	widget.BaseWidget
	img  *canvas.Image
	cli  *client.Client
	info *client.Info
}

func newRemoteCanvas() *remoteCanvas {
	r := &remoteCanvas{
		img: canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 2, 2))),
	}
	r.img.FillMode = canvas.ImageFillContain
	r.img.ScaleMode = canvas.ImageScaleFastest
	r.ExtendBaseWidget(r)
	return r
}

func (r *remoteCanvas) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.img)
}

func (r *remoteCanvas) toRemote(pos fyne.Position) (int, int) {
	if r.info == nil || r.Size().Width == 0 || r.Size().Height == 0 {
		return 0, 0
	}
	sx := float32(pos.X) / r.Size().Width
	sy := float32(pos.Y) / r.Size().Height
	x := int(sx * float32(r.info.Width))
	y := int(sy * float32(r.info.Height))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= r.info.Width {
		x = r.info.Width - 1
	}
	if y >= r.info.Height {
		y = r.info.Height - 1
	}
	return x, y
}

func (r *remoteCanvas) MouseMoved(e *desktop.MouseEvent) {
	if r.cli == nil {
		return
	}
	x, y := r.toRemote(e.Position)
	go r.cli.Move(x, y)
}

func (r *remoteCanvas) MouseIn(*desktop.MouseEvent) {}
func (r *remoteCanvas) MouseOut()                   {}

func (r *remoteCanvas) MouseDown(e *desktop.MouseEvent) {
	if r.cli == nil {
		return
	}
	x, y := r.toRemote(e.Position)
	left := e.Button == desktop.MouseButtonPrimary
	go func() {
		_ = r.cli.Move(x, y)
		_ = r.cli.Click(left, true)
	}()
}

func (r *remoteCanvas) MouseUp(e *desktop.MouseEvent) {
	if r.cli == nil {
		return
	}
	left := e.Button == desktop.MouseButtonPrimary
	go r.cli.Click(left, false)
}

func (r *remoteCanvas) Scrolled(e *fyne.ScrollEvent) {
	if r.cli == nil {
		return
	}
	dy := int(e.Scrolled.DY)
	if dy == 0 {
		return
	}
	go r.cli.Scroll(dy)
}

// Start abre a janela do cliente.
func Start() {
	a := app.New()
	w := a.NewWindow("fvremote — Cliente")
	w.Resize(fyne.NewSize(1000, 640))

	onionEntry := widget.NewEntry()
	onionEntry.SetPlaceHolder("cole o endereço .onion aqui (ex.: abcd...xyz.onion)")

	screen := newRemoteCanvas()

	filesList := widget.NewList(
		func() int { return 0 },
		func() fyne.CanvasObject { return widget.NewLabel("modelo") },
		func(id widget.ListItemID, o fyne.CanvasObject) {},
	)

	status := widget.NewLabel("Status: desconectado")

	var cli *client.Client
	var items []client.FileItem
	var selected int = -1

	filesList.OnSelected = func(id widget.ListItemID) { selected = id }

	connectBtn := widget.NewButton("Conectar", func() {
		onion := onionEntry.Text
		if onion == "" {
			dialog.ShowError(fmt.Errorf("informe o endereço .onion"), w)
			return
		}
		status.SetText("Status: iniciando Tor…")
		go func() {
			tr := tor.NewClient()
			if err := tr.Start(); err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			c, err := client.New(tr.SocksAddr(), onion)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			info, err := c.Info()
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			fyne.Do(func() {
				cli = c
				screen.cli = c
				screen.info = info
				status.SetText("Status: conectado")
				w.Canvas().Focus(screen)
			})
			go fetchStream(c, screen, w)
			refreshFiles(c, filesList, &items, w)
		}()
	})

	refreshBtn := widget.NewButton("Atualizar arquivos", func() {
		if cli == nil {
			return
		}
		refreshFiles(cli, filesList, &items, w)
	})

	uploadBtn := widget.NewButton("Enviar arquivo", func() {
		if cli == nil {
			dialog.ShowInformation("fvremote", "Conecte-se primeiro.", w)
			return
		}
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			path := rc.URI().Path()
			_ = rc.Close()
			go func() {
				if err := cli.Upload(path); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
					return
				}
				refreshFiles(cli, filesList, &items, w)
			}()
		}, w)
	})

	downloadBtn := widget.NewButton("Baixar selecionado", func() {
		if cli == nil || selected < 0 || selected >= len(items) {
			dialog.ShowInformation("fvremote", "Selecione um arquivo.", w)
			return
		}
		it := items[selected]
		if it.Dir {
			return
		}
		dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			dst := wc.URI().Path()
			_ = wc.Close()
			go func() {
				if err := cli.Download(it.Name, dst); err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
				}
			}()
		}, w)
	})

	filesTab := container.NewBorder(
		container.NewHBox(refreshBtn, uploadBtn, downloadBtn),
		nil, nil, nil,
		filesList,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Tela", screen),
		container.NewTabItem("Arquivos", filesTab),
	)

	top := container.NewVBox(onionEntry, container.NewHBox(connectBtn, status))
	w.SetContent(container.NewBorder(top, nil, nil, nil, tabs))

	w.ShowAndRun()
}

func refreshFiles(c *client.Client, list *widget.List, items *[]client.FileItem, w fyne.Window) {
	go func() {
		res, err := c.List()
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		fyne.Do(func() {
			*items = res
			list.Length = func() int { return len(res) }
			list.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
				label := o.(*widget.Label)
				if id < len(res) {
					label.SetText(fmt.Sprintf("%s  (%d bytes)", res[id].Name, res[id].Size))
				}
			}
			list.Refresh()
		})
	}()
}
