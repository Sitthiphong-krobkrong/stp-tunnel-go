// Command relnotes turns one version's section of CHANGELOG.md into the
// release notes teammates see, so the notes are written once (in the
// changelog) and never drift between files.
//
//	go run ./tools/relnotes -version 1.2.0 -format txt    > RELEASE-NOTES.txt
//	go run ./tools/relnotes -version 1.2.0 -format notes  # latest.json "notes"
//	go run ./tools/relnotes -version 1.2.1 -since 1.0.0   # everything after 1.0.0
//
// -since is the version teammates have now (what the update feed says):
// someone jumping from 1.0.0 to 1.2.1 should read what 1.1.0 and 1.2.0
// brought too, not just the last patch.
//
// Only user-facing subsections are kept (Added / Changed / Fixed / Security /
// Known issues); "### Internal" and anything else is for developers and
// stays out. It exits non-zero when the version has no section or nothing
// user-facing in it, so a release can't ship without notes.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// userSections maps changelog subsection headings to the Thai headings shown
// to users, in the order they're printed.
var userSections = []struct{ heading, title string }{
	{"Added", "มีอะไรใหม่"},
	{"Changed", "เปลี่ยนแปลง"},
	{"Fixed", "แก้ไข"},
	{"Security", "ความปลอดภัย"},
	{"Known issues", "ปัญหาที่ทราบแล้ว"},
}

type release struct {
	version, date string
	items         map[string][]string // subsection heading -> bullets
}

func main() {
	version := flag.String("version", "", "version to extract, e.g. 1.2.0 (required)")
	path := flag.String("changelog", "CHANGELOG.md", "path to the changelog")
	format := flag.String("format", "txt", "txt (RELEASE-NOTES.txt) or notes (latest.json notes)")
	since := flag.String("since", "", "also include every version newer than this one (the version users have now)")
	flag.Parse()

	if *version == "" {
		fail("-version is required")
	}
	all, err := parse(*path)
	if err != nil {
		fail(err.Error())
	}
	rels, err := selectReleases(all, *version, *since)
	if err != nil {
		fail(err.Error())
	}

	switch *format {
	case "txt":
		fmt.Print(renderTxt(rels))
	case "notes":
		fmt.Print(renderNotes(rels))
	default:
		fail("unknown -format " + *format)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "relnotes:", msg)
	os.Exit(1)
}

var versionHeading = regexp.MustCompile(`^## \[([^\]]+)\](?: - (\S+))?`)

// parse reads every "## [x.y.z] - date" section, newest first (file order).
func parse(path string) ([]*release, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rels []*release
	var cur *release
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := versionHeading.FindStringSubmatch(line); m != nil {
			cur, section = nil, ""
			if m[1] != "Unreleased" {
				cur = &release{version: m[1], date: m[2], items: map[string][]string{}}
				rels = append(rels, cur)
			}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "### "):
			section = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		case strings.HasPrefix(line, "- "):
			cur.items[section] = append(cur.items[section], plain(strings.TrimPrefix(line, "- ")))
		case strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" && len(cur.items[section]) > 0:
			// wrapped continuation of the previous bullet
			last := len(cur.items[section]) - 1
			cur.items[section][last] += " " + plain(strings.TrimSpace(line))
		}
	}
	return rels, sc.Err()
}

// selectReleases picks version itself plus, with since, every version newer
// than since — newest first. version must exist and have user-facing notes.
func selectReleases(all []*release, version, since string) ([]*release, error) {
	var out []*release
	found := false
	for _, r := range all {
		switch {
		case r.version == version:
			found = true
			if !r.hasUserNotes() {
				return nil, fmt.Errorf("CHANGELOG.md [%s] has nothing under Added/Changed/Fixed — users would get empty release notes", version)
			}
			out = append(out, r)
		case since != "" && newer(r.version, since) && newer(version, r.version) && r.hasUserNotes():
			out = append(out, r)
		}
	}
	if !found {
		return nil, fmt.Errorf("CHANGELOG.md has no \"## [%s]\" section — write the release notes there first", version)
	}
	return out, nil
}

func (r *release) hasUserNotes() bool {
	for _, s := range userSections {
		if len(r.items[s.heading]) > 0 {
			return true
		}
	}
	return false
}

// newer reports whether semver a > b (numeric parts only).
func newer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			fmt.Sscan(pb[i], &y)
		}
		if x != y {
			return x > y
		}
	}
	return false
}

var (
	mdLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	mdCode = regexp.MustCompile("`([^`]*)`")
	mdBold = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

// plain strips the markdown that reads badly in a .txt file or a dialog.
func plain(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdCode.ReplaceAllString(s, "$1")
	s = mdBold.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func renderTxt(rels []*release) string {
	var b strings.Builder
	top := rels[0]
	title := "STP Tunnel " + top.version
	if top.date != "" {
		title += " — " + top.date
	}
	fmt.Fprintln(&b, title)
	fmt.Fprintln(&b, "macOS 10.13 ขึ้นไป (Intel และ Apple Silicon) · Windows 10 ขึ้นไป (64-bit)")

	for _, r := range rels {
		if len(rels) > 1 {
			fmt.Fprintf(&b, "\n========== %s", r.version)
			if r.date != "" {
				fmt.Fprintf(&b, " (%s)", r.date)
			}
			fmt.Fprintln(&b, " ==========")
		}
		for _, s := range userSections {
			items := r.items[s.heading]
			if len(items) == 0 {
				continue
			}
			fmt.Fprintf(&b, "\n%s\n", s.title)
			for _, it := range items {
				fmt.Fprintf(&b, "  • %s\n", it)
			}
		}
	}

	fmt.Fprint(&b, `
ติดตั้ง / อัปเดต
  • macOS: แตก STPTunnel-`+top.version+`-mac.zip แล้วลาก STPTunnel.app ไปไว้ใน Applications (ปิดแอปตัวเก่าก่อน)
  • Windows: แตก STPTunnel-`+top.version+`-win.zip แล้ววาง STPTunnel.exe ทับตัวเดิม
  • project, tunnel และรหัสผ่านที่จำไว้ยังอยู่ครบ เพราะเก็บแยกจากตัวแอป
  • วิธีใช้ทีละขั้น: GUIDE-macOS.md / GUIDE-Windows.md หรือ Help > Getting started ในแอป
`)
	return b.String()
}

// renderNotes is the short form for the in-app "Update Available" dialog:
// Added/Changed/Fixed/Security only (known issues stay in the txt), with a
// version line between releases when more than one is included.
func renderNotes(rels []*release) string {
	var lines []string
	for _, r := range rels {
		if len(rels) > 1 {
			lines = append(lines, r.version)
		}
		for _, s := range userSections {
			if s.heading == "Known issues" {
				continue
			}
			for _, it := range r.items[s.heading] {
				lines = append(lines, "• "+it)
			}
		}
	}
	return strings.Join(lines, "\n")
}
