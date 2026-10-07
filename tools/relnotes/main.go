// Command relnotes turns one version's section of CHANGELOG.md into the
// release notes teammates see, so the notes are written once (in the
// changelog) and never drift between files.
//
//	go run ./tools/relnotes -version 1.2.0 -format txt    > RELEASE-NOTES.txt
//	go run ./tools/relnotes -version 1.2.0 -format notes  # latest.json "notes"
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
	flag.Parse()

	if *version == "" {
		fail("-version is required")
	}
	rel, err := parse(*path, *version)
	if err != nil {
		fail(err.Error())
	}

	switch *format {
	case "txt":
		fmt.Print(renderTxt(rel))
	case "notes":
		fmt.Print(renderNotes(rel))
	default:
		fail("unknown -format " + *format)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "relnotes:", msg)
	os.Exit(1)
}

var versionHeading = regexp.MustCompile(`^## \[([^\]]+)\](?: - (\S+))?`)

func parse(path, version string) (*release, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rel := &release{version: version, items: map[string][]string{}}
	found, section := false, ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := versionHeading.FindStringSubmatch(line); m != nil {
			if found {
				break // reached the next (older) version
			}
			if m[1] == version {
				found, rel.date = true, m[2]
			}
			continue
		}
		if !found {
			continue
		}
		switch {
		case strings.HasPrefix(line, "### "):
			section = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		case strings.HasPrefix(line, "- "):
			rel.items[section] = append(rel.items[section], plain(strings.TrimPrefix(line, "- ")))
		case strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" && len(rel.items[section]) > 0:
			// wrapped continuation of the previous bullet
			last := len(rel.items[section]) - 1
			rel.items[section][last] += " " + plain(strings.TrimSpace(line))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("CHANGELOG.md has no \"## [%s]\" section — write the release notes there first", version)
	}
	for _, s := range userSections {
		if len(rel.items[s.heading]) > 0 {
			return rel, nil
		}
	}
	return nil, fmt.Errorf("CHANGELOG.md [%s] has nothing under Added/Changed/Fixed — users would get empty release notes", version)
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

func renderTxt(r *release) string {
	var b strings.Builder
	title := "STP Tunnel " + r.version
	if r.date != "" {
		title += " — " + r.date
	}
	fmt.Fprintln(&b, title)
	fmt.Fprintln(&b, "macOS 10.13 ขึ้นไป (Intel และ Apple Silicon) · Windows 10 ขึ้นไป (64-bit)")

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

	fmt.Fprint(&b, `
ติดตั้ง / อัปเดต
  • macOS: แตก STPTunnel-`+r.version+`-mac.zip แล้วลาก STPTunnel.app ไปไว้ใน Applications (ปิดแอปตัวเก่าก่อน)
  • Windows: แตก STPTunnel-`+r.version+`-win.zip แล้ววาง STPTunnel.exe ทับตัวเดิม
  • project, tunnel และรหัสผ่านที่จำไว้ยังอยู่ครบ เพราะเก็บแยกจากตัวแอป
  • วิธีใช้ทีละขั้น: GUIDE-macOS.md / GUIDE-Windows.md หรือ Help > Getting started ในแอป
`)
	return b.String()
}

func renderNotes(r *release) string {
	var lines []string
	for _, s := range userSections {
		for _, it := range r.items[s.heading] {
			lines = append(lines, "• "+it)
		}
	}
	return strings.Join(lines, "\n")
}
