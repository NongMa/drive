package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestVersionGreater(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"0.1.1", "v0.1.0", true},
		{"v1.0.0", "0.9.9", true},
		{"0.1.0", "0.1.0", false},
		{"v0.1.0", "0.2.0", false},
		{"0.10.0", "0.9.0", true},
		{"garbage", "0.1.0", false},
	}
	for _, c := range cases {
		if got := versionGreater(c.a, c.b); got != c.want {
			t.Errorf("versionGreater(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSettingsUpdateUI(t *testing.T) {
	a := &app{
		screen:       "files",
		email:        "test@example.com",
		settingsOpen: true,
		dirDraft:     "",
		limitDraft:   "1",
		settings:     appSettings{MaxDownloads: 1},
		updateInfo: &releaseUpdate{
			Version: "0.2.0",
			URL:     "https://github.com/NongMa/drive/releases/tag/v0.2.0",
			Name:    "NongMa Drive 0.2.0",
			Date:    "2026-11-26",
		},
		updateState: "发现新版本 v0.2.0",
		downloads:   map[string]*downloadTask{},
		expanded:    map[string]bool{},
	}
	tt := ui.NewTester(a.view, 1400, 900)
	for _, label := range []string{"检查更新", "前往更新", "发现新版本 v0.2.0", "v0.2.0 · 2026-11-26", "当前版本 v" + fallbackVersion} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("settings missing %q", label)
		}
	}
}

func TestUpdateFromRelease(t *testing.T) {
	base := &githubRelease{
		TagName:     "v0.2.0",
		Name:        "NongMa Drive 0.2.0",
		HTMLURL:     "https://github.com/NongMa/drive/releases/tag/v0.2.0",
		PublishedAt: "2026-11-26T12:00:00Z",
	}
	if got := updateFromRelease(base, "0.1.0"); got == nil {
		t.Fatal("expected update for newer release")
	} else {
		if got.Version != "0.2.0" || got.Date != "2026-11-26" || got.URL != base.HTMLURL {
			t.Fatalf("unexpected update: %+v", got)
		}
	}
	if got := updateFromRelease(base, "0.2.0"); got != nil {
		t.Fatalf("same version must not be an update: %+v", got)
	}

	for _, rel := range []*githubRelease{
		{TagName: "v0.3.0", HTMLURL: base.HTMLURL, Draft: true},
		{TagName: "v0.3.0", HTMLURL: base.HTMLURL, Prerelease: true},
		{TagName: "v0.0.1", HTMLURL: base.HTMLURL},
		{TagName: "v0.3.0"},
	} {
		if got := updateFromRelease(rel, "0.1.0"); got != nil {
			t.Fatalf("expected no update for %+v, got %+v", rel, got)
		}
	}
}
