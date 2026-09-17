// Command sshtunnelapp ("stp-tunnel") is a small desktop GUI for managing
// several SSH local port-forwards ("tunnels") grouped by project (bastion
// server). See README-style notes throughout — this file wires the UI;
// model.go holds persisted state, tunnel.go runs the ssh subprocesses, and
// sshkey.go manages the one shared identity key.
package main

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/ssh"
)

// column widths (px) shared between the manual header row and the data rows,
// so they stay visually aligned.
var colWidths = []float32{46, 90, 70, 200, 90, 170, 90, 170, 80}

const rowHeight = 34

type App struct {
	fyneApp fyne.App
	win     fyne.Window

	store   *Store
	runtime *TunnelRuntime

	projectID string
	selected  map[string]bool // tunnel ID -> checked
	lastError map[string]string

	// passwords entered this session, keyed by project ID. Deliberately
	// never written to disk — they live only as long as the app runs.
	sessionPasswords map[string]string

	list          *widget.List
	projectSelect *widget.Select
	keyLabel      *widget.Label
	selectedLabel *widget.Label
	logEntry      *widget.Entry
	emptyState    *widget.Label
	logScroll     *container.Scroll

	btnDeleteSelected *widget.Button
	btnStartSelected  *widget.Button
	btnStopSelected   *widget.Button
}

func main() {
	fyneApp := app.New()
	fyneApp.Settings().SetTheme(&stpTheme{})
	fyneApp.SetIcon(appIcon) // Dock/Cmd+Tab icon when run outside the .app bundle

	win := fyneApp.NewWindow("STP Tunnel")
	win.Resize(fyne.NewSize(980, 640))

	store, err := LoadStore()
	if err != nil {
		store = &Store{}
	}

	a := &App{
		fyneApp:          fyneApp,
		win:              win,
		store:            store,
		selected:         map[string]bool{},
		lastError:        map[string]string{},
		sessionPasswords: map[string]string{},
	}
	a.runtime = NewTunnelRuntime(a.onTunnelExit, a.onTunnelLog)

	if len(store.Projects) > 0 {
		a.projectID = store.Projects[0].ID
	}

	win.SetMainMenu(a.buildMainMenu())
	win.SetContent(a.buildUI())
	a.setupSystemTray()
	a.checkForUpdatesQuietly()
	a.startLatencyRefresh()
	win.SetCloseIntercept(a.confirmClose)
	win.ShowAndRun()
}

// confirmClose runs instead of the window actually closing (see
// win.SetCloseIntercept above). With nothing running there's nothing to
// warn about, so it quits straight away; otherwise it asks first, since
// quitting stops every tunnel at once with no way back.
func (a *App) confirmClose() {
	running := 0
	for _, t := range a.store.Tunnels {
		if a.runtime.IsRunning(t.ID) {
			running++
		}
	}
	if running == 0 {
		a.quit()
		return
	}

	dialog.ShowConfirm("Quit STP Tunnel?",
		fmt.Sprintf("%d tunnel(s) are still running. Quitting will stop them all.", running),
		func(ok bool) {
			if ok {
				a.quit()
			}
		}, a.win)
}

// onTunnelExit runs on a background goroutine (see tunnel.go) whenever a
// tunnel's ssh process dies on its own. fyne.Do marshals the UI update back
// onto Fyne's own goroutine — required whenever fyne widgets are touched
// from outside a widget callback.
func (a *App) onTunnelExit(tunnelID string, err error) {
	fyne.Do(func() {
		if err != nil {
			a.lastError[tunnelID] = err.Error()
			a.appendLog(fmt.Sprintf("[%s] ssh exited: %s", a.tunnelName(tunnelID), err))
		}
		a.refreshRows()
	})
}

// onTunnelLog runs on a background goroutine (see tunnel.go) for every line
// of an ssh subprocess's stderr — connection errors, host key prompts, etc.
func (a *App) onTunnelLog(tunnelID, line string) {
	name := a.tunnelName(tunnelID)
	fyne.Do(func() {
		a.appendLog(fmt.Sprintf("[%s] %s", name, line))
	})
}

