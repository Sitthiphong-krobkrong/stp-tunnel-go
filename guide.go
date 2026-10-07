package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Each page shows a real screenshot of the app with the control to use boxed
// in red. The shots are generated from the real UI with sample data — after
// changing the UI, regenerate them:
//
//	go test -tags screenshots -run TestGuideScreenshots .
//
//go:embed docs/guide/*.png
var guideShots embed.FS

func guideShot(name string) fyne.Resource {
	data, err := guideShots.ReadFile("docs/guide/" + name + ".png")
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(name+".png", data)
}

type guidePage struct {
	title string
	body  string
	shot  string // docs/guide/<shot>.png
}

var guidePages = []guidePage{
	{
		title: "1. สร้าง Project",
		body: "STP Tunnel เปิดทางจากเครื่องคุณไปยัง server ภายใน (database, API) ผ่าน bastion ที่คุณ SSH เข้าได้\n" +
			"เริ่มจากกด New เพื่อสร้าง project — หรือถ้าเพื่อนส่งไฟล์ setup มาให้ ใช้เมนู File > Import setup... แล้วข้ามไปข้อ 5 ได้เลย",
		shot: "01-new-project",
	},
	{
		title: "2. กรอกข้อมูล bastion แล้วกด Test",
		body: "ใส่ชื่อ project, host และ user ของ bastion (port ปกติคือ 22) กด Test เพื่อลองต่อดูก่อน " +
			"ขึ้นข้อความสีเขียวแปลว่าใช้ได้ แล้วกด Save",
		shot: "02-project-form",
	},
	{
		title: "3. เพิ่ม tunnel",
		body:  "กด Add tunnel เพื่อเพิ่ม service ที่อยากเข้าถึง เช่น database หนึ่งตัว = หนึ่ง tunnel",
		shot:  "03-add-tunnel",
	},
	{
		title: "4. ใส่ port",
		body: "Local port = port บนเครื่องคุณที่จะใช้ต่อ · Target host / Target port = ที่อยู่ของ service ใน network ภายใน " +
			"ที่ bastion มองเห็น แล้วกด Save",
		shot: "04-tunnel-form",
	},
	{
		title: "5. ติ๊กแล้วกด Start",
		body:  "ติ๊กช่องหน้า tunnel ที่จะใช้ (ติ๊กหลายตัวพร้อมกันได้) แล้วกด Start",
		shot:  "05-start",
	},
	{
		title: "6. ใส่รหัสผ่าน (ครั้งแรกครั้งเดียว)",
		body: "ใส่รหัสผ่าน SSH ของคุณ ติ๊ก Remember this password ไว้ " +
			"รหัสจะเก็บใน Keychain ของระบบ ไม่อยู่ในไฟล์ใดๆ และครั้งหน้าไม่ต้องกรอกอีก",
		shot: "06-password",
	},
	{
		title: "7. ใช้งานได้แล้ว",
		body: "สถานะ RUNNING = ใช้ได้ ต่อโปรแกรม DB / browser ไปที่ localhost:port ในช่อง URL ได้เลย\n" +
			"ถ้าขึ้น ERROR ให้ดูสาเหตุในช่อง LOG ด้านล่าง แก้แล้วกด Start ใหม่ · ปิดหน้าต่าง = หยุดทุก tunnel",
		shot: "07-running",
	},
	{
		title: "8. แชร์ให้ทีม",
		body: "เมนู File > Export setup... เลือก project ที่จะแชร์แล้วส่งไฟล์ให้เพื่อน ไฟล์ไม่มีรหัสผ่านอยู่ข้างใน\n" +
			"เปิดคู่มือนี้อีกครั้งได้ที่ Help > Getting started หรือปุ่ม ⋮ มุมขวาบน",
		shot: "08-export",
	},
}

// showGuideIfFirstRun opens the guide the first time the app is started on
// this machine. "Seen" is a marker file next to config.json rather than Fyne
// Preferences, which only persist when the app has an ID — and `go run`
// builds don't.
func (a *App) showGuideIfFirstRun() {
	marker, err := guideMarkerPath()
	if err != nil {
		return // can't remember it either way, so don't nag every launch
	}
	if _, err := os.Stat(marker); err == nil {
		return
	}
	a.showGuide()
}

func guideMarkerPath() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "guide-seen"), nil
}

func markGuideSeen() {
	if marker, err := guideMarkerPath(); err == nil {
		_ = os.WriteFile(marker, nil, 0600)
	}
}

// showGuide is a short step-by-step walkthrough (Back / Next / Skip). The
// last page offers New Project / Import when there's nothing set up yet, so
// a first-time user can go straight from reading to doing.
func (a *App) showGuide() {
	var d *dialog.CustomDialog
	page := 0

	title := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := widget.NewLabel("")
	body.Wrapping = fyne.TextWrapWord
	img := canvas.NewImageFromResource(nil)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(700, 420))
	step := widget.NewLabel("")
	step.Importance = widget.LowImportance

	skipBtn := widget.NewButton("ข้าม", func() { d.Hide() })
	skipBtn.Importance = widget.LowImportance
	backBtn := widget.NewButton("ย้อนกลับ", nil)
	nextBtn := widget.NewButton("ถัดไป", nil)
	nextBtn.Importance = widget.HighImportance

	// Shortcuts into the real actions, only on the last page of a fresh install.
	newProjectBtn := widget.NewButton("New Project", func() {
		d.Hide()
		a.showNewProjectDialog()
	})
	importBtn := widget.NewButton("Import setup...", func() {
		d.Hide()
		a.doImportConfig()
	})
	newProjectBtn.Importance = widget.HighImportance
	shortcutsHint := widget.NewLabel("ยังไม่มี project — เริ่มได้เลย:")
	shortcuts := container.NewHBox(shortcutsHint, newProjectBtn, importBtn)

	footer := container.NewHBox(skipBtn, step, layout.NewSpacer(), backBtn, nextBtn)
	content := container.NewBorder(
		container.NewVBox(title, body), container.NewVBox(shortcuts, footer), nil, nil, img)

	show := func() {
		p := guidePages[page]
		title.SetText(p.title)
		body.SetText(p.body)
		img.Resource = guideShot(p.shot)
		img.Refresh()
		step.SetText(fmt.Sprintf("%d / %d", page+1, len(guidePages)))

		last := page == len(guidePages)-1
		if page == 0 {
			backBtn.Disable()
		} else {
			backBtn.Enable()
		}
		if last {
			nextBtn.SetText("เริ่มใช้งาน")
			skipBtn.Hide()
		} else {
			nextBtn.SetText("ถัดไป")
			skipBtn.Show()
		}
		if last && len(a.store.Projects) == 0 {
			shortcuts.Show()
		} else {
			shortcuts.Hide()
		}
		content.Refresh() // re-lay out: button labels and shown/hidden rows changed size
	}
	backBtn.OnTapped = func() { page--; show() }
	nextBtn.OnTapped = func() {
		if page == len(guidePages)-1 {
			d.Hide()
			return
		}
		page++
		show()
	}

	d = dialog.NewCustomWithoutButtons("Getting started", content, a.win)
	d.SetOnClosed(markGuideSeen)
	show()
	d.Resize(fyne.NewSize(900, 620))
	d.Show()
}
