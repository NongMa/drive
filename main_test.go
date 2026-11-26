package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLoadEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("RESOURCE_HUB_API=https://example.com/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	t.Setenv("RESOURCE_HUB_API", "")
	loadEnv()
	if got := os.Getenv("RESOURCE_HUB_API"); got != "https://example.com/" {
		t.Fatalf("loaded %q", got)
	}
}

// The view runs without a window in tests, so the login screen can be
// checked without opening a GUI.
func TestLoginView(t *testing.T) {
	a := &app{base: defaultBaseURL}
	a.setAPI()
	tt := ui.NewTester(a.view, 900, 620)
	for _, text := range []string{"资源管家", "登录以管理你的文件", "服务地址", "邮箱", "密码", "登录"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in view, texts: %q", text, tt.Texts())
		}
	}
}
