package ui

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// remoteCanvas exibe o stream da tela remota e envia eventos de mouse/teclado.
type remoteCanvas struct {
	widget.BaseWidget
	app *App
	img *canvas.Image
}

func newRemoteCanvas(app *App) *remoteCanvas {
	r := &remoteCanvas{
		app: app,
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
	a := r.app
	if a.remoteW == 0 || r.Size().Width == 0 || r.Size().Height == 0 {
		return 0, 0
	}
	sx := float32(pos.X) / r.Size().Width
	sy := float32(pos.Y) / r.Size().Height
	x := int(sx * float32(a.remoteW))
	y := int(sy * float32(a.remoteH))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= a.remoteW {
		x = a.remoteW - 1
	}
	if y >= a.remoteH {
		y = a.remoteH - 1
	}
	return x, y
}

func (r *remoteCanvas) canControl() bool {
	return r.app != nil && r.app.rem != nil && r.app.permission == "full" && r.app.sessionID != ""
}

func (r *remoteCanvas) MouseMoved(e *desktop.MouseEvent) {
	if !r.canControl() {
		return
	}
	x, y := r.toRemote(e.Position)
	go r.app.rem.Move(r.app.sessionID, x, y)
}

func (r *remoteCanvas) MouseIn(*desktop.MouseEvent) {}
func (r *remoteCanvas) MouseOut()                   {}

func (r *remoteCanvas) MouseDown(e *desktop.MouseEvent) {
	if !r.canControl() {
		return
	}
	x, y := r.toRemote(e.Position)
	left := e.Button == desktop.MouseButtonPrimary
	go func() {
		_ = r.app.rem.Move(r.app.sessionID, x, y)
		_ = r.app.rem.Click(r.app.sessionID, left, true)
	}()
}

func (r *remoteCanvas) MouseUp(e *desktop.MouseEvent) {
	if !r.canControl() {
		return
	}
	left := e.Button == desktop.MouseButtonPrimary
	go r.app.rem.Click(r.app.sessionID, left, false)
}

func (r *remoteCanvas) Scrolled(e *fyne.ScrollEvent) {
	if !r.canControl() {
		return
	}
	if dy := int(e.Scrolled.DY); dy != 0 {
		go r.app.rem.Scroll(r.app.sessionID, dy)
	}
}

func (r *remoteCanvas) FocusGained() {}
func (r *remoteCanvas) FocusLost()   {}

func (r *remoteCanvas) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(r); c != nil {
		c.Focus(r)
	}
}

func (r *remoteCanvas) TypedRune(rn rune) {
	if !r.canControl() {
		return
	}
	s := string(rn)
	go func() {
		_ = r.app.rem.Key(r.app.sessionID, s, true)
		_ = r.app.rem.Key(r.app.sessionID, s, false)
	}()
}

func (r *remoteCanvas) TypedKey(e *fyne.KeyEvent) {
	if !r.canControl() {
		return
	}
	if name, ok := specialKeys[e.Name]; ok {
		go func() {
			_ = r.app.rem.Key(r.app.sessionID, name, true)
			_ = r.app.rem.Key(r.app.sessionID, name, false)
		}()
	}
}

func (r *remoteCanvas) KeyDown(e *fyne.KeyEvent) {
	if !r.canControl() {
		return
	}
	if name, ok := modifierKeys[e.Name]; ok {
		go r.app.rem.Key(r.app.sessionID, name, true)
	}
}

func (r *remoteCanvas) KeyUp(e *fyne.KeyEvent) {
	if !r.canControl() {
		return
	}
	if name, ok := modifierKeys[e.Name]; ok {
		go r.app.rem.Key(r.app.sessionID, name, false)
	}
}

// readFrames lê frames JPEG (prefixo de 4 bytes big-endian) e atualiza o canvas.
func (r *remoteCanvas) readFrames(body io.ReadCloser) {
	defer body.Close()
	br := bufio.NewReader(body)
	var hdr [4]byte
	for {
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n == 0 || n > 64<<20 {
			return
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(br, data); err != nil {
			return
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		fyne.Do(func() {
			r.img.Image = img
			r.img.Refresh()
		})
	}
}

var specialKeys = map[fyne.KeyName]string{
	fyne.KeyReturn:    "enter",
	fyne.KeyEnter:     "enter",
	fyne.KeyBackspace: "backspace",
	fyne.KeyTab:       "tab",
	fyne.KeyEscape:    "esc",
	fyne.KeyDelete:    "delete",
	fyne.KeyInsert:    "insert",
	fyne.KeyUp:        "up",
	fyne.KeyDown:      "down",
	fyne.KeyLeft:      "left",
	fyne.KeyRight:     "right",
	fyne.KeyHome:      "home",
	fyne.KeyEnd:       "end",
	fyne.KeyPageUp:    "pageup",
	fyne.KeyPageDown:  "pagedown",
	fyne.KeySpace:     "space",
	fyne.KeyF1:        "f1",
	fyne.KeyF2:        "f2",
	fyne.KeyF3:        "f3",
	fyne.KeyF4:        "f4",
	fyne.KeyF5:        "f5",
	fyne.KeyF6:        "f6",
	fyne.KeyF7:        "f7",
	fyne.KeyF8:        "f8",
	fyne.KeyF9:        "f9",
	fyne.KeyF10:       "f10",
	fyne.KeyF11:       "f11",
	fyne.KeyF12:       "f12",
}

var modifierKeys = map[fyne.KeyName]string{
	desktop.KeyShiftLeft:    "shift",
	desktop.KeyShiftRight:   "shift",
	desktop.KeyControlLeft:  "ctrl",
	desktop.KeyControlRight: "ctrl",
	desktop.KeyAltLeft:      "alt",
	desktop.KeyAltRight:     "alt",
	desktop.KeySuperLeft:    "cmd",
	desktop.KeySuperRight:   "cmd",
}
