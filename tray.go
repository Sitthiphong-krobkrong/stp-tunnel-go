package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

//go:embed icon.png
var iconBytes []byte

var appIcon = fyne.NewStaticResource("icon.png", iconBytes)

// setupSystemTray adds the tray menu (Show / Stop all tunnels / Quit) where
// the platform's desktop.App supports one.
//
// It deliberately does NOT intercept the window close button to hide instead
// of quit: the tray icon itself doesn't reliably render on macOS with the
// current fyne.io/systray (known upstream issue, see fyne-io/fyne#4548 —
// the status item registers with Control Center but its icon never gets
// applied, so it's invisible). Hiding on close with no visible tray icon to
// bring the window back from would strand the app running with no way to
// reach it short of Force Quit. Revisit once that's fixed upstream.
func (a *App) setupSystemTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}

	menu := fyne.NewMenu("STP Tunnel",
		fyne.NewMenuItem("Show STP Tunnel", func() {
			fyne.Do(func() {
				a.win.Show()
				a.win.RequestFocus()
			})
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Stop all tunnels", func() {
			a.runtime.StopAll()
			fyne.Do(a.refreshRows)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", a.quit),
	)

	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(appIcon)
}

// quit stops every running tunnel before actually exiting — the tray's Quit
// item and the File menu's both go through here so a background tunnel is
// never left dangling after the process ends.
func (a *App) quit() {
	a.runtime.StopAll()
	a.fyneApp.Quit()
}
