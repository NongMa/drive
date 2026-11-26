package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"nongma-drive/api"
)

func TestSelectAllClick(t *testing.T) {
	a := &app{
		screen:   "files",
		email:    "test@example.com",
		viewMode: "grid",
		files: []api.File{
			{ID: "a", Name: "a.jpg", MimeType: "image/jpeg"},
			{ID: "b", Name: "b.pdf", MimeType: "application/pdf"},
		},
		downloads: map[string]*downloadTask{},
		expanded:  map[string]bool{},
	}
	tt := ui.NewTester(a.view, 1400, 900)
	if err := tt.Click("全选"); err != nil {
		t.Fatalf("click 全选: %v", err)
	}
	if got := a.gridSelected.Len(); got != len(a.files) {
		t.Fatalf("selected=%d want=%d", got, len(a.files))
	}
}
