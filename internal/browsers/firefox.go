package browsers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type firefoxProfile struct {
	dir     string
	display string
	cookies string
}

func firefoxProfiles(root string) []firefoxProfile {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil
	}

	parsed := parseFirefoxProfilesIni(filepath.Join(root, "profiles.ini"), root)
	if len(parsed) > 0 {
		return parsed
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []firefoxProfile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		cookies := filepath.Join(dir, "cookies.sqlite")
		if !fileExists(cookies) {
			continue
		}
		out = append(out, firefoxProfile{
			dir:     entry.Name(),
			display: entry.Name(),
			cookies: cookies,
		})
	}
	return out
}

func parseFirefoxProfilesIni(iniPath, root string) []firefoxProfile {
	f, err := os.Open(iniPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	type pending struct {
		name       string
		path       string
		isRelative string
	}
	var current pending
	var found []pending
	flush := func() {
		if strings.TrimSpace(current.path) != "" {
			found = append(found, current)
		}
		current = pending{}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "Name":
			current.name = value
		case "Path":
			current.path = value
		case "IsRelative":
			current.isRelative = value
		}
	}
	flush()

	var out []firefoxProfile
	for _, p := range found {
		dir := p.path
		if p.isRelative != "0" {
			dir = filepath.Join(root, p.path)
		}
		cookies := filepath.Join(dir, "cookies.sqlite")
		if !fileExists(cookies) {
			continue
		}
		display := p.name
		if display == "" {
			display = filepath.Base(dir)
		}
		out = append(out, firefoxProfile{
			dir:     filepath.Base(dir),
			display: display,
			cookies: cookies,
		})
	}
	return out
}

func extractFirefox(src Source) ([]Cookie, error) {
	if src.CookiesPath == "" || !fileExists(src.CookiesPath) {
		return nil, fmt.Errorf("firefox cookies not found for %s", src.Label())
	}

	tmp, err := os.MkdirTemp("", "chatbang-ff-cookies-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	copied := filepath.Join(tmp, "cookies.sqlite")
	if err := copyFile(src.CookiesPath, copied); err != nil {
		return nil, fmt.Errorf("copy firefox cookies: %w", err)
	}
	for _, side := range []string{"-wal", "-shm", "-journal"} {
		_ = copyIfExists(src.CookiesPath+side, copied+side)
	}

	return readFirefoxCookies(copied)
}
