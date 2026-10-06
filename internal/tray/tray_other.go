//go:build !linux && !windows

package tray

import "context"

// TrayIcon é um stub para plataformas sem suporte a bandeja (Windows, macOS).
// Mantém a API idêntica à implementação Linux para que app.go compile em todos os sistemas.
type TrayIcon struct{}

// NewTrayIcon cria um ícone de bandeja no-op.
func NewTrayIcon(_ any) *TrayIcon { return &TrayIcon{} }

func (t *TrayIcon) Start(ctx context.Context) error { return nil }
func (t *TrayIcon) Stop()                           {}
func (t *TrayIcon) SetTitle(title string)           {}
func (t *TrayIcon) ShowMenu(m *Menu)                {}
