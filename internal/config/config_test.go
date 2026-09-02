package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	s := Parse(strings.NewReader(`# comment
browser=/usr/bin/chromium
headless=false
profile=/home/user/chatbang/profile_data
`))
	if s.Browser != "/usr/bin/chromium" || s.Headless != false || s.Profile != "/home/user/chatbang/profile_data" {
		t.Fatalf("got %+v", s)
	}

	s = Parse(strings.NewReader(""))
	if s.Headless != true {
		t.Fatal("default headless should be true")
	}
}

func TestPathsForHome(t *testing.T) {
	p := PathsForHome("/home/user")
	if p.File != "/home/user/.config/chatbang/chatbang" {
		t.Fatalf("unexpected config file path: %s", p.File)
	}
	if p.Profile != "/home/user/.config/chatbang/profile_data" {
		t.Fatalf("unexpected profile path: %s", p.Profile)
	}
	if p.Images != "/home/user/chatbang/images" {
		t.Fatalf("unexpected images path: %s", p.Images)
	}
}

func TestDetectBrowserPrefersChromiumOverBrowserOS(t *testing.T) {
	dir := t.TempDir()
	chromium := filepath.Join(dir, "chromium")
	appImage := filepath.Join(dir, "BrowserOS.AppImage")
	if err := os.WriteFile(chromium, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appImage, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := detectBrowserIn([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != chromium {
		t.Fatalf("got %q, want %q", got, chromium)
	}
}

func TestDetectBrowserFallsBackToBrowserOS(t *testing.T) {
	dir := t.TempDir()
	appImage := filepath.Join(dir, "BrowserOS.AppImage")
	if err := os.WriteFile(appImage, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := detectBrowserIn([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != appImage {
		t.Fatalf("got %q, want %q", got, appImage)
	}
}

func TestResolveBrowser(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(existing, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, updated, err := resolveBrowser(existing, []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if updated || got != existing {
		t.Fatalf("got path=%q updated=%v, want keep %q", got, updated, existing)
	}

	dir := t.TempDir()
	want := filepath.Join(dir, "chromium-browser")
	if err := os.WriteFile(want, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, updated, err = resolveBrowser("/no/such/google-chrome", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !updated || got != want {
		t.Fatalf("got path=%q updated=%v, want %q", got, updated, want)
	}
}

func TestIsSnapBrowser(t *testing.T) {
	if !IsSnapBrowser("/snap/bin/chromium") {
		t.Fatal("snap path should be detected")
	}
	wrapper := filepath.Join(t.TempDir(), "chromium-browser")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec /snap/bin/chromium \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsSnapBrowser(wrapper) {
		t.Fatal("ubuntu snap wrapper should be detected")
	}
	native := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(native, []byte("ELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsSnapBrowser(native) {
		t.Fatal("regular binary should not be snap")
	}
}

func TestProfileDir(t *testing.T) {
	if got := ProfileDir("/home/me", "/usr/bin/chrome", "/custom"); got != "/custom" {
		t.Fatalf("configured profile: %q", got)
	}
	wrapper := filepath.Join(t.TempDir(), "chromium-browser")
	if err := os.WriteFile(wrapper, []byte("exec /snap/bin/chromium\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ProfileDir("/home/me", wrapper, "")
	if got != "/home/me/chatbang/profile_data" {
		t.Fatalf("snap profile = %q", got)
	}
	got = ProfileDir("/home/me", "/opt/google/chrome/chrome", "")
	if got != "/home/me/.config/chatbang/profile_data" {
		t.Fatalf("native profile = %q", got)
	}
}

func TestFormat(t *testing.T) {
	got := Format(Settings{
		Browser:  "/usr/bin/chromium-browser",
		Headless: true,
		Profile:  "/home/me/chatbang/profile_data",
	})
	want := "browser=/usr/bin/chromium-browser\nheadless=true\nprofile=/home/me/chatbang/profile_data\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
