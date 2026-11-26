package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"nongma-drive/api"
)

func TestCheckboxesAlign(t *testing.T) {
	a := &app{
		screen:    "files",
		email:     "test@example.com",
		viewMode:  "list",
		files:     []api.File{{ID: "a", Name: "align.jpg", MimeType: "image/jpeg"}},
		downloads: map[string]*downloadTask{},
		expanded:  map[string]bool{},
	}
	tt := ui.NewTester(a.view, 1400, 900)

	findX := func(label string) float32 {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", label)
		}
		return r.X
	}

	titleX := findX("文件夹")
	allX := findX("全选")
	name, ok := tt.Find("align.jpg")
	if !ok {
		t.Fatal("file name not found")
	}
	tt.Move(name.X+10, name.Y+10)
	rowX := findX("选中 align.jpg")

	for _, pair := range [][2]float32{{allX, titleX}, {rowX, titleX}, {rowX, allX}} {
		if d := math.Abs(float64(pair[0] - pair[1])); d > 1 {
			t.Fatalf("x mismatch: %.1f vs %.1f (delta %.1f)", pair[0], pair[1], d)
		}
	}
}
