package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/victorbillyph/fvremote/internal/hub"
)

type filesTab struct {
	app    *App
	path   string
	list   *widget.List
	items  []hub.FileItem
	sel    int
	pathLb *widget.Label
	root   fyne.CanvasObject
}

func newFilesTab(app *App) *filesTab {
	f := &filesTab{app: app, sel: -1}
	f.pathLb = widget.NewLabel("~")
	f.list = widget.NewList(
		func() int { return len(f.items) },
		func() fyne.CanvasObject { return widget.NewLabel("modelo") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < len(f.items) {
				it := f.items[id]
				prefix := "  "
				if it.Dir {
					prefix = "[dir] "
				}
				o.(*widget.Label).SetText(fmt.Sprintf("%s%s  (%d bytes)", prefix, it.Name, it.Size))
			}
		},
	)
	f.list.OnSelected = func(id widget.ListItemID) {
		f.sel = id
		if id >= 0 && id < len(f.items) && f.items[id].Dir {
			f.path = f.items[id].Path
			f.refresh()
		}
	}

	up := widget.NewButton("↑ Subir", func() {
		f.path = filepath.Dir(f.path)
		f.refresh()
	})
	refresh := widget.NewButton("Atualizar", func() { f.refresh() })
	download := widget.NewButton("Baixar", func() { f.downloadSelected() })
	upload := widget.NewButton("Enviar arquivo", func() { f.upload() })

	top := container.NewBorder(nil, nil, nil, container.NewHBox(refresh, download, upload), container.NewHBox(up, f.pathLb))
	f.root = container.NewBorder(top, nil, nil, nil, f.list)
	return f
}

func (f *filesTab) object() fyne.CanvasObject { return f.root }

func (f *filesTab) refresh() {
	a := f.app
	if a.rem == nil || a.sessionID == "" || a.permission != "full" {
		return
	}
	if f.path == "" {
		f.path, _ = homeDir()
	}
	f.pathLb.SetText(f.path)
	go func() {
		items, err := a.rem.List(a.sessionID, f.path)
		if err != nil {
			fyne.Do(func() { a.setStatus(err.Error(), "err") })
			return
		}
		fyne.Do(func() {
			f.items = items
			f.sel = -1
			f.list.Refresh()
		})
	}()
}

func (f *filesTab) downloadSelected() {
	a := f.app
	if f.sel < 0 || f.sel >= len(f.items) || f.items[f.sel].Dir {
		dialog.ShowInformation("fvremote", "Selecione um arquivo para baixar.", a.win)
		return
	}
	it := f.items[f.sel]
	dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil || wc == nil {
			return
		}
		dst := wc.URI().Path()
		_ = wc.Close()
		go func() {
			if err := a.rem.Download(a.sessionID, it.Path, dst); err != nil {
				fyne.Do(func() { a.setStatus(err.Error(), "err") })
			}
		}()
	}, a.win)
}

func (f *filesTab) upload() {
	a := f.app
	if a.rem == nil || a.sessionID == "" || a.permission != "full" {
		dialog.ShowInformation("fvremote", "É necessário acesso total para enviar arquivos.", a.win)
		return
	}
	dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		path := rc.URI().Path()
		_ = rc.Close()
		go func() {
			if err := a.rem.Upload(a.sessionID, path); err != nil {
				fyne.Do(func() { a.setStatus(err.Error(), "err") })
				return
			}
			fyne.Do(func() { a.setStatus("Arquivo enviado: "+filepath.Base(path), "ok") })
		}()
	}, a.win)
}

func homeDir() (string, error) {
	return os.UserHomeDir()
}
