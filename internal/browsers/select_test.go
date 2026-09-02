package browsers

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestPromptSelectBrowser(t *testing.T) {
	sources := []Source{
		{Name: "Firefox", Profile: "default"},
		{Name: "Brave", Profile: "Default"},
	}
	var out bytes.Buffer
	got, err := PromptSelect(&out, strings.NewReader("2\n"), sources)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Brave" {
		t.Fatalf("got %+v, want Brave", got)
	}
	text := out.String()
	if !strings.Contains(text, "1) Firefox — default") || !strings.Contains(text, "3) Log in manually in a new window") {
		t.Fatalf("unexpected menu:\n%s", text)
	}
}

func TestPromptSelectManual(t *testing.T) {
	sources := []Source{{Name: "Firefox", Profile: "default"}}
	got, err := PromptSelect(io.Discard, strings.NewReader("2\n"), sources)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected manual login, got %+v", got)
	}
}

func TestPromptSelectRetry(t *testing.T) {
	sources := []Source{{Name: "Firefox", Profile: "default"}}
	var out bytes.Buffer
	got, err := PromptSelect(&out, strings.NewReader("nope\n1\n"), sources)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Firefox" {
		t.Fatalf("got %+v, want Firefox", got)
	}
	if !strings.Contains(out.String(), "Please enter a number") {
		t.Fatalf("expected retry prompt, got:\n%s", out.String())
	}
}

func TestPromptSelectEmptySources(t *testing.T) {
	got, err := PromptSelect(io.Discard, strings.NewReader("1\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil source, got %+v", got)
	}
}
