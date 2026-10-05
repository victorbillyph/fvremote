package tray

import "context"

type IconEvent func()

type MenuItem struct {
	Label string
	OnClick func()
}

type Menu struct {
	Items []MenuItem
	OnExit func()
}

type Tray interface {
	Start(ctx context.Context) error
	Stop()
	SetTitle(string)
	ShowMenu(*Menu)
}