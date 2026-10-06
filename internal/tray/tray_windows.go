//go:build windows

package tray

import "context"

// TrayIcon is a no-op tray implementation for Windows.
//
// A native Shell_NotifyIcon-based implementation was attempted but removed
// because the Go toolchain used to build this project lacks low-level
// Windows syscall helpers (e.g. syscall.NewLazySystemDLL). Windows users
// still get desktop notifications via Wails' runtime and can use the main
// window normally; there is simply no notification-area icon.
type TrayIcon struct{}

// NewTrayIcon returns a no-op tray icon on Windows.
func NewTrayIcon(_ interface{}) *TrayIcon { return &TrayIcon{} }

func (t *TrayIcon) Start(ctx context.Context) error { return nil }

func (t *TrayIcon) Stop() {}

func (t *TrayIcon) SetTitle(string) {}

func (t *TrayIcon) ShowMenu(*Menu) {}
