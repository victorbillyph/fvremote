//go:build windows

package tray

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	moduser32             = syscall.NewLazySystemDLL("user32.dll")
	procShellNotifyIcon   = moduser32.NewProc("Shell_NotifyIcon")
	procLoadImageW        = moduser32.NewProc("LoadImageW")
	procSetCursor         = moduser32.NewProc("SetCursor")
)

const (
	nimAdd        = 0x00000000
	nimRemove     = 0x00000002
	nifIcon       = 0x00000002
	nifTooltip    = 0x00000004
	nifMessage    = 0x00000001
	idTray        = 0
	nireMessage   = 0x00000000
)

type WINNOTIFYICONDATA struct {
	CbSize     uint32
	HWnd       syscall.HWND
	UID        uint32
	UFlags     uint32
	HIcon      syscall.HICON
	SzTip      [128]uint16
	UnionFlags uint32
	HoverIcon  syscall.HICON
}

type winTray struct {
	running  bool
	events   chan struct{}
	hwnd     syscall.HWND
	hinst    syscall.HINSTANCE
	icon     syscall.HICON
	tooltip  string
}

func (t *winTray) Start(ctx context.Context) error {
	t.running = true
	t.events = make(chan struct{}, 16)

	// Get the executable path to load its icon
	exe, _ := os.Executable()

	// Load the small icon from the executable
	icon, err := loadIcon(exe)
	if err != nil {
		runtime.KeepAlive(nil)
		icon = 0
	}

	// Create a hidden window to receive tray messages
	szClass := "fvremoteTrayClass"
	atom := syscall.GlobalAddAtom(syscall.StringToUTF16Ptr(szClass))
	if atom == 0 {
		return syscall.GetLastError()
	}

	// Window procedure
	wndProc := syscall.NewCallback(func(hwnd syscall.HWND, msg uint32, wparam, lparam uintptr) uintptr {
		if msg == uint32(syscall.WM_TASKBAR_CREATED) || true {
			// We'll handle this differently
		}
		return syscall.DefWindowProc(hwnd, msg, wparam, lparam)
	})

	// Register window class
	wc := &syscall.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(syscall.WNDCLASSEX{})),
		LpfnWndProc:   wndProc,
		ClsName:       syscall.StringToUTF16Ptr(szClass),
		HInstance:     syscall.GetModuleHandle(nil),
		HIcon:         0,
		HCursor:       0,
		HbrBackground: 0,
		LpszMenuName:  nil,
		LpszClassName: syscall.StringToUTF16Ptr(szClass),
	}
	classes := syscall.RegisterClassEx(wc)

	// Create the window
	hwnd, _ := syscall.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr(szClass),
		syscall.StringToUTF16Ptr("fvremote-tray"),
		0,
		0, 0, 1, 1,
		0,
		0,
		wc.HInstance,
		0,
	)
	t.hwnd = hwnd

	// Add tray icon
	var nid WINNOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = idTray
	nid.UFlags = nifIcon | nifTooltip | nifMessage
	nid.HIcon = icon
	copy(nid.SzTip[:], syscall.StringToUTF16Ptr("fvremote"))
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))

	// Listen for events in a goroutine
	go t.listenEvents()
	return nil
}

func (t *winTray) listenEvents() {
	for t.running {
		var msg syscall.MSG
		if syscall.GetMessage(&msg, 0, 0, 0) != 0 {
			if msg.Message == uint32(syscall.WM_COMMAND) && int(wparam(syscall.HIWORD(msg.WParam))) == idTray {
				select {
				case t.events <- struct{}{}:
				default:
				}
			}
			syscall.DispatchMessage(&msg)
		}
	}
}

func (t *winTray) Stop() {
	t.running = false
	if t.hwnd != 0 {
		var nid WINNOTIFYICONDATA
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.HWnd = t.hwnd
		nid.UID = idTray
		nid.UFlags = nimRemove
		procShellNotifyIcon.Call(nimRemove, uintptr(unsafe.Pointer(&nid)))
		syscall.DestroyWindow(t.hwnd)
	}
	if t.icon != 0 {
		syscall.DestroyIcon(t.icon)
	}
}

func (t *winTray) SetTitle(title string) {
	t.tooltip = title
	var nid WINNOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = t.hwnd
	nid.UID = idTray
	nid.UFlags = nifTooltip
	copy(nid.SzTip[:], syscall.StringToUTF16Ptr(title))
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (t *winTray) ShowMenu(m *Menu) {
	// Build a popup menu at mouse position
	// For now, just note the request
	_ = m
	// A full implementation would track mouse position and create a HMENU
}

func loadIcon(path string) (syscall.HICON, error) {
	icon, _, err := procLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		0x00000010, // IMAGE_ICON
		16, 16,     // desired width/height
		0,          // lrflags
	)
	if icon == 0 {
		return 0, err
	}
	return syscall.HICON(icon), nil
}

func wparam(lo uintptr) uintptr {
	return lo >> 16
}