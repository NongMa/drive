package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"nongma-drive/api"
)

func listNameX(t *testing.T, task *downloadTask) float32 {
	t.Helper()
	a := &app{
		screen:    "files",
		email:     "test@example.com",
		viewMode:  "list",
		files:     []api.File{{ID: "a", Name: "align.jpg", MimeType: "image/jpeg", Size: 1024}},
		downloads: map[string]*downloadTask{},
		expanded:  map[string]bool{},
	}
	if task != nil {
		a.downloads["a"] = task
	}
	tt := ui.NewTester(a.view, 1400, 900)
	r, ok := tt.Find("align.jpg")
	if !ok {
		t.Fatal("file name not found")
	}
	return r.X
}

func TestListHeaderAlignsWithData(t *testing.T) {
	a := &app{
		screen:    "files",
		email:     "test@example.com",
		viewMode:  "list",
		files:     []api.File{{ID: "a", Name: "align.jpg", MimeType: "image/jpeg", Size: 1024}},
		downloads: map[string]*downloadTask{"a": {ID: "a", Status: "error"}},
		expanded:  map[string]bool{},
	}
	tt := ui.NewTester(a.view, 1400, 900)
	header, ok := tt.Find("大小")
	if !ok {
		t.Fatal("size header not found")
	}
	data, ok := tt.Find("1.0 KB")
	if !ok {
		t.Fatal("size cell not found")
	}
	if d := math.Abs(float64(header.X+header.W) - float64(data.X+data.W)); d > 1 {
		t.Fatalf("size column right edges off by %.1f (header %.1f, data %.1f)", d, header.X+header.W, data.X+data.W)
	}
}

func TestListStatusDoesNotShiftColumns(t *testing.T) {
	plain := listNameX(t, nil)
	for _, task := range []*downloadTask{
		{ID: "a", Status: "queued"},
		{ID: "a", Status: "downloading", Percent: 42},
		{ID: "a", Status: "done", Percent: 100},
		{ID: "a", Status: "error"},
	} {
		got := listNameX(t, task)
		if d := math.Abs(float64(got - plain)); d > 1 {
			t.Fatalf("status %q shifted name column by %.1f (plain %.1f, got %.1f)", task.Status, d, plain, got)
		}
	}
}
