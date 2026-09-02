package browsers

import "strings"

// Cookie is a decrypted browser cookie used to import a ChatGPT session.
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  float64
	Secure   bool
	HTTPOnly bool
	SameSite string
	Session  bool
}

var chatGPTHosts = []string{
	"chatgpt.com",
	"openai.com",
	"oaistatic.com",
	"chat.openai.com",
}

func hostMatches(domain, host string) bool {
	d := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	h := strings.ToLower(host)
	return d == h || strings.HasSuffix(d, "."+h)
}

// FilterChatGPT keeps cookies that belong to ChatGPT / OpenAI auth domains.
func FilterChatGPT(cookies []Cookie) []Cookie {
	out := make([]Cookie, 0, len(cookies))
	for _, c := range cookies {
		for _, host := range chatGPTHosts {
			if hostMatches(c.Domain, host) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// HasChatGPTSession reports whether cookies include a ChatGPT login token.
func HasChatGPTSession(cookies []Cookie) bool {
	for _, c := range cookies {
		if strings.TrimSpace(c.Value) == "" {
			continue
		}
		name := strings.ToLower(c.Name)
		if strings.Contains(name, "session-token") || name == "access_token" {
			return true
		}
	}
	return false
}
