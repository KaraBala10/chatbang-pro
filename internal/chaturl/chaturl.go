package chaturl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	DefaultURL = "https://chatgpt.com"
	TempURL    = "https://chatgpt.com/?temporary-chat=true"
)

func NormalizeCustomGPT(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("custom GPT URL is empty")
	}

	switch {
	case strings.HasPrefix(input, "/g/"):
		input = "https://chatgpt.com" + input
	case strings.HasPrefix(input, "g-"):
		input = "https://chatgpt.com/g/" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("invalid custom GPT URL: %w", err)
	}
	if u.Scheme == "" {
		u, err = url.Parse("https://" + input)
		if err != nil {
			return "", fmt.Errorf("invalid custom GPT URL: %w", err)
		}
	}

	host := strings.ToLower(u.Hostname())
	if host != "chatgpt.com" && host != "www.chatgpt.com" {
		return "", fmt.Errorf("custom GPT URL must be on chatgpt.com")
	}

	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if !strings.HasPrefix(path, "/g/g-") {
		return "", fmt.Errorf("custom GPT URL must look like https://chatgpt.com/g/g-...")
	}

	result := "https://chatgpt.com" + path
	if u.Query().Get("temporary-chat") == "true" {
		result = WithTemporaryChat(result)
	}
	return result, nil
}

func WithTemporaryChat(chatURL string) string {
	u, err := url.Parse(chatURL)
	if err != nil {
		return chatURL
	}
	q := u.Query()
	q.Set("temporary-chat", "true")
	u.RawQuery = q.Encode()
	return u.String()
}

func IsTemporary(chatURL string) bool {
	u, err := url.Parse(chatURL)
	if err != nil {
		return false
	}
	return u.Query().Get("temporary-chat") == "true"
}

var conversationUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ConversationPermalink returns a shareable chatgpt.com conversation URL
// (https://chatgpt.com/c/{uuid} or a custom-GPT /c/ path). Empty if raw is not one.
func ConversationPermalink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "chatgpt.com", "www.chatgpt.com", "chat.openai.com":
	default:
		return ""
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	id := conversationIDFromPath(path)
	if id == "" {
		return ""
	}
	i := strings.LastIndex(path, "/c/")
	if i < 0 {
		return ""
	}
	return "https://chatgpt.com" + path[:i] + "/c/" + id
}

func conversationIDFromPath(path string) string {
	const marker = "/c/"
	i := strings.LastIndex(path, marker)
	if i < 0 {
		return ""
	}
	id := path[i+len(marker):]
	if j := strings.IndexByte(id, '/'); j >= 0 {
		id = id[:j]
	}
	id = strings.TrimSpace(id)
	if !conversationUUID.MatchString(id) {
		return ""
	}
	return strings.ToLower(id)
}

func CustomGPTPathPrefix(chatURL string) string {
	if !strings.Contains(chatURL, "/g/g-") {
		return ""
	}
	u, err := url.Parse(chatURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/")
}

func Resolve(temporaryChat bool, customGPT string) (string, error) {
	if customGPT != "" {
		resolved, err := NormalizeCustomGPT(customGPT)
		if err != nil {
			return "", err
		}
		if temporaryChat {
			return WithTemporaryChat(resolved), nil
		}
		return resolved, nil
	}
	if temporaryChat {
		return TempURL, nil
	}
	return DefaultURL, nil
}
