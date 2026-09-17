package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// version and buildDate are filled in at build time by build.sh:
//
//	go build -ldflags "-X main.version=1.0.1 -X main.buildDate=2026-09-17" -o stp-tunnel .
//
// The value here is the fallback for a plain `go build` during development —
// deliberately not a real-looking version number, so a binary someone forgot
// to stamp is never mistaken for an actual release.
var (
	version   = "dev"
	buildDate = ""
)

// copyright is shown in the window footer.
const copyright = "\u00a9 2026 stp"

// updateFeedURL points at a small JSON file describing the latest release.
// Leave it empty until there's somewhere to host releases — the update check
// then simply stays out of the way instead of failing.
//
// Expected contents:
//
//	{"version": "1.0.1", "url": "https://.../releases", "notes": "what changed"}
const updateFeedURL = ""

type releaseInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

// versionString is the short form shown in the footer, e.g. "v1.0.0".
func versionString() string {
	return "v" + version
}

// versionDetail adds the build date, for bug reports.
func versionDetail() string {
	if buildDate == "" {
		return versionString()
	}
	return fmt.Sprintf("%s (%s)", versionString(), buildDate)
}

// checkForUpdate fetches the release feed and reports a newer version, if any.
// It returns nil, nil when the app is current or no feed is configured.
func checkForUpdate() (*releaseInfo, error) {
	if updateFeedURL == "" {
		return nil, fmt.Errorf("no release feed is configured in this build")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(updateFeedURL)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check for updates: server said %s", resp.Status)
	}

	var latest releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}

	if isNewer(latest.Version, version) {
		return &latest, nil
	}
	return nil, nil
}

// isNewer compares two versions number-group by number-group, so it works for
// both semver-style (1.2.0) and date-style (2026.09.17) versions.
func isNewer(candidate, current string) bool {
	a, b := versionParts(candidate), versionParts(current)
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func versionParts(v string) []int {
	var out []int
	for _, field := range strings.FieldsFunc(v, func(r rune) bool { return r < '0' || r > '9' }) {
		n, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

// diagnostics gathers everything worth pasting into a bug report — and
// nothing that shouldn't leave the machine: passwords are never included.
func (a *App) diagnostics() string {
	var b strings.Builder

	fmt.Fprintf(&b, "STP Tunnel %s\n", versionDetail())
	fmt.Fprintf(&b, "OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "Time: %s\n\n", time.Now().Format(time.RFC3339))

	if proj, ok := a.currentProject(); ok {
		fmt.Fprintf(&b, "Project: %s\n", proj.Name)
		fmt.Fprintf(&b, "  host: %s@%s:%s\n", proj.BastionUser, proj.BastionHost, portOrDefault(proj.BastionPort))
		fmt.Fprintf(&b, "  auth: %s\n", map[bool]string{true: "identity file", false: "password"}[proj.IdentityFile != ""])
		fmt.Fprintf(&b, "  skip host key check: %v\n", proj.SkipHostKeyCheck)
	} else {
		b.WriteString("Project: (none selected)\n")
	}

	tunnels := a.currentTunnels()
	running := 0
	for _, t := range tunnels {
		if a.runtime.IsRunning(t.ID) {
			running++
		}
	}
	fmt.Fprintf(&b, "Tunnels: %d (%d running)\n", len(tunnels), running)
	for _, t := range tunnels {
		status, _ := a.tunnelStatus(t)
		fmt.Fprintf(&b, "  %-16s %s:%s → %s:%s  [%s]\n",
			t.ServiceName, "localhost", t.LocalPort, t.TargetHost, t.TargetPort, status)
	}

	b.WriteString("\nLast log lines:\n")
	b.WriteString(lastLines(a.logEntry.Text, 20))

	return b.String()
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
