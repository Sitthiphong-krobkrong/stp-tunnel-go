//go:build screenshots

// Regenerates the step-by-step screenshots used by the Getting started guide
// (guide.go embeds docs/guide/*.png). Each shot is the real UI — buildUI()
// and the real dialogs — rendered off-screen with made-up sample data (no
// real hosts), plus a red box around the control that step is about.
//
// Run after any UI change:
//
//	go test -tags screenshots -run TestGuideScreenshots .
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

const (
	shotDir   = "docs/guide"
	shotScale = 2 // crisp on Retina screens
)

var (
	shotSize      = fyne.NewSize(1000, 600)
	highlightRed  = color.NRGBA{R: 0xff, G: 0x4d, B: 0x4f, A: 0xff}
	demoProject   = Project{ID: "demo", Name: "Demo UAT", BastionHost: "bastion.example.com", BastionUser: "demo"}
	demoProject2  = Project{ID: "demo2", Name: "Demo Prod", BastionHost: "bastion-prod.example.com", BastionUser: "demo"}
	demoPostgres  = TunnelConfig{ID: "t1", ProjectID: "demo", ServiceName: "PostgreSQL", LocalPort: "5432", TargetHost: "10.0.0.20", TargetPort: "5432"}
	demoCamunda   = TunnelConfig{ID: "t2", ProjectID: "demo", ServiceName: "Camunda", LocalPort: "8080", TargetHost: "10.0.0.30", TargetPort: "8080"}
	demoProdRedis = TunnelConfig{ID: "t3", ProjectID: "demo2", ServiceName: "Redis", LocalPort: "6379", TargetHost: "10.1.0.40", TargetPort: "6379"}
)

func TestGuideScreenshots(t *testing.T) {
	if err := os.MkdirAll(shotDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Empty app → New
	a, w := newShotApp(t, &Store{})
	shoot(t, w, "01-new-project", findButton(w, "New"))

	// 2. New Project form, filled in and tested
	a, w = newShotApp(t, &Store{})
	a.showNewProjectDialog()
	setEntries(w, "Demo UAT", "bastion.example.com", "demo")
	if l := findLabel(w, "Not tested yet"); l != nil {
		l.Importance = widget.SuccessImportance
		l.SetText("Connected to demo@bastion.example.com successfully.")
	}
	w.Canvas().Overlays().Top().Refresh()
	shoot(t, w, "02-project-form", findButton(w, "Test"))

	// 3. Project, no tunnels yet → Add tunnel
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject}})
	shoot(t, w, "03-add-tunnel", findButton(w, "Add tunnel"))

	// 4. Add Tunnel form
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject}})
	a.showAddTunnelDialog()
	entries := setEntries(w, "PostgreSQL", "5432", "10.0.0.20", "5432")
	shoot(t, w, "04-tunnel-form", entries[1:])

	// 5. Tick a tunnel → Start
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject}, Tunnels: []TunnelConfig{demoPostgres, demoCamunda}})
	a.selected[demoPostgres.ID] = true
	a.refreshRows()
	shoot(t, w, "05-start", findCheckedRowBox(w), findButton(w, "Start"))

	// 6. First Start asks for the password
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject}, Tunnels: []TunnelConfig{demoPostgres, demoCamunda}})
	a.selected[demoPostgres.ID] = true
	a.refreshRows()
	a.promptPassword(demoProject, nil)
	setEntries(w, "secret-password")
	shoot(t, w, "06-password", findCheck(w, "Remember this password"))

	// 7. Running: status, latency and the localhost link to use
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject}, Tunnels: []TunnelConfig{demoPostgres, demoCamunda}})
	fakeRunning(a, demoPostgres.ID, 23*time.Millisecond)
	a.lastError[demoCamunda.ID] = "listen on local port 8080: address already in use"
	a.appendLog("[PostgreSQL] connecting to bastion.example.com:22 as demo")
	a.appendLog("[PostgreSQL] listening on localhost:5432 → 10.0.0.20:5432")
	a.appendLog("[Camunda] listen on local port 8080: address already in use")
	a.refreshRows()
	shoot(t, w, "07-running", findParent(w, findText(w, "RUNNING")), findHyperlink(w, "localhost:5432"))

	// 8. Share with the team: Export, pick projects
	a, w = newShotApp(t, &Store{Projects: []Project{demoProject, demoProject2}, Tunnels: []TunnelConfig{demoPostgres, demoCamunda, demoProdRedis}})
	a.doExportConfig()
	shoot(t, w, "08-export",
		[]fyne.CanvasObject{findCheck(w, "Demo UAT  —  2 tunnel(s)"), findCheck(w, "Demo Prod  —  1 tunnel(s)")},
		findButton(w, "Export..."))
}

