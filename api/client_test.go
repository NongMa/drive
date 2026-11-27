package api

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveSessionCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "session.json")
	c := New("https://example.com")
	if err := c.SaveSession(path); err != nil {
		t.Fatalf("SaveSession with missing parent dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	u, _ := url.Parse("https://example.com")

	first := New("https://example.com")
	first.jar.SetCookies(u, []*http.Cookie{{
		Name: "next-auth.session-token", Value: "test-jwt", Path: "/",
		Expires: time.Now().Add(time.Hour), HttpOnly: true,
	}})
	if err := first.SaveSession(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("session file permissions: %o", info.Mode().Perm())
	}

	second := New("https://example.com")
	if err := second.LoadSession(path); err != nil {
		t.Fatal(err)
	}
	cookies := second.jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "next-auth.session-token" || cookies[0].Value != "test-jwt" {
		t.Fatalf("restored cookies: %+v", cookies)
	}

	second.ClearSession(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file not removed: %v", err)
	}
}
