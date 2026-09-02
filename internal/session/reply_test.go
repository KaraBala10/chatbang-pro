package session

import (
	"strings"
	"testing"
)

func TestFormatReplyBlockRoundTripRaw(t *testing.T) {
	if stdoutIsTTY() {
		t.Skip("stdout is a TTY; this test checks the raw (non-TTY) block")
	}
	cases := []string{
		"pong",
		"Hello! 👋 كيفك؟",
		"line one\nline two",
		"[image] /tmp/cat.png",
		"caption\n[image] /tmp/cat.png",
		"```python\nprint(\"hi\")\n```",
		"",
	}
	for _, in := range cases {
		got, ok := parseReplyBlock(formatReplyBlock(in))
		if !ok {
			t.Fatalf("parse failed for %q", in)
		}
		if got != in {
			t.Fatalf("got %q want %q", got, in)
		}
	}
}

func TestFormatReplyBlockMarkers(t *testing.T) {
	out := formatReplyBlock("pong")
	if !strings.HasPrefix(out, "<<<chatbang-pro\n") || !strings.HasSuffix(out, ">>>\n") {
		t.Fatalf("got %q", out)
	}
	body, ok := parseReplyBlock(out)
	if !ok || !strings.Contains(body, "pong") {
		t.Fatalf("body = %q ok=%v", body, ok)
	}
}

func TestRenderTerminalMarkdownCode(t *testing.T) {
	in := "```python\nprint(\"Hello, world!\")\n```"
	got := renderTerminalMarkdown(in)
	if !strings.Contains(got, "print") {
		t.Fatalf("expected rendered code, got %q", got)
	}
	if strings.Contains(got, "| ```python") {
		t.Fatal("pipe-prefixed raw fence should not appear")
	}
}

func TestRenderTerminalMarkdownKeepsImageLine(t *testing.T) {
	got := renderTerminalMarkdown("hi\n[image] /tmp/cat.png")
	if !strings.Contains(got, "[image] /tmp/cat.png") {
		t.Fatalf("got %q", got)
	}
}
