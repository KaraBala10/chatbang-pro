package session

import (
	"strings"
	"testing"
)

func TestExtractSandboxPaths(t *testing.T) {
	text := "See [Download](sandbox:/mnt/data/count_to_100.txt) for the list."
	got := extractSandboxPaths(text)
	if len(got) != 1 || got[0] != "sandbox:/mnt/data/count_to_100.txt" {
		t.Fatalf("got %#v", got)
	}
	if extractSandboxPaths("sandbox:/mnt only partial") != nil {
		t.Fatal("expected partial sandbox path to be rejected")
	}
}

func TestParseConversationSSESandbox(t *testing.T) {
	raw := `data: {"message":{"id":"msg-abc","author":{"role":"assistant"},"content":{"content_type":"text","parts":["Here you go sandbox:/mnt/data/list.txt"]}}}`
	got := parseConversationSSE(raw)
	if got.MessageID != "msg-abc" {
		t.Fatalf("message id = %q", got.MessageID)
	}
	if len(got.SandboxPaths) != 1 || got.SandboxPaths[0] != "sandbox:/mnt/data/list.txt" {
		t.Fatalf("sandbox paths = %#v", got.SandboxPaths)
	}
}

func TestCleanFileAttachmentText(t *testing.T) {
	raw := `from pathlib import Path

path = Path("/mnt/data/hello_world.py") path.write_text('print("Hello, World!")\n', encoding="utf-[Download the Python
file](sandbox:/mnt/data/hello_world.py)`
	got := cleanFileAttachmentText(raw)
	if got != "" {
		t.Fatalf("expected empty after clean, got %q", got)
	}
	got = cleanFileAttachmentText("Here is your file.\npath = Path(\"/mnt/data/x.txt\")")
	if got != "Here is your file." {
		t.Fatalf("got %q", got)
	}
}

func TestFormatTurnFilesOnly(t *testing.T) {
	raw := `path = Path("/mnt/data/hello_world.py") path.write_text('print("Hello")')`
	out, err := formatTurn(raw, nil, []savedFile{{Path: "/tmp/hello_world.py"}})
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if strings.Contains(body, "pathlib") || strings.Contains(body, "write_text") {
		t.Fatalf("code leaked into output: %q", body)
	}
	if !strings.Contains(body, "[file] /tmp/hello_world.py") {
		t.Fatalf("missing file line: %q", body)
	}
}
