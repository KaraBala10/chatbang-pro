package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

var browserSearchDirs = []string{"/usr/bin", "/bin", "/usr/local/bin", "/snap/bin"}

var preferredBrowsers = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"ungoogled-chromium",
	"microsoft-edge",
	"microsoft-edge-stable",
	"brave-browser",
	"vivaldi",
	"opera",
	"msedge",
}

var fallbackBrowsers = []string{
	"BrowserOS.AppImage",
	"browseros",
	"BrowserOS",
}

// Settings holds parsed values from the chatbang config file.
type Settings struct {
	Browser  string
	Headless bool
	Profile  string
}

// Paths holds config and profile locations under the user home directory.
type Paths struct {
	Dir     string
	File    string
	Profile string
	Images  string
}

// PathsForHome returns standard chatbang config paths for a home directory.
func PathsForHome(homeDir string) Paths {
	dir := filepath.Join(homeDir, ".config", "chatbang")
	return Paths{
		Dir:     dir,
		File:    filepath.Join(dir, "chatbang"),
		Profile: filepath.Join(dir, "profile_data"),
		Images:  filepath.Join(homeDir, "chatbang", "images"),
	}
}

func AllocatorOptions(browserPath, profileDir string, headless bool) []chromedp.ExecAllocatorOption {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		ExecPathOptions(browserPath)...,
	)
	opts = append(opts,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-extensions", false),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-default-apps", false),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("profile-directory", "Default"),
	)
	opts = append(opts, chromedp.Flag("headless", headless))
	return opts
}

// ExecPathOptions are chromedp allocator options for a browser executable.
func ExecPathOptions(browserPath string) []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{chromedp.ExecPath(browserPath)}
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func searchDirs(home string) []string {
	dirs := make([]string, 0, len(browserSearchDirs)+1)
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"))
	}
	return append(dirs, browserSearchDirs...)
}

func findBrowser(dirs, names []string) string {
	for _, name := range names {
		for _, dir := range dirs {
			path := filepath.Join(dir, name)
			if fileExists(path) {
				return path
			}
		}
	}
	return ""
}

// DetectBrowser searches standard locations, preferring Chromium/Chrome over BrowserOS.
func DetectBrowser() (string, error) {
	home, _ := os.UserHomeDir()
	return DetectBrowserIn(home)
}

// DetectBrowserIn searches for a Chromium-based browser under home and system dirs.
func DetectBrowserIn(home string) (string, error) {
	return detectBrowserIn(searchDirs(home))
}

func detectBrowserIn(dirs []string) (string, error) {
	if path := findBrowser(dirs, preferredBrowsers); path != "" {
		return path, nil
	}
	if path := findBrowser(dirs, fallbackBrowsers); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("no Chromium-based browser found in ~/.local/bin, /bin, /usr/bin, or /usr/local/bin")
}

// ResolveBrowser returns a usable browser path. If configured is empty or missing, it detects one.
// updated is true when the caller should persist the new path to the config file.
func ResolveBrowser(configured, home string) (path string, updated bool, err error) {
	return resolveBrowser(configured, searchDirs(home))
}

func resolveBrowser(configured string, dirs []string) (path string, updated bool, err error) {
	if configured != "" && fileExists(configured) {
		return configured, false, nil
	}
	path, err = detectBrowserIn(dirs)
	if err != nil {
		if configured != "" {
			return "", false, fmt.Errorf("configured browser %s is missing, and no Chromium-based browser was found", configured)
		}
		return "", false, err
	}
	return path, path != configured, nil
}

// IsSnapBrowser reports whether the executable is Ubuntu's Chromium snap wrapper.
func IsSnapBrowser(path string) bool {
	if path == "" {
		return false
	}
	resolved := path
	if p, err := filepath.EvalSymlinks(path); err == nil {
		resolved = p
	}
	if strings.Contains(path, "/snap/") || strings.Contains(resolved, "/snap/") {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 16*1024 {
		return false
	}
	return bytes.Contains(data, []byte("/snap/bin/chromium")) || bytes.Contains(data, []byte("snap run chromium"))
}

// ProfileDir is the Chromium user-data dir. Snap Chromium cannot write hidden
// paths like ~/.config, so it uses ~/chatbang/profile_data instead.
func ProfileDir(home, browserPath, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	if IsSnapBrowser(browserPath) {
		return filepath.Join(home, "chatbang", "profile_data")
	}
	return filepath.Join(home, ".config", "chatbang", "profile_data")
}

// Format writes the on-disk config body.
func Format(s Settings) string {
	var b strings.Builder
	b.WriteString("browser=" + s.Browser + "\n")
	b.WriteString("headless=" + strconv.FormatBool(s.Headless) + "\n")
	if strings.TrimSpace(s.Profile) != "" {
		b.WriteString("profile=" + s.Profile + "\n")
	}
	return b.String()
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// Parse reads browser, headless, and profile settings from a config file reader.
func Parse(r io.Reader) Settings {
	s := Settings{Headless: true}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "browser":
			s.Browser = value
		case "headless":
			if parsed, ok := parseBool(value); ok {
				s.Headless = parsed
			}
		case "profile":
			s.Profile = value
		}
	}
	return s
}
