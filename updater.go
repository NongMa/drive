package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"nongma-drive/api"
)

// latestReleaseURL is the public GitHub API endpoint for the newest release of
// this desktop app. It contains no deployment-specific information.
const latestReleaseURL = "https://api.github.com/repos/NongMa/drive/releases/latest"

// fallbackVersion matches the version in mygo.json; it is used when the
// runtime has no embedded build version (for example in dev builds).
const fallbackVersion = "0.2.2"

func currentVersion() string {
	if v := strings.TrimSpace(mygo.App.Version()); v != "" {
		return v
	}
	return fallbackVersion
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []githubAsset `json:"assets"`
}

type releaseAsset struct {
	Name string
	URL  string
	Size int64
}

type releaseUpdate struct {
	Version string
	URL     string
	Name    string
	Date    string
	Asset   *releaseAsset
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

// pickUpdateAsset selects the installer for the running platform. Windows uses
// the setup exe; macOS uses the dmg matching the CPU architecture.
func pickUpdateAsset(rel *githubRelease) *releaseAsset {
	if rel == nil {
		return nil
	}
	var want string
	switch runtime.GOOS {
	case "windows":
		want = "setup-"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			want = "-arm64.dmg"
		} else {
			want = "-amd64.dmg"
		}
	default:
		return nil
	}
	var fallback *releaseAsset
	for _, a := range rel.Assets {
		if a.BrowserDownloadURL == "" {
			continue
		}
		name := strings.ToLower(a.Name)
		asset := &releaseAsset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size}
		if strings.Contains(name, want) {
			return asset
		}
		if runtime.GOOS == "windows" && strings.HasSuffix(name, ".exe") &&
			!strings.Contains(name, "portable") && fallback == nil {
			fallback = asset
		}
	}
	return fallback
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
		Asset:   pickUpdateAsset(rel),
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
			a.updateDownload, a.updatePath = nil, ""
			if found == nil {
				a.updateState = "已是最新版本"
			} else if found.Asset == nil {
				a.updateState = "发现新版本 v" + found.Version + "，但没有当前平台的安装包"
			} else {
				a.updateState = "发现新版本 v" + found.Version
			}
		})
	})
}

// downloadUpdate fetches the installer into the download directory and keeps
// its progress in a dedicated task so the settings dialog can show it inline.
func (a *app) downloadUpdate() {
	if a.updateInfo == nil || a.updateInfo.Asset == nil {
		return
	}
	if a.updateDownload != nil && a.updateDownload.Status == "downloading" {
		return
	}
	asset := a.updateInfo.Asset
	dir := strings.TrimSpace(a.settings.DownloadDir)
	if dir == "" {
		dir = a.defaultDownloadDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		a.updateErr = "无法创建下载目录: " + err.Error()
		return
	}
	dest := filepath.Join(dir, asset.Name)
	_ = os.Remove(dest)
	task := &downloadTask{ID: "__update__", Name: asset.Name, Status: "downloading"}
	a.updateDownload = task
	a.updateErr = ""
	a.async(func() {
		err := api.DownloadToFile(asset.URL, dest, func(done, total int64) {
			pct := 0
			if total > 0 {
				pct = int(float64(done) * 100 / float64(total))
			}
			a.win.Update(func() { task.Percent = pct })
		})
		a.win.Update(func() {
			if err != nil {
				task.Status, task.Err = "error", err.Error()
				a.updateErr = "更新下载失败: " + err.Error()
				return
			}
			task.Status, task.Percent = "done", 100
			a.updatePath = dest
		})
	})
}

// installUpdate opens the downloaded installer/dmg with the system handler.
func (a *app) installUpdate() {
	if a.updatePath == "" {
		return
	}
	_ = mygo.Shell.OpenPath(a.updatePath)
}
