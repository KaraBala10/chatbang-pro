package browsers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type discoverOptions struct {
	Home       string
	BinaryDirs []string
	SearchPATH bool
}

var defaultBinaryDirs = []string{"/usr/bin", "/bin", "/usr/local/bin", "/snap/bin"}

type chromiumSpec struct {
	name       string
	binaries   []string
	absPaths   []string
	configDirs []string
}

var chromiumSpecs = []chromiumSpec{
	{
		name:     "BrowserOS",
		binaries: []string{"BrowserOS.AppImage", "browseros", "BrowserOS"},
		configDirs: []string{
			".config/browser-os",
			".config/browseros",
			".config/BrowserOS",
		},
	},
	{
		name:     "Google Chrome",
		binaries: []string{"google-chrome", "google-chrome-stable"},
		absPaths: []string{"/opt/google/chrome/google-chrome", "/opt/google/chrome/chrome"},
		configDirs: []string{
			".config/google-chrome",
			".var/app/com.google.Chrome/config/google-chrome",
		},
	},
	{
		name:     "Chromium",
		binaries: []string{"chromium", "chromium-browser"},
		absPaths: []string{"/usr/lib/chromium-browser/chromium-browser", "/usr/lib/chromium/chromium"},
		configDirs: []string{
			".config/chromium",
			"snap/chromium/common/chromium",
			".var/app/org.chromium.Chromium/config/chromium",
		},
	},
	{
		name:     "Brave",
		binaries: []string{"brave-browser", "brave"},
		absPaths: []string{"/opt/brave.com/brave/brave-browser", "/opt/brave.com/brave/brave"},
		configDirs: []string{
			".config/BraveSoftware/Brave-Browser",
			".var/app/com.brave.Browser/config/BraveSoftware/Brave-Browser",
		},
	},
	{
		name:     "Microsoft Edge",
		binaries: []string{"microsoft-edge", "microsoft-edge-stable", "msedge"},
		absPaths: []string{"/opt/microsoft/msedge/microsoft-edge", "/opt/microsoft/msedge/msedge"},
		configDirs: []string{
			".config/microsoft-edge",
			".var/app/com.microsoft.Edge/config/microsoft-edge",
		},
	},
	{
		name:       "Vivaldi",
		binaries:   []string{"vivaldi", "vivaldi-stable"},
		absPaths:   []string{"/opt/vivaldi/vivaldi"},
		configDirs: []string{".config/vivaldi"},
	},
	{
		name:       "Opera",
		binaries:   []string{"opera"},
		absPaths:   []string{"/usr/lib/x86_64-linux-gnu/opera/opera"},
		configDirs: []string{".config/opera"},
	},
}

type firefoxSpec struct {
	name       string
	binaries   []string
	absPaths   []string
	configDirs []string
}

var firefoxSpecDefault = firefoxSpec{
	name:     "Firefox",
	binaries: []string{"firefox", "firefox-esr"},
	configDirs: []string{
		".mozilla/firefox",
		"snap/firefox/common/.mozilla/firefox",
		".var/app/org.mozilla.firefox/.mozilla/firefox",
	},
}

// Discover finds browser profiles under home that look like they store cookies.
func Discover(home string) []Source {
	return discover(discoverOptions{
		Home:       home,
		BinaryDirs: defaultBinaryDirs,
		SearchPATH: true,
	})
}

func discover(opts discoverOptions) []Source {
	var sources []Source
	for _, spec := range chromiumSpecs {
		sources = append(sources, discoverChromium(opts, spec)...)
	}
	sources = append(sources, discoverFirefox(opts, firefoxSpecDefault)...)
	sources = dedupeSources(sources)
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Name != sources[j].Name {
			return sources[i].Name < sources[j].Name
		}
		return sources[i].Label() < sources[j].Label()
	})
	return sources
}

func discoverChromium(opts discoverOptions, spec chromiumSpec) []Source {
	exe := findBinary(spec.binaries, spec.absPaths, opts)
	var sources []Source
	for _, rel := range spec.configDirs {
		userData := filepath.Join(opts.Home, rel)
		profiles := chromiumProfiles(userData)
		for _, p := range profiles {
			src := Source{
				Name:        spec.name,
				Profile:     p.display,
				ProfileDir:  p.dir,
				ExecPath:    exe,
				UserDataDir: userData,
				CookiesPath: p.cookies,
				Kind:        KindChromium,
			}
			sources = append(sources, src)
		}
	}
	return sources
}

func discoverFirefox(opts discoverOptions, spec firefoxSpec) []Source {
	exe := findBinary(spec.binaries, spec.absPaths, opts)
	var sources []Source
	for _, rel := range spec.configDirs {
		root := filepath.Join(opts.Home, rel)
		for _, p := range firefoxProfiles(root) {
			sources = append(sources, Source{
				Name:        spec.name,
				Profile:     p.display,
				ProfileDir:  p.dir,
				ExecPath:    exe,
				UserDataDir: root,
				CookiesPath: p.cookies,
				Kind:        KindFirefox,
			})
		}
	}
	return sources
}

func findBinary(names, absPaths []string, opts discoverOptions) string {
	dirs := opts.BinaryDirs
	if opts.Home != "" {
		dirs = append([]string{filepath.Join(opts.Home, ".local", "bin")}, dirs...)
	}
	for _, dir := range dirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if fileExists(path) {
				return path
			}
		}
	}
	for _, path := range absPaths {
		if fileExists(path) {
			return path
		}
	}
	if opts.SearchPATH {
		for _, name := range names {
			if path, err := exec.LookPath(name); err == nil {
				return path
			}
		}
	}
	return ""
}

type profileInfo struct {
	dir     string
	display string
	cookies string
}

type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name     string `json:"name"`
			UserName string `json:"user_name"`
			GAIAName string `json:"gaia_name"`
		} `json:"info_cache"`
	} `json:"profile"`
}

func chromiumProfiles(userDataDir string) []profileInfo {
	st, err := os.Stat(userDataDir)
	if err != nil || !st.IsDir() {
		return nil
	}

	byDir := map[string]profileInfo{}
	data, err := os.ReadFile(filepath.Join(userDataDir, "Local State"))
	if err == nil {
		var state localState
		if json.Unmarshal(data, &state) == nil {
			for dir, info := range state.Profile.InfoCache {
				display := strings.TrimSpace(info.Name)
				if display == "" {
					display = dir
				}
				if extra := firstNonEmpty(info.UserName, info.GAIAName); extra != "" && extra != display {
					display = display + " (" + extra + ")"
				}
				byDir[dir] = profileInfo{dir: dir, display: display}
			}
		}
	}
	if _, ok := byDir["Default"]; !ok {
		byDir["Default"] = profileInfo{dir: "Default", display: "Default"}
	}

	var out []profileInfo
	for dir, p := range byDir {
		profileDir := filepath.Join(userDataDir, dir)
		cookies := chromiumCookiesPath(profileDir)
		if cookies == "" {
			continue
		}
		p.cookies = cookies
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].dir == "Default" {
			return true
		}
		if out[j].dir == "Default" {
			return false
		}
		return out[i].display < out[j].display
	})
	return out
}

func chromiumCookiesPath(profileDir string) string {
	candidates := []string{
		filepath.Join(profileDir, "Network", "Cookies"),
		filepath.Join(profileDir, "Cookies"),
	}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() && st.Size() > 0 {
			return path
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func dedupeSources(sources []Source) []Source {
	seen := make(map[string]struct{}, len(sources))
	out := make([]Source, 0, len(sources))
	for _, src := range sources {
		key := src.dedupeKey()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, src)
	}
	return out
}
