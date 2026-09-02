package session

import (
	"os"
	"strings"

	markdown "github.com/MichaelMure/go-term-markdown"
	"golang.org/x/term"
)

const (
	replyOpen  = "<<<chatbang-pro"
	replyClose = ">>>"
)

func stdoutIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func replyWidth() int {
	w := 80
	if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw >= 40 {
		w = tw
	}
	if w > 120 {
		w = 120
	}
	return w
}

func renderTerminalMarkdown(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var mdLines, imageLines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "[image] ") {
			imageLines = append(imageLines, line)
			continue
		}
		mdLines = append(mdLines, line)
	}
	text := strings.TrimRight(strings.Join(mdLines, "\n"), "\n")
	out := text
	if strings.TrimSpace(text) != "" {
		if rendered := markdown.Render(text, replyWidth(), 0); len(rendered) > 0 {
			out = strings.TrimRight(string(rendered), "\n")
		}
	}
	if len(imageLines) == 0 {
		return out
	}
	if out != "" {
		out += "\n"
	}
	return out + strings.Join(imageLines, "\n")
}

// formatReplyBlock wraps the assistant reply between <<<chatbang-pro and >>>.
// On a terminal it renders Markdown (code fences, lists, …); when stdout is
// not a TTY it keeps the raw Markdown for scripts.
func formatReplyBlock(body string) string {
	body = strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	inner := body
	if stdoutIsTTY() {
		inner = renderTerminalMarkdown(body)
	}
	inner = strings.TrimRight(inner, "\n")
	var b strings.Builder
	b.WriteString(replyOpen)
	b.WriteByte('\n')
	if inner != "" {
		b.WriteString(inner)
		b.WriteByte('\n')
	}
	b.WriteString(replyClose)
	b.WriteByte('\n')
	return b.String()
}

func parseReplyBlock(s string) (string, bool) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	start := strings.Index(s, replyOpen+"\n")
	if start < 0 {
		return "", false
	}
	s = s[start+len(replyOpen)+1:]
	if strings.HasPrefix(s, replyClose) {
		return "", true
	}
	end := strings.Index(s, "\n"+replyClose)
	if end < 0 {
		if strings.HasSuffix(s, replyClose) {
			end = len(s) - len(replyClose)
			if end > 0 && s[end-1] == '\n' {
				s = s[:end-1]
			} else {
				s = s[:end]
			}
		} else {
			return "", false
		}
	} else {
		s = s[:end]
	}
	if s == "" {
		return "", true
	}
	lines := strings.Split(s, "\n")
	prefixed := true
	for _, line := range lines {
		if line != "|" && !strings.HasPrefix(line, "| ") {
			prefixed = false
			break
		}
	}
	if !prefixed {
		return s, true
	}
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "| ") {
			out = append(out, line[2:])
		} else {
			out = append(out, "")
		}
	}
	return strings.Join(out, "\n"), true
}
