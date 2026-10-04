package clientui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

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

func (r *remoteCanvas) FocusGained() {}
func (r *remoteCanvas) FocusLost()   {}

func (r *remoteCanvas) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(r); c != nil {
		c.Focus(r)
	}
}

func (r *remoteCanvas) TypedRune(rn rune) {
	if r.cli == nil {
		return
	}
	s := string(rn)
	go func() {
		_ = r.cli.Key(s, true)
		_ = r.cli.Key(s, false)
	}()
}

func (r *remoteCanvas) TypedKey(e *fyne.KeyEvent) {
	if r.cli == nil {
		return
	}
	if name, ok := specialKeys[e.Name]; ok {
		go func() {
			_ = r.cli.Key(name, true)
			_ = r.cli.Key(name, false)
		}()
	}
}

func (r *remoteCanvas) KeyDown(e *fyne.KeyEvent) {
	if r.cli == nil {
		return
	}
	if name, ok := modifierKeys[e.Name]; ok {
		go r.cli.Key(name, true)
	}
}

func (r *remoteCanvas) KeyUp(e *fyne.KeyEvent) {
	if r.cli == nil {
		return
	}
	if name, ok := modifierKeys[e.Name]; ok {
		go r.cli.Key(name, false)
	}
}
