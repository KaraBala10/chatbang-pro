package browsers

import "fmt"

// Kind is the cookie store type of a discovered browser profile.
type Kind int

const (
	KindChromium Kind = iota
	KindFirefox
)

// Source is an installed browser profile that may hold a ChatGPT session.
type Source struct {
	Name        string
	Profile     string
	ProfileDir  string
	ExecPath    string
	UserDataDir string
	CookiesPath string
	Kind        Kind
}

// Label is the text shown in the setup menu.
func (s Source) Label() string {
	profile := s.Profile
	if profile == "" {
		profile = s.ProfileDir
	}
	if profile == "" {
		return s.Name
	}
	return fmt.Sprintf("%s — %s", s.Name, profile)
}

func (s Source) dedupeKey() string {
	if s.CookiesPath != "" {
		return s.CookiesPath
	}
	return s.UserDataDir + "\x00" + s.ProfileDir
}
