package main

import (
	"encoding/json"
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// buildMainMenu keeps the housekeeping actions (sharing a setup, reporting a
// problem, checking for updates) out of the main window, where only the
// day-to-day tunnel controls belong.
func (a *App) buildMainMenu() *fyne.MainMenu {
	file := fyne.NewMenu("File",
		fyne.NewMenuItem("Import setup...", a.doImportConfig),
		fyne.NewMenuItem("Export setup...", a.doExportConfig),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", a.quit),
	)

	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("Copy diagnostics", a.doCopyDiagnostics),
		fyne.NewMenuItem("Check for updates", a.doCheckForUpdates),
	)

	return fyne.NewMainMenu(file, help)
}

// ---------- share a setup between teammates ----------

func (a *App) doExportConfig() {
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()

		data, err := json.MarshalIndent(a.store, "", "  ")
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if _, err := w.Write(data); err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		dialog.ShowInformation("Exported",
			fmt.Sprintf("Saved %d project(s) and %d tunnel(s).\nPasswords are never included.",
				len(a.store.Projects), len(a.store.Tunnels)), a.win)
	}, a.win)

	save.SetFileName("stp-tunnel-setup.json")
	save.Show()
}

func (a *App) doImportConfig() {
	open := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()

		data, err := io.ReadAll(r)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		var incoming Store
		if err := json.Unmarshal(data, &incoming); err != nil {
			dialog.ShowError(fmt.Errorf("this doesn't look like an exported setup: %w", err), a.win)
			return
		}

		projects, tunnels := a.store.Merge(&incoming)
		if projects == 0 && tunnels == 0 {
			dialog.ShowInformation("Nothing New", "Everything in that file is already here.", a.win)
			return
		}

		if err := a.store.Save(); err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		if a.projectID == "" && len(a.store.Projects) > 0 {
			a.projectID = a.store.Projects[0].ID
		}
		a.projectSelect.SetOptions(projectNames(a.store))
		if p, ok := a.currentProject(); ok {
			a.projectSelect.SetSelected(p.Name)
		}
		a.refreshRows()
		a.refreshKeyLabel()

		dialog.ShowInformation("Imported",
			fmt.Sprintf("Added %d project(s) and %d tunnel(s).\nPress Start and enter your own password to use them.",
				projects, tunnels), a.win)
	}, a.win)

	open.Show()
}

// ---------- support ----------

func (a *App) doCopyDiagnostics() {
	report := a.diagnostics()
	a.win.Clipboard().SetContent(report)

	preview := widget.NewMultiLineEntry()
	preview.SetText(report)
	preview.Disable()
	preview.TextStyle = fyne.TextStyle{Monospace: true}

	scroll := container.NewVScroll(preview)
	scroll.SetMinSize(fyne.NewSize(520, 320))

	content := container.NewBorder(
		widget.NewLabel("Copied to the clipboard — paste it into your bug report."),
		nil, nil, nil, scroll,
	)
	dialog.ShowCustom("Diagnostics", "Close", content, a.win)
}

func (a *App) doCheckForUpdates() {
	go func() {
		latest, err := checkForUpdate()

		fyne.Do(func() {
			switch {
			case err != nil:
				dialog.ShowError(err, a.win)
			case latest == nil:
				dialog.ShowInformation("Up to Date",
					fmt.Sprintf("You're on the latest version (%s).", versionDetail()), a.win)
			default:
				a.showUpdateAvailable(latest)
			}
		})
	}()
}

func (a *App) showUpdateAvailable(latest *releaseInfo) {
	body := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Version %s is available (you have %s).", latest.Version, version)),
	)
	if latest.Notes != "" {
		notes := widget.NewLabel(latest.Notes)
		notes.Wrapping = fyne.TextWrapWord
		body.Add(notes)
	}
	if latest.URL != "" {
		if u, err := parseURL(latest.URL); err == nil {
			body.Add(widget.NewHyperlink("Download it here", u))
		}
	}

	dialog.ShowCustom("Update Available", "Later", body, a.win)
}

// checkForUpdatesQuietly runs at startup: it only speaks up when there is
// actually something newer, and never blocks or nags on failure.
func (a *App) checkForUpdatesQuietly() {
	if updateFeedURL == "" {
		return
	}
	go func() {
		latest, err := checkForUpdate()
		if err != nil || latest == nil {
			return
		}
		fyne.Do(func() { a.showUpdateAvailable(latest) })
	}()
}