func (a *App) tunnelName(id string) string {
	for _, t := range a.store.Tunnels {
		if t.ID == id {
			return t.ServiceName
		}
	}
	return id
}

// appendLog must only be called from Fyne's own goroutine (wrap with
// fyne.Do from anywhere else).
func (a *App) appendLog(line string) {
	if a.logEntry == nil {
		return
	}
	text := a.logEntry.Text
	if text != "" {
		text += "\n"
	}
	a.logEntry.SetText(text + line)
	if a.logScroll != nil {
		a.logScroll.ScrollToBottom()
	}
}

func (a *App) currentTunnels() []TunnelConfig {
	return a.store.TunnelsForProject(a.projectID)
}

func (a *App) currentProject() (Project, bool) {
	return a.store.FindProject(a.projectID)
}

// ---------- top-level layout ----------

func (a *App) buildUI() fyne.CanvasObject {
	title := canvas.NewText("STP Tunnel", colForeground)
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}

	header := container.NewBorder(nil, nil, title, a.buildHeaderMenuButton())

	projectBar := a.buildProjectRow()
	connLine := a.buildConnectionLine()
	toolbar := a.buildToolbar()
	rows := a.buildTable()
	logSection := a.buildLogSection()

	top := container.NewVBox(
		header,
		widget.NewSeparator(),
		projectBar,
		connLine,
		widget.NewSeparator(),
		toolbar,
	)

	bottom := container.NewVBox(logSection, a.buildFooter())
	return container.NewPadded(container.NewBorder(top, bottom, nil, nil, rows))
}

// buildConnectionLine shows, in one muted line, exactly what the selected
// project will connect as — the thing users need to sanity-check before
// pressing Start.
func (a *App) buildConnectionLine() fyne.CanvasObject {
	a.keyLabel = widget.NewLabel("")
	a.keyLabel.TextStyle = fyne.TextStyle{Monospace: true}
	a.keyLabel.Importance = widget.LowImportance
	a.refreshKeyLabel()
	return a.keyLabel
}

// buildFooter keeps the version and credit on screen at all times — a
// teammate reporting a problem can read it off without opening a menu.
func (a *App) buildFooter() fyne.CanvasObject {
	left := canvas.NewText(fmt.Sprintf("STP Tunnel %s", versionString()), colMuted)
	left.TextSize = 11
	left.TextStyle = fyne.TextStyle{Monospace: true}

	right := canvas.NewText(copyright, colMuted)
	right.TextSize = 11

	return container.NewBorder(nil, nil, left, right)
}

// buildHeaderMenuButton is a kebab-menu icon at the top-right of the window,
// mirroring Help → Copy diagnostics / Check for updates from the OS menu bar
// — that menu bar is easy to never notice, this keeps both one click away
// without cluttering the main tunnel table with extra buttons.
func (a *App) buildHeaderMenuButton() fyne.CanvasObject {
	var btn *widget.Button
	btn = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Copy diagnostics", a.doCopyDiagnostics),
			fyne.NewMenuItem("Check for updates", a.doCheckForUpdates),
		)
		popup := widget.NewPopUpMenu(menu, a.win.Canvas())
		popup.ShowAtRelativePosition(fyne.NewPos(0, btn.Size().Height), btn)
	})
	btn.Importance = widget.LowImportance
	return btn
}

