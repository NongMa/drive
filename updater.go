package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// latestReleaseURL is the public GitHub API endpoint for the newest release of
// this desktop app. It contains no deployment-specific information.
const latestReleaseURL = "https://api.github.com/repos/NongMa/nongma-drive/releases/latest"

// fallbackVersion matches the version in mygo.json; it is used when the
// runtime has no embedded build version (for example in dev builds).
const fallbackVersion = "0.2.0"

func currentVersion() string {
	if v := strings.TrimSpace(mygo.App.Version()); v != "" {
		return v
	}
	return fallbackVersion
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

type releaseUpdate struct {
	Version string
	URL     string
	Name    string
	Date    string
}

// fetchLatestRelease returns the newest public release from GitHub.
func fetchLatestRelease(hc *http.Client) (*githubRelease, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nongma-drive")
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回 %s", res.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析更新信息失败: %w", err)
	}
	return &rel, nil
}

var versionRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

func versionParts(s string) (major, minor, patch int, ok bool) {
	m := versionRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return major, minor, patch, true
}

// versionGreater reports whether a is a newer semantic version than b.
func versionGreater(a, b string) bool {
	am, an, ap, aok := versionParts(a)
	bm, bn, bp, bok := versionParts(b)
	if !aok || !bok {
		return false
	}
	if am != bm {
		return am > bm
	}
	if an != bn {
		return an > bn
	}
	return ap > bp
}

// updateFromRelease converts a GitHub release into an update only when it is a
// public, stable release newer than the running app.
func updateFromRelease(rel *githubRelease, current string) *releaseUpdate {
	if rel == nil || rel.Draft || rel.Prerelease || rel.TagName == "" || rel.HTMLURL == "" {
		return nil
	}
	if !versionGreater(rel.TagName, current) {
		return nil
	}
	date := rel.PublishedAt
	if i := strings.IndexByte(date, 'T'); i > 0 {
		date = date[:i]
	}
	return &releaseUpdate{
		Version: strings.TrimPrefix(rel.TagName, "v"),
		URL:     rel.HTMLURL,
		Name:    rel.Name,
		Date:    date,
	}
}

func (a *app) checkUpdate() {
	if a.updateChecking {
		return
	}
	a.updateChecking = true
	a.updateErr = ""
	a.async(func() {
		rel, err := fetchLatestRelease(nil)
		var found *releaseUpdate
		if err == nil {
			found = updateFromRelease(rel, currentVersion())
		}
		a.win.Update(func() {
			a.updateChecking = false
			if err != nil {
				a.updateErr = "检查更新失败: " + err.Error()
				return
			}
			a.updateInfo = found
			if found == nil {
				a.updateState = "已是最新版本"
			} else {
				a.updateState = "发现新版本 v" + found.Version
			}
		})
	})
}
