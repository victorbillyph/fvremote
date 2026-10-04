package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Cores do tema do fvremote.
var (
	colBg      = color.NRGBA{R: 0x0F, G: 0x11, B: 0x17, A: 0xFF}
	colCard    = color.NRGBA{R: 0x17, G: 0x1B, B: 0x24, A: 0xFF}
	colField   = color.NRGBA{R: 0x1E, G: 0x23, B: 0x30, A: 0xFF}
	colFg      = color.NRGBA{R: 0xE6, G: 0xE9, B: 0xEF, A: 0xFF}
	colMuted   = color.NRGBA{R: 0x8A, G: 0x93, B: 0xA6, A: 0xFF}
	colPrimary = color.NRGBA{R: 0x4F, G: 0x8C, B: 0xFF, A: 0xFF}
	colOk      = color.NRGBA{R: 0x3D, G: 0xD5, B: 0x98, A: 0xFF}
	colWarn    = color.NRGBA{R: 0xF5, G: 0xB4, B: 0x3D, A: 0xFF}
	colErr     = color.NRGBA{R: 0xF5, G: 0x5E, B: 0x5E, A: 0xFF}
)

type appTheme struct{}

func newTheme() fyne.Theme { return appTheme{} }

func (appTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colCard
	case theme.ColorNameForeground:
		return colFg
	case theme.ColorNamePrimary:
		return colPrimary
	case theme.ColorNameButton:
		return colField
	case theme.ColorNameInputBackground:
		return colField
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0x2E, G: 0x36, B: 0x48, A: 0xFF}
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNameSuccess:
		return colOk
	case theme.ColorNameWarning:
		return colWarn
	case theme.ColorNameError:
		return colErr
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x28, G: 0x2F, B: 0x3E, A: 0xFF}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x33, G: 0x3B, B: 0x4D, A: 0xFF}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameText:
		return 14
	default:
		return theme.DefaultTheme().Size(name)
	}
}