// ---------- building the app ----------

func newShotApp(t *testing.T, store *Store) (*App, fyne.Window) {
	fa := test.NewTempApp(t)
	fa.Settings().SetTheme(&stpTheme{})

	a := &App{
		fyneApp:          fa,
		store:            store,
		selected:         map[string]bool{},
		lastError:        map[string]string{},
		sessionPasswords: map[string]string{},
	}
	a.runtime = NewTunnelRuntime(nil, nil)
	if len(store.Projects) > 0 {
		a.projectID = store.Projects[0].ID
	}

	w := test.NewTempWindow(t, nil)
	w.Canvas().(test.WindowlessCanvas).SetScale(shotScale)
	a.win = w
	w.SetContent(a.buildUI())
	w.Resize(shotSize)
	return a, w
}

// fakeRunning registers a tunnel as running with a measured latency, without
// any real SSH connection behind it.
func fakeRunning(a *App, id string, latency time.Duration) {
	a.runtime.mu.Lock()
	defer a.runtime.mu.Unlock()
	a.runtime.running[id] = &liveTunnel{pinged: true, latency: latency, done: make(chan struct{})}
}

// ---------- finding widgets in the rendered tree ----------

func walk(o fyne.CanvasObject, visit func(fyne.CanvasObject) bool) bool {
	if o == nil || !o.Visible() {
		return false
	}
	if visit(o) {
		return true
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			if walk(ch, visit) {
				return true
			}
		}
	}
	if wd, ok := o.(fyne.Widget); ok {
		for _, ch := range test.WidgetRenderer(wd).Objects() {
			if walk(ch, visit) {
				return true
			}
		}
	}
	return false
}

// find searches the top dialog first (when one is open), then the window.
func find(w fyne.Window, match func(fyne.CanvasObject) bool) fyne.CanvasObject {
	var found fyne.CanvasObject
	roots := []fyne.CanvasObject{w.Canvas().Content()}
	if top := w.Canvas().Overlays().Top(); top != nil {
		roots = []fyne.CanvasObject{top}
	}
	for _, r := range roots {
		walk(r, func(o fyne.CanvasObject) bool {
			if match(o) {
				found = o
				return true
			}
			return false
		})
	}
	return found
}

func findButton(w fyne.Window, text string) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		b, ok := o.(*widget.Button)
		return ok && b.Text == text
	})
}

func findCheck(w fyne.Window, text string) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		c, ok := o.(*widget.Check)
		return ok && c.Text == text
	})
}

func findCheckedRowBox(w fyne.Window) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		c, ok := o.(*widget.Check)
		return ok && c.Text == "" && c.Checked
	})
}

func findLabel(w fyne.Window, text string) *widget.Label {
	l, _ := find(w, func(o fyne.CanvasObject) bool {
		l, ok := o.(*widget.Label)
		return ok && l.Text == text
	}).(*widget.Label)
	return l
}

func findText(w fyne.Window, text string) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		t, ok := o.(*canvas.Text)
		return ok && t.Text == text
	})
}