func (a *App) buildLogSection() fyne.CanvasObject {
	a.logEntry = widget.NewMultiLineEntry()
	a.logEntry.Wrapping = fyne.TextWrapWord
	a.logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	a.logEntry.Disable() // read-only — this is a log viewer, not an input

	a.logScroll = container.NewVScroll(a.logEntry)
	a.logScroll.SetMinSize(fyne.NewSize(0, 150))

	label := widget.NewLabelWithStyle("LOG", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	label.Importance = widget.LowImportance

	clearBtn := widget.NewButton("Clear", func() {
		a.logEntry.SetText("")
	})
	clearBtn.Importance = widget.LowImportance

	head := container.NewBorder(nil, nil, label, clearBtn)
	return container.NewBorder(container.NewVBox(widget.NewSeparator(), head), nil, nil, nil, a.logScroll)
}

func (a *App) buildProjectRow() fyne.CanvasObject {
	a.projectSelect = widget.NewSelect(projectNames(a.store), func(name string) {
		for _, p := range a.store.Projects {
			if p.Name == name {
				a.projectID = p.ID
				a.selected = map[string]bool{}
				a.refreshRows()
				a.refreshKeyLabel()
			}
		}
	})
	a.projectSelect.PlaceHolder = "No project yet"
	if p, ok := a.currentProject(); ok {
		a.projectSelect.SetSelected(p.Name)
	}

	newBtn := widget.NewButton("New", a.showNewProjectDialog)
	editBtn := widget.NewButton("Edit", a.showEditProjectDialog)
	delBtn := widget.NewButton("Delete", a.showDeleteProjectDialog)
	for _, b := range []*widget.Button{newBtn, editBtn, delBtn} {
		b.Importance = widget.LowImportance
	}

	label := widget.NewLabel("Project")
	label.Importance = widget.LowImportance

	return container.NewHBox(label, a.projectSelect, newBtn, editBtn, delBtn)
}

func (a *App) buildToolbar() fyne.CanvasObject {
	addBtn := widget.NewButtonWithIcon("Add tunnel", theme.ContentAddIcon(), a.showAddTunnelDialog)
	addBtn.Importance = widget.HighImportance

	a.btnStartSelected = widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), a.doStartSelected)
	a.btnStopSelected = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), a.doStopSelected)
	stopAllBtn := widget.NewButton("Stop all", a.doStopAll)
	a.btnDeleteSelected = widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), a.doDeleteSelected)
	stopAllBtn.Importance = widget.LowImportance
	a.btnDeleteSelected.Importance = widget.LowImportance

	a.selectedLabel = widget.NewLabel("Selected: 0")
	a.selectedLabel.Importance = widget.LowImportance

	return container.NewHBox(
		addBtn,
		widget.NewSeparator(),
		a.btnStartSelected, a.btnStopSelected, stopAllBtn,
		widget.NewSeparator(),
		a.btnDeleteSelected,
		layout.NewSpacer(),
		a.selectedLabel,
	)
}

// ---------- tunnel rows ----------
//
// This is a widget.List, not a widget.Table, on purpose: Fyne's Table
// intercepts taps for its own cell selection, so interactive widgets inside
// table cells (checkboxes, links, buttons) never receive clicks. List passes
// events through to its item widgets normally.

func (a *App) buildTable() fyne.CanvasObject {
	headerRow := a.buildHeaderRow()

	a.list = widget.NewList(
		func() int { return len(a.currentTunnels()) },
		a.newRowTemplate,
		a.updateRow,
	)

	a.emptyState = widget.NewLabelWithStyle(
		"No tunnels in this project yet \u2014 press \u201cAdd tunnel\u201d to create one.",
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true},
	)
	a.emptyState.Importance = widget.LowImportance

	body := container.NewStack(a.list, container.NewCenter(a.emptyState))
	a.refreshEmptyState()

	return container.NewBorder(headerRow, nil, nil, nil, body)
}

// refreshRows redraws the tunnel rows and everything that depends on them.
func (a *App) refreshRows() {
	if a.list == nil {
		return
	}
	a.list.Refresh()
	a.refreshEmptyState()
	a.refreshSelectedCount()
}

// startLatencyRefresh redraws the tunnel rows on a fixed interval for the
// life of the app, so the Latency column keeps up with the heartbeats that
// TunnelRuntime measures in the background (see tunnel.go's pingLoop) even
// when nothing else about a tunnel has changed.
func (a *App) startLatencyRefresh() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(a.refreshRows)
		}
	}()
}

