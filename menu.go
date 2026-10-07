package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

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

// doExportConfig asks which projects to share (the current one is ticked by
// default) — usually a teammate only needs the one project they're joining,
// not every bastion you happen to have.
func (a *App) doExportConfig() {
	if len(a.store.Projects) == 0 {
		dialog.ShowInformation("Nothing to Export", "Create a project first.", a.win)
		return
	}

	choices := make([]projectChoice, len(a.store.Projects))
	for i, p := range a.store.Projects {
		choices[i] = projectChoice{
			Project: p,
			Note:    fmt.Sprintf("%d tunnel(s)", len(a.store.TunnelsForProject(p.ID))),
			Checked: p.ID == a.projectID || len(a.store.Projects) == 1,
		}
	}

	a.pickProjects("Export Projects", "Export...", choices, func(ids []string) {
		a.saveExport(a.store.Subset(ids))
	})
}

func (a *App) saveExport(out *Store) {
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()

		data, err := json.MarshalIndent(out, "", "  ")
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
				len(out.Projects), len(out.Tunnels)), a.win)
	}, a.win)

	name := "stp-tunnel-setup.json"
	if len(out.Projects) == 1 {
		name = "stp-tunnel-" + fileSlug(out.Projects[0].Name) + ".json"
	}
	save.SetFileName(name)
	save.Show()
}

// doImportConfig reads a setup file and, when it holds more than one
// project, lets the user pick which ones to bring in. Projects that are
// already fully here are listed but can't be ticked.
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
		if err := json.Unmarshal(data, &incoming); err != nil || len(incoming.Projects) == 0 {
			if err == nil {
				err = fmt.Errorf("it contains no projects")
			}
			dialog.ShowError(fmt.Errorf("this doesn't look like an exported setup: %w", err), a.win)
			return
		}

		var choices []projectChoice
		for _, p := range incoming.Projects {
			_, exists := a.store.FindProject(p.ID)
			newTunnels := a.store.NewTunnelsFor(&incoming, p.ID)
			c := projectChoice{Project: p}
			switch {
			case !exists:
				c.Note = fmt.Sprintf("new, %d tunnel(s)", newTunnels)
				c.Checked = true
			case newTunnels > 0:
				c.Note = fmt.Sprintf("already here, adds %d new tunnel(s)", newTunnels)
				c.Checked = true
			default:
				c.Note = "already here"
				c.Disabled = true
			}
			choices = append(choices, c)
		}

		pickable := 0
		var only []string
		for _, c := range choices {
			if !c.Disabled {
				pickable++
				only = append(only, c.Project.ID)
			}
		}
		switch pickable {
		case 0:
			dialog.ShowInformation("Nothing New", "Everything in that file is already here.", a.win)
		case 1:
			a.applyImport(incoming.Subset(only)) // nothing to choose between
		default:
			a.pickProjects("Import Projects", "Import", choices, func(ids []string) {
				a.applyImport(incoming.Subset(ids))
			})
		}
	}, a.win)

	open.Show()
}

func (a *App) applyImport(incoming *Store) {
	projects, tunnels := a.store.Merge(incoming)
	if projects == 0 && tunnels == 0 {
		dialog.ShowInformation("Nothing New", "Everything in that file is already here.", a.win)
		return
	}

	if err := a.store.Save(); err != nil {
		dialog.ShowError(err, a.win)
		return
	}

	// Jump to the first imported project so its tunnels are right there.
	a.projectID = incoming.Projects[0].ID
	a.selected = map[string]bool{}
	a.projectSelect.SetOptions(projectNames(a.store))
	if p, ok := a.currentProject(); ok {
		a.projectSelect.SetSelected(p.Name)
	}
	a.refreshRows()
	a.refreshKeyLabel()

	dialog.ShowInformation("Imported",
		fmt.Sprintf("Added %d project(s) and %d tunnel(s).\nPress Start and enter your own password to use them.",
			projects, tunnels), a.win)
}

// projectChoice is one row of the export/import project picker.
type projectChoice struct {
	Project  Project
	Note     string // shown after the name, e.g. "3 tunnel(s)"
	Checked  bool
	Disabled bool
}

// pickProjects shows a checklist of projects with a "Select all" toggle and
// calls fn with the ticked project IDs. Confirming with nothing ticked is
// refused rather than producing an empty file / no-op import.
func (a *App) pickProjects(title, confirm string, choices []projectChoice, fn func(ids []string)) {
	checks := make([]*widget.Check, len(choices))
	rows := container.NewVBox()
	for i, c := range choices {
		checks[i] = widget.NewCheck(fmt.Sprintf("%s  —  %s", c.Project.Name, c.Note), nil)
		checks[i].SetChecked(c.Checked)
		if c.Disabled {
			checks[i].Disable()
		}
		rows.Add(checks[i])
	}

	selectAll := widget.NewCheck("Select all", func(on bool) {
		for _, ch := range checks {
			if !ch.Disabled() {
				ch.SetChecked(on)
			}
		}
	})

	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(420, min(float32(len(choices))*40, 280)))
	content := container.NewBorder(
		container.NewVBox(selectAll, widget.NewSeparator()), nil, nil, nil, scroll)

	dialog.ShowCustomConfirm(title, confirm, "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		var ids []string
		for i, ch := range checks {
			if ch.Checked && !ch.Disabled() {
				ids = append(ids, choices[i].Project.ID)
			}
		}
		if len(ids) == 0 {
			dialog.ShowInformation("Nothing Selected", "Tick at least one project.", a.win)
			return
		}
		fn(ids)
	}, a.win)
}

// fileSlug turns a project name into something safe for a file name.
func fileSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		return "project"
	}
	return slug
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
