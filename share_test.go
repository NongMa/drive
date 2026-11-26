package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"nongma-drive/api"
)

func TestShareBarCopyAndClose(t *testing.T) {
	const link = "https://example.com/s/abc123"
	a := &app{
		screen:    "files",
		email:     "test@example.com",
		viewMode:  "list",
		files:     []api.File{{ID: "a", Name: "a.jpg", MimeType: "image/jpeg"}},
		shareURL:  link,
		downloads: map[string]*downloadTask{},
		expanded:  map[string]bool{},
	}
	tt := ui.NewTester(a.view, 1400, 900)

	if err := tt.Click("复制"); err != nil {
		t.Fatalf("click copy: %v", err)
	}
	if got := tt.Clipboard(); got != link {
		t.Fatalf("clipboard=%q want=%q", got, link)
	}
	if a.notice == "" {
		t.Fatal("expected copy feedback notice")
	}

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("click close: %v", err)
	}
	if a.shareURL != "" {
		t.Fatalf("shareURL=%q want empty", a.shareURL)
	}
}
