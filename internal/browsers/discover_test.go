package browsers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverChromiumAndFirefox(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	chromeBin := filepath.Join(binDir, "google-chrome")
	if err := os.WriteFile(chromeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	chromeRoot := filepath.Join(home, ".config", "google-chrome")
	defaultCookies := filepath.Join(chromeRoot, "Default", "Network", "Cookies")
	if err := os.MkdirAll(filepath.Dir(defaultCookies), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultCookies, []byte("cookies"), 0o644); err != nil {
		t.Fatal(err)
	}
	localState := `{
		"profile": {
			"info_cache": {
				"Default": {"name": "Person 1", "user_name": "me@example.com"}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(chromeRoot, "Local State"), []byte(localState), 0o644); err != nil {
		t.Fatal(err)
	}

	ffRoot := filepath.Join(home, ".mozilla", "firefox")
	ffProfile := filepath.Join(ffRoot, "abcd.default")
	if err := os.MkdirAll(ffProfile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ffProfile, "cookies.sqlite"), []byte("sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	ini := `[Profile0]
Name=default
IsRelative=1
Path=abcd.default
Default=1
`
	if err := os.WriteFile(filepath.Join(ffRoot, "profiles.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}

	got := discover(discoverOptions{
		Home:       home,
		BinaryDirs: []string{binDir},
		SearchPATH: false,
	})
	if len(got) != 2 {
		t.Fatalf("got %d sources, want 2: %+v", len(got), got)
	}

	var chrome, firefox *Source
	for i := range got {
		switch got[i].Name {
		case "Google Chrome":
			chrome = &got[i]
		case "Firefox":
			firefox = &got[i]
		}
	}
	if chrome == nil || firefox == nil {
		t.Fatalf("missing browser in %+v", got)
	}
	if chrome.ExecPath != chromeBin {
		t.Fatalf("chrome exec = %q, want %q", chrome.ExecPath, chromeBin)
	}
	if !strings.Contains(chrome.Label(), "Person 1") || !strings.Contains(chrome.Label(), "me@example.com") {
		t.Fatalf("chrome label = %q", chrome.Label())
	}
	if firefox.Profile != "default" {
		t.Fatalf("firefox profile = %q", firefox.Profile)
	}
	if firefox.CookiesPath != filepath.Join(ffProfile, "cookies.sqlite") {
		t.Fatalf("firefox cookies = %q", firefox.CookiesPath)
	}
}

func TestDiscoverSkipsEmptyProfiles(t *testing.T) {
	home := t.TempDir()
	chromeRoot := filepath.Join(home, ".config", "google-chrome")
	if err := os.MkdirAll(filepath.Join(chromeRoot, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := discover(discoverOptions{Home: home, SearchPATH: false})
	if len(got) != 0 {
		t.Fatalf("expected no sources, got %+v", got)
	}
}

func TestDiscoverBrowserOS(t *testing.T) {
	home := t.TempDir()
	appImage := filepath.Join(home, ".local", "bin", "BrowserOS.AppImage")
	if err := os.MkdirAll(filepath.Dir(appImage), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appImage, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	cookies := filepath.Join(home, ".config", "browser-os", "Default", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookies), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookies, []byte("cookies"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := discover(discoverOptions{Home: home, SearchPATH: false})
	if len(got) != 1 {
		t.Fatalf("got %d sources, want 1: %+v", len(got), got)
	}
	if got[0].Name != "BrowserOS" {
		t.Fatalf("name = %q", got[0].Name)
	}
	if got[0].ExecPath != appImage {
		t.Fatalf("exec = %q, want %q", got[0].ExecPath, appImage)
	}
	if got[0].CookiesPath != cookies {
		t.Fatalf("cookies = %q", got[0].CookiesPath)
	}
}