func (a *App) refreshEmptyState() {
	if a.emptyState == nil {
		return
	}
	a.emptyState.Hidden = len(a.currentTunnels()) != 0
	a.emptyState.Refresh()
}

func (a *App) buildHeaderRow() fyne.CanvasObject {
	selectAll := widget.NewCheck("", func(checked bool) {
		for _, t := range a.currentTunnels() {
			a.selected[t.ID] = checked
		}
		a.refreshRows()
	})

	labels := []string{"", "Status", "Latency", "Service name", "Local port", "Target host", "Target port", "URL", ""}
	row := container.NewHBox()
	for i, text := range labels {
		var cell fyne.CanvasObject
		if i == 0 {
			cell = selectAll
		} else {
			lbl := widget.NewLabelWithStyle(strings.ToUpper(text), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			lbl.Importance = widget.LowImportance
			cell = lbl
		}
		row.Add(container.NewGridWrap(fyne.NewSize(colWidths[i], 28), cell))
	}
	return container.NewVBox(row, widget.NewSeparator())
}

func (a *App) newRowTemplate() fyne.CanvasObject {
	mono := fyne.TextStyle{Monospace: true}

	cells := []fyne.CanvasObject{
		widget.NewCheck("", nil),
		newStatusBadge(),
		widget.NewLabelWithStyle("", fyne.TextAlignTrailing, mono),
		widget.NewLabel(""),
		widget.NewLabelWithStyle("", fyne.TextAlignTrailing, mono),
		widget.NewLabelWithStyle("", fyne.TextAlignLeading, mono),
		widget.NewLabelWithStyle("", fyne.TextAlignTrailing, mono),
		widget.NewHyperlink("", nil),
		widget.NewButton("Edit", nil),
	}

	row := container.NewHBox()
	for i, c := range cells {
		row.Add(container.NewGridWrap(fyne.NewSize(colWidths[i], rowHeight), c))
	}
	return row
}

// newStatusBadge draws a rounded pill the row's status colour fills in.
func newStatusBadge() fyne.CanvasObject {
	bg := canvas.NewRectangle(colPill)
	bg.CornerRadius = 9

	text := canvas.NewText("", colMuted)
	text.TextSize = 10.5
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter

	pill := container.NewStack(bg, container.NewCenter(text))
	return container.NewPadded(pill)
}

func setStatusBadge(o fyne.CanvasObject, label string, fg color.Color) {
	pill := o.(*fyne.Container).Objects[0].(*fyne.Container)
	text := pill.Objects[1].(*fyne.Container).Objects[0].(*canvas.Text)

	text.Text = label
	text.Color = fg
	text.Refresh()
}

// rowCell digs the widget out of the i-th fixed-width wrapper in a row.
func rowCell(o fyne.CanvasObject, i int) fyne.CanvasObject {
	return o.(*fyne.Container).Objects[i].(*fyne.Container).Objects[0]
}

func (a *App) updateRow(i widget.ListItemID, o fyne.CanvasObject) {
	tunnels := a.currentTunnels()
	if i >= len(tunnels) {
		return
	}
	t := tunnels[i]

	check := rowCell(o, 0).(*widget.Check)
	check.OnChanged = nil // avoid firing the handler while syncing state below
	check.SetChecked(a.selected[t.ID])
	check.OnChanged = func(checked bool) {
		a.selected[t.ID] = checked
		a.refreshSelectedCount()
	}

	statusText, statusColor := a.tunnelStatus(t)
	setStatusBadge(rowCell(o, 1), statusText, statusColor)
	setLatencyLabel(rowCell(o, 2).(*widget.Label), a.runtime.Health(t.ID))

	rowCell(o, 3).(*widget.Label).SetText(t.ServiceName)
	rowCell(o, 4).(*widget.Label).SetText(t.LocalPort)
	rowCell(o, 5).(*widget.Label).SetText(t.TargetHost)
	rowCell(o, 6).(*widget.Label).SetText(t.TargetPort)

	link := rowCell(o, 7).(*widget.Hyperlink)
	link.SetText(fmt.Sprintf("localhost:%s", t.LocalPort))
	if u, err := url.Parse(fmt.Sprintf("http://localhost:%s", t.LocalPort)); err == nil {
		link.SetURL(u)
	}

	editBtn := rowCell(o, 8).(*widget.Button)
	editBtn.OnTapped = func() { a.showEditTunnelDialog(t) }
}

// setLatencyLabel shows the tunnel's most recent SSH heartbeat round-trip
// time — blank while stopped, "…" for the brief window before the first
// heartbeat lands, and "timeout" (in the danger colour) if the last one
// failed, which usually means the connection is about to be reported dead.
func setLatencyLabel(lbl *widget.Label, health TunnelHealth) {
	switch {
	case !health.Running:
		lbl.Importance = widget.LowImportance
		lbl.SetText("")
	case health.Err != nil:
		lbl.Importance = widget.DangerImportance
		lbl.SetText("timeout")
	case !health.Pinged:
		lbl.Importance = widget.LowImportance
		lbl.SetText("…")
	default:
		lbl.Importance = widget.LowImportance
		lbl.SetText(fmt.Sprintf("%dms", health.Latency.Milliseconds()))
	}
}

func (a *App) tunnelStatus(t TunnelConfig) (label string, fg color.Color) {
	if a.runtime.IsRunning(t.ID) {
		return "RUNNING", colSuccess
	}
	if msg := a.lastError[t.ID]; msg != "" {
		return "ERROR", colDanger
	}
	return "STOPPED", colMuted
}

func (a *App) refreshSelectedCount() {
	if a.selectedLabel == nil {
		return // toolbar not built yet — happens once, from buildProjectRow's initial SetSelected
	}
	n := 0
	for _, v := range a.selected {
		if v {
			n++
		}
	}
	a.selectedLabel.SetText(fmt.Sprintf("Selected: %d", n))

	// Start/Stop/Delete act on the checked rows, so they're meaningless
	// (and previously just popped a "Nothing Selected" dialog) with none
	// checked. Stop all is deliberately left enabled — it ignores the
	// checkboxes and stops every running tunnel in the project.
	if n == 0 {
		a.btnStartSelected.Disable()
		a.btnStopSelected.Disable()
		a.btnDeleteSelected.Disable()
	} else {
		a.btnStartSelected.Enable()
		a.btnStopSelected.Enable()
		a.btnDeleteSelected.Enable()
	}
}

func (a *App) refreshKeyLabel() {
	if a.keyLabel == nil {
		return
	}

	proj, ok := a.currentProject()
	if !ok {
		a.keyLabel.SetText("No project selected")
		return
	}

	auth := "password (asked when you press Start)"
	if proj.IdentityFile != "" {
		auth = "key: " + proj.IdentityFile
	}

	a.keyLabel.SetText(fmt.Sprintf("%s@%s:%s  ·  %s",
		proj.BastionUser, proj.BastionHost, portOrDefault(proj.BastionPort), auth))
}

// ---------- bulk actions ----------

func (a *App) selectedTunnels() []TunnelConfig {
	var out []TunnelConfig
	for _, t := range a.currentTunnels() {
		if a.selected[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) doStartSelected() {
	sel := a.selectedTunnels()
	if len(sel) == 0 {
		dialog.ShowInformation("Nothing Selected", "Tick a tunnel first, then press Start.", a.win)
		return
	}
	proj, ok := a.currentProject()
	if !ok {
		dialog.ShowError(fmt.Errorf("select or create a project first"), a.win)
		return
	}
	if proj.BastionHost == "" || proj.BastionUser == "" {
		dialog.ShowError(fmt.Errorf("project %q is missing a host/user — press Edit to fill it in", proj.Name), a.win)
		return
	}

	a.withAuth(proj, func(auth []ssh.AuthMethod) {
		for _, t := range sel {
			if a.runtime.IsRunning(t.ID) {
				continue
			}
			delete(a.lastError, t.ID)

			// Dialling can take seconds (or time out), so never do it on the
			// UI goroutine — the window would freeze until it finished.
			go func(t TunnelConfig) {
				err := a.runtime.Start(t, proj, auth)
				fyne.Do(func() {
					if err != nil {
						a.lastError[t.ID] = err.Error()
						a.appendLog(fmt.Sprintf("[%s] %s", t.ServiceName, err))
					}
					a.refreshRows()
				})
			}(t)
		}
		a.refreshRows()
	})
}

// withAuth resolves how to authenticate to a project's server and hands the
// result to fn. Order: a configured key file, then a password saved in the OS
// keychain, then one already entered this session, and only failing all of
// those does it ask the user.
func (a *App) withAuth(proj Project, fn func([]ssh.AuthMethod)) {
	if keyFile := expandHome(proj.IdentityFile); keyFile != "" {
		auth, err := KeyAuth(keyFile, "")
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		fn(auth)
		return
	}

	if pw, ok := a.sessionPasswords[proj.ID]; ok {
		fn(PasswordAuth(pw))
		return
	}

	if pw, err := loadPassword(proj.ID); err == nil && pw != "" {
		a.sessionPasswords[proj.ID] = pw // avoid asking the OS again this session
		fn(PasswordAuth(pw))
		return
	}

	a.promptPassword(proj, fn)
}

func (a *App) promptPassword(proj Project, fn func([]ssh.AuthMethod)) {
	pwEntry := widget.NewPasswordEntry()
	pwEntry.SetPlaceHolder("SSH password")

	remember := widget.NewCheck("Remember this password", nil)
	remember.SetChecked(true)

	hint := widget.NewLabel("Saved in your system keychain, not in any file.")
	hint.Importance = widget.LowImportance

	form := dialog.NewForm(
		fmt.Sprintf("Password for %s@%s", proj.BastionUser, proj.BastionHost),
		"Connect", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Password", pwEntry),
			widget.NewFormItem("", remember),
			widget.NewFormItem("", hint),
		},
		func(ok bool) {
			if !ok || pwEntry.Text == "" {
				return
			}

			a.sessionPasswords[proj.ID] = pwEntry.Text
			if remember.Checked {
				if err := savePassword(proj.ID, pwEntry.Text); err != nil {
					// Not fatal: the tunnel can still start, we just won't
					// remember the password next time.
					a.appendLog(fmt.Sprintf("could not save password to the keychain: %v", err))
				}
			}
			fn(PasswordAuth(pwEntry.Text))
		},
		a.win,
	)
	form.Resize(fyne.NewSize(420, form.MinSize().Height))
	form.Show()
}

func (a *App) doStopSelected() {
	sel := a.selectedTunnels()
	if len(sel) == 0 {
		dialog.ShowInformation("Nothing Selected", "Check the checkbox next to a tunnel first, then click Stop Selected.", a.win)
		return
	}
	for _, t := range sel {
		_ = a.runtime.Stop(t.ID)
	}
	a.refreshRows()
}

func (a *App) doStopAll() {
	a.runtime.StopAll()
	a.refreshRows()
}

func (a *App) doDeleteSelected() {
	sel := a.selectedTunnels()
	if len(sel) == 0 {
		dialog.ShowInformation("Nothing Selected", "Check the checkbox next to a tunnel first, then click Delete Selected.", a.win)
		return
	}
	dialog.ShowConfirm("Delete Selected", fmt.Sprintf("Delete %d tunnel(s)? This cannot be undone.", len(sel)), func(ok bool) {
		if !ok {
			return
		}
		for _, t := range sel {
			_ = a.runtime.Stop(t.ID)
			delete(a.selected, t.ID)
		}
		var remaining []TunnelConfig
		for _, t := range a.store.Tunnels {
			keep := true
			for _, d := range sel {
				if d.ID == t.ID {
					keep = false
					break
				}
			}
			if keep {
				remaining = append(remaining, t)
			}
		}
		a.store.Tunnels = remaining
		_ = a.store.Save()
		a.refreshRows()
		a.refreshSelectedCount()
	}, a.win)
}

// expandHome turns a leading "~" into the user's home directory — ssh
// understands "~/.ssh/id_rsa" but exec.Command passes it through literally.
func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// ---------- project dialogs ----------

func (a *App) showNewProjectDialog() {
	a.showProjectForm(Project{ID: newID(), BastionPort: "22"}, true)
}

func (a *App) showEditProjectDialog() {
	p, ok := a.currentProject()
	if !ok {
		dialog.ShowError(fmt.Errorf("no project selected"), a.win)
		return
	}
	a.showProjectForm(p, false)
}

func (a *App) showProjectForm(p Project, isNew bool) {
	name := widget.NewEntry()
	name.SetText(p.Name)
	host := widget.NewEntry()
	host.SetText(p.BastionHost)
	user := widget.NewEntry()
	user.SetText(p.BastionUser)
	port := widget.NewEntry()
	port.SetText(portOrDefault(p.BastionPort))

	identity := widget.NewEntry()
	identity.SetText(p.IdentityFile)
	identity.SetPlaceHolder("blank = log in with a password instead")
	browse := widget.NewButton("Browse...", func() {
		dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			identity.SetText(uc.URI().Path())
		}, a.win).Show()
	})
	identityRow := container.NewBorder(nil, nil, nil, browse, identity)

	skipHostKey := widget.NewCheck("Skip host key check (for servers that get rebuilt often)", nil)
	skipHostKey.SetChecked(p.SkipHostKeyCheck)

	passwordRow := widget.NewLabel("Asked when you press Start")
	passwordRow.Importance = widget.LowImportance
	forgetBtn := widget.NewButton("Forget saved password", func() {
		forgetPassword(p.ID)
		delete(a.sessionPasswords, p.ID)
		passwordRow.SetText("Forgotten — you'll be asked again")
	})
	if !hasSavedPassword(p.ID) {
		forgetBtn.Disable()
	} else {
		passwordRow.SetText("Saved in your system keychain")
	}

	form := dialog.NewForm(
		map[bool]string{true: "New Project", false: "Edit Project"}[isNew],
		"Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", name),
			widget.NewFormItem("Bastion host", host),
			widget.NewFormItem("Bastion user", user),
			widget.NewFormItem("Bastion port", port),
			widget.NewFormItem("Identity file", identityRow),
			widget.NewFormItem("", skipHostKey),
			widget.NewFormItem("Password", container.NewBorder(nil, nil, nil, forgetBtn, passwordRow)),
		},
		func(ok bool) {
			if !ok {
				return
			}
			if name.Text == "" || host.Text == "" || user.Text == "" {
				dialog.ShowError(fmt.Errorf("name, host, and user are required"), a.win)
				return
			}
			p.Name = name.Text
			p.BastionHost = host.Text
			p.BastionUser = user.Text
			p.BastionPort = port.Text
			p.IdentityFile = identity.Text
			p.SkipHostKeyCheck = skipHostKey.Checked

			if isNew {
				a.store.Projects = append(a.store.Projects, p)
				a.projectID = p.ID
			} else {
				for i := range a.store.Projects {
					if a.store.Projects[i].ID == p.ID {
						a.store.Projects[i] = p
					}
				}
			}
			_ = a.store.Save()
			a.projectSelect.SetOptions(projectNames(a.store))
			a.projectSelect.SetSelected(p.Name)
			a.refreshRows()
		},
		a.win,
	)
	form.Resize(fyne.NewSize(420, form.MinSize().Height))
	form.Show()
}