// findParent returns the container holding target — e.g. the status pill
// around its RUNNING text.
func findParent(w fyne.Window, target fyne.CanvasObject) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		c, ok := o.(*fyne.Container)
		if !ok {
			return false
		}
		for _, ch := range c.Objects {
			if ch == target {
				return true
			}
		}
		return false
	})
}

func findHyperlink(w fyne.Window, text string) fyne.CanvasObject {
	return find(w, func(o fyne.CanvasObject) bool {
		h, ok := o.(*widget.Hyperlink)
		return ok && h.Text == text
	})
}

// setEntries types the values into the open dialog's entries, in order.
func setEntries(w fyne.Window, values ...string) []fyne.CanvasObject {
	var entries []fyne.CanvasObject
	walk(w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) bool {
		if e, ok := o.(*widget.Entry); ok && len(entries) < len(values) {
			e.SetText(values[len(entries)])
			entries = append(entries, e)
		}
		return false
	})
	return entries
}

// ---------- capture + highlight ----------

// shoot captures the window and draws a red box around each highlight. A
// highlight is one control, or a []fyne.CanvasObject boxed together as one.
func shoot(t *testing.T, w fyne.Window, name string, highlights ...any) {
	t.Helper()
	img := w.Canvas().Capture()
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Src)

	for _, h := range highlights {
		var objs []fyne.CanvasObject
		switch v := h.(type) {
		case []fyne.CanvasObject:
			objs = v
		case fyne.CanvasObject:
			objs = []fyne.CanvasObject{v}
		}
		if len(objs) == 0 {
			t.Fatalf("%s: a control to highlight was not found", name)
		}
		var box image.Rectangle
		for i, o := range objs {
			if o == nil {
				t.Fatalf("%s: a control to highlight was not found", name)
			}
			if r := boundsOf(o); i == 0 {
				box = r
			} else {
				box = box.Union(r)
			}
		}
		strokeRect(out, box, 3*shotScale, highlightRed)
	}

	// A dialog sits small in the middle of an empty backdrop; crop to it so
	// it stays readable when the guide scales the shot down.
	var final image.Image = out
	if w.Canvas().Overlays().Top() != nil {
		final = cropToContent(out, 24*shotScale)
	}

	f, err := os.Create(filepath.Join(shotDir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, final); err != nil {
		t.Fatal(err)
	}
}

// boundsOf is o's on-screen rectangle in image pixels, padded a little so
// the box sits around the control rather than on its edge.
func boundsOf(o fyne.CanvasObject) image.Rectangle {
	const pad = 5
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	size := o.Size()
	return image.Rect(
		int((pos.X-pad)*shotScale), int((pos.Y-pad)*shotScale),
		int((pos.X+size.Width+pad)*shotScale), int((pos.Y+size.Height+pad)*shotScale),
	)
}

func strokeRect(img *image.NRGBA, r image.Rectangle, width int, c color.Color) {
	src := image.NewUniform(c)
	for _, side := range []image.Rectangle{
		image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+width),
		image.Rect(r.Min.X, r.Max.Y-width, r.Max.X, r.Max.Y),
		image.Rect(r.Min.X, r.Min.Y, r.Min.X+width, r.Max.Y),
		image.Rect(r.Max.X-width, r.Min.Y, r.Max.X, r.Max.Y),
	} {
		draw.Draw(img, side.Intersect(img.Bounds()), src, image.Point{}, draw.Over)
	}
}

// cropToContent trims the uniform backdrop (the colour of the top-left pixel)
// around whatever is drawn on it, keeping margin pixels on each side.
func cropToContent(img *image.NRGBA, margin int) image.Image {
	b := img.Bounds()
	bg := img.NRGBAAt(b.Min.X, b.Min.Y)
	box := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y) != bg {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if box.Empty() {
		return img
	}
	return img.SubImage(box.Inset(-margin).Intersect(b))
}
