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
	session *supportSession
	path    string
	list    *widget.List
	items   []hub.FileItem
	sel     int
	pathLb  *widget.Label
	root    fyne.CanvasObject
}

func newFilesTab(s *supportSession) *filesTab {
	f := &filesTab{session: s, sel: -1}
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
	upload := widget.NewButton("Enviar", func() { f.upload() })

	top := container.NewBorder(nil, nil, nil, container.NewHBox(refresh, download, upload), container.NewHBox(up, f.pathLb))
	f.root = container.NewBorder(top, nil, nil, nil, f.list)
	return f
}

func (f *filesTab) object() fyne.CanvasObject { return f.root }

func (f *filesTab) refresh() {
	s := f.session
	if s == nil || s.rem == nil || s.id == "" || s.permission != hub.PermFull {
		return
	}
	if f.path == "" {
		f.path, _ = os.UserHomeDir()
	}
	f.pathLb.SetText(f.path)
	go func() {
		items, err := s.rem.List(s.id, f.path)
		if err != nil {
			fyne.Do(func() { s.setStatus(err.Error(), "err") })
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
	s := f.session
	if f.sel < 0 || f.sel >= len(f.items) || f.items[f.sel].Dir {
		dialog.ShowInformation("fvremote", "Selecione um arquivo para baixar.", s.app.win)
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
			if err := s.rem.Download(s.id, it.Path, dst); err != nil {
				fyne.Do(func() { s.setStatus(err.Error(), "err") })
			}
		}()
	}, s.app.win)
}

func (f *filesTab) upload() {
	s := f.session
	if s.permission != hub.PermFull {
		dialog.ShowInformation("fvremote", "É necessário acesso total para enviar arquivos.", s.app.win)
		return
	}
	dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		path := rc.URI().Path()
		_ = rc.Close()
		go func() {
			if err := s.rem.Upload(s.id, path); err != nil {
				fyne.Do(func() { s.setStatus(err.Error(), "err") })
				return
			}
			fyne.Do(func() { s.setStatus("Arquivo enviado: "+filepath.Base(path), "ok") })
		}()
	}, s.app.win)
}