func (a *App) showDeleteProjectDialog() {
	p, ok := a.currentProject()
	if !ok {
		return
	}
	dialog.ShowConfirm("Delete Project", fmt.Sprintf("Delete project %q and all its tunnels? This cannot be undone.", p.Name), func(confirm bool) {
		if !confirm {
			return
		}
		for _, t := range a.store.TunnelsForProject(p.ID) {
			_ = a.runtime.Stop(t.ID)
		}
		forgetPassword(p.ID)
		delete(a.sessionPasswords, p.ID)
		var keepProjects []Project
		for _, pr := range a.store.Projects {
			if pr.ID != p.ID {
				keepProjects = append(keepProjects, pr)
			}
		}
		var keepTunnels []TunnelConfig
		for _, t := range a.store.Tunnels {
			if t.ProjectID != p.ID {
				keepTunnels = append(keepTunnels, t)
			}
		}
		a.store.Projects = keepProjects
		a.store.Tunnels = keepTunnels
		_ = a.store.Save()

		a.projectID = ""
		if len(a.store.Projects) > 0 {
			a.projectID = a.store.Projects[0].ID
		}
		a.projectSelect.SetOptions(projectNames(a.store))
		if pr, ok := a.currentProject(); ok {
			a.projectSelect.SetSelected(pr.Name)
		} else {
			a.projectSelect.ClearSelected()
		}
		a.refreshRows()
	}, a.win)
}

func projectNames(s *Store) []string {
	var out []string
	for _, p := range s.Projects {
		out = append(out, p.Name)
	}
	return out
}

// ---------- tunnel dialogs ----------

func (a *App) showAddTunnelDialog() {
	if a.projectID == "" {
		dialog.ShowError(fmt.Errorf("create a project first"), a.win)
		return
	}
	a.showTunnelForm(TunnelConfig{ID: newID(), ProjectID: a.projectID}, true)
}

func (a *App) showEditTunnelDialog(t TunnelConfig) {
	a.showTunnelForm(t, false)
}

func (a *App) showTunnelForm(t TunnelConfig, isNew bool) {
	name := widget.NewEntry()
	name.SetText(t.ServiceName)
	localPort := widget.NewEntry()
	localPort.SetText(t.LocalPort)
	targetHost := widget.NewEntry()
	targetHost.SetText(t.TargetHost)
	if targetHost.Text == "" {
		targetHost.SetText("localhost")
	}
	targetPort := widget.NewEntry()
	targetPort.SetText(t.TargetPort)

	form := dialog.NewForm(
		map[bool]string{true: "Add Tunnel", false: "Edit Tunnel"}[isNew],
		"Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Service name", name),
			widget.NewFormItem("Local port", localPort),
			widget.NewFormItem("Target host", targetHost),
			widget.NewFormItem("Target port", targetPort),
		},
		func(ok bool) {
			if !ok {
				return
			}
			if _, err := strconv.Atoi(localPort.Text); err != nil {
				dialog.ShowError(fmt.Errorf("local port must be a number"), a.win)
				return
			}
			if _, err := strconv.Atoi(targetPort.Text); err != nil {
				dialog.ShowError(fmt.Errorf("target port must be a number"), a.win)
				return
			}

			// The live tunnel (if any) was started with the old values baked
			// in — traffic keeps flowing to them until it's restarted, so
			// warn rather than let the table quietly show settings that
			// aren't actually in effect yet.
			liveChanged := !isNew && a.runtime.IsRunning(t.ID) &&
				(t.LocalPort != localPort.Text || t.TargetHost != targetHost.Text || t.TargetPort != targetPort.Text)

			t.ServiceName = name.Text
			t.LocalPort = localPort.Text
			t.TargetHost = targetHost.Text
			t.TargetPort = targetPort.Text

			if isNew {
				a.store.Tunnels = append(a.store.Tunnels, t)
			} else {
				for i := range a.store.Tunnels {
					if a.store.Tunnels[i].ID == t.ID {
						a.store.Tunnels[i] = t
					}
				}
			}
			_ = a.store.Save()
			a.refreshRows()

			if liveChanged {
				dialog.ShowInformation("Restart needed",
					fmt.Sprintf("%q is still running with the old settings. Stop and Start it again to use the new ones.", t.ServiceName),
					a.win)
			}
		},
		a.win,
	)
	form.Resize(fyne.NewSize(380, form.MinSize().Height))
	form.Show()
}

// parseURL is a small wrapper so callers don't each import net/url.
func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
