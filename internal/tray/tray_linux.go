//go:build linux

package tray

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
)

type IconStatus int

const (
	IconStatusNormal IconStatus = iota
	IconStatusAttention
	IconStatusOffline
)



type TrayIcon struct {
	bus              *dbus.Conn
	name             string
	identifier       string
	timeout          uint32
	category         string
	iconName         string
	status           IconStatus
	menu             *Menu
	balloonTimeout   uint32
	balloonIcon      string
	balloonTitle     string
	balloonText      string
	interactive      bool
	xa               dbus.BusObject
	notifications    map[uint32]*notification
	notificationLock struct{}
}

type notification struct {
	id         uint32
	identifier string
	timeout    uint32
	category   string
	iconName   string
	status     IconStatus
	title      string
	body       string
	actions    []string
	actionMap  map[string]string
}

func NewTrayIcon(bus *dbus.Conn) *TrayIcon {
	return &TrayIcon{
		bus:      bus,
		name:     "fvremote",
		iconName: "application-x-executable",
		status:   IconStatusNormal,
		notifications: make(map[uint32]*notification),
	}
}

func (t *TrayIcon) Start(ctx context.Context) error {
	// Ensure we have a session bus
	if t.bus == nil {
		var err error
		t.bus, err = dbus.SessionBus()
		if err != nil {
			return fmt.Errorf("failed to connect to session bus: %w", err)
		}
	}

	// Request name on the session bus (best-effort)
	_, err := t.bus.RequestName("org.freedesktop.Notifications", dbus.RequestNameFlags(0))
	if err != nil {
		_ = err // non-fatal; we'll still send plain notifications
	}

	// Get the notifications object
	t.xa = t.bus.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		t.Stop()
	}()

	return nil
}

func (t *TrayIcon) Stop() {
	// Remove any pending notifications
	for id := range t.notifications {
		t.removeNotification(id)
	}
	t.notifications = make(map[uint32]*notification)
}

func (t *TrayIcon) SetTitle(title string) {
	t.category = title
}

func (t *TrayIcon) ShowMenu(m *Menu) {
	t.menu = m
	// Best-effort: if we have a menu, note it
	// Full popup menu would require X11/Wayland integration
	_ = m
}

func (t *TrayIcon) addNotification(n *notification) (uint32, error) {
	if t.xa == nil {
		return 0, fmt.Errorf("notifications not initialized")
	}

	// Build actions list and action map
	actions := make([]string, 0, len(n.actions)+len(n.actionMap)*2)
	actionMap := make(map[string]string, len(n.actionMap))
	for _, a := range n.actions {
		actions = append(actions, a)
		actionMap[a] = a // simplified: use the title as both ID and title
	}

	hints := make(map[string]dbus.Variant)
	hints["x-notification-id"] = dbus.MakeVariant(n.identifier)
	if n.category != "" {
		hints["x-category-id"] = dbus.MakeVariant(n.category)
	}

	obj := t.xa
	call := obj.Call(
		"org.freedesktop.Notifications.Notify",
		0,
		t.name,
		uint32(0),
		n.iconName,
		n.title,
		n.body,
		actions,
		hints,
		int32(n.timeout),
	)

	if call.Err != nil {
		return 0, fmt.Errorf("failed to send notification: %w", call.Err)
	}

	var dbusID uint32
	if err := call.Store(&dbusID); err != nil {
		return 0, fmt.Errorf("failed to store notification ID: %w", err)
	}

	notif := &notification{
		id:         dbusID,
		identifier: n.identifier,
		timeout:    n.timeout,
		category:   n.category,
		iconName:   n.iconName,
		status:     n.status,
		title:      n.title,
		body:       n.body,
		actions:    n.actions,
		actionMap:  actionMap,
	}

	t.notifications[dbusID] = notif
	return dbusID, nil
}

func (t *TrayIcon) removeNotification(id uint32) {
	if t.xa == nil {
		return
	}
	obj := t.xa
	call := obj.Call(
		"org.freedesktop.Notifications.CloseNotification",
		0,
		id,
	)
	_ = call
	delete(t.notifications, id)
}

func (t *TrayIcon) SendNotification(n *notification) error {
	_, err := t.addNotification(n)
	return err
}

func (t *TrayIcon) setStatus(s IconStatus) {
	t.status = s
	// Update the indicator status if possible
	_ = s
}

func (t *TrayIcon) setBalloon(icon, title, text string, timeout uint32) {
	t.balloonIcon = icon
	t.balloonTitle = title
	t.balloonText = text
	t.balloonTimeout = timeout
	// Best-effort balloon display via regular notification
	if t.xa != nil {
		n := &notification{
			identifier: "balloon",
			title:      title,
			body:       text,
			timeout:    timeout,
		}
		_ = t.SendNotification(n)
	}
}