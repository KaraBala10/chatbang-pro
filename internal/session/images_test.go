package session

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileIDFromPointer(t *testing.T) {
	cases := map[string]string{
		"file-service://file-abc123": "file-abc123",
		"file-xyz":                   "file-xyz",
		"https://example.com/x":      "",
	}
	for in, want := range cases {
		if got := fileIDFromPointer(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestIsGeneratedImageURL(t *testing.T) {
	if !isGeneratedImageURL("https://chatgpt.com/backend-api/estuary/content?id=file_1") {
		t.Fatal("expected estuary url")
	}
	if !isGeneratedImageURL("https://files.oaiusercontent.com/file-1") {
		t.Fatal("expected oaiusercontent url")
	}
	if !looksLikeImagePrompt("ولد صورة لقطة في حديقة") {
		t.Fatal("arabic image prompt")
	}
	if !looksLikeImagePrompt("remove background") {
		t.Fatal("image edit follow-up")
	}
	if isGeneratedImageURL("https://chatgpt.com/favicon.ico") {
		t.Fatal("favicon should not count")
	}
}

func TestImageExt(t *testing.T) {
	if imageExt([]byte{0x89, 0x50, 0x4E, 0x47}) != ".png" {
		t.Fatal("png")
	}
	if imageExt([]byte{0xFF, 0xD8, 0xFF, 0xE0}) != ".jpg" {
		t.Fatal("jpg")
	}
}

func TestSaveImageBytes(t *testing.T) {
	dir := t.TempDir()
	png := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, minGeneratedImageBytes/8+1)
	seen := map[string]bool{}
	img, ok, err := saveImageBytes(dir, png, seen)
	if err != nil || !ok {
		t.Fatalf("save: ok=%v err=%v", ok, err)
	}
	if _, err := os.Stat(img.Path); err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(img.Path) != ".png" {
		t.Fatalf("ext %s", img.Path)
	}
	_, ok, err = saveImageBytes(dir, png, seen)
	if err != nil || ok {
		t.Fatal("duplicate should be skipped")
	}
}

func TestLiveImageGen(t *testing.T) {
	if os.Getenv("CHATBANG_LIVE_IMAGE") != "1" {
		t.Skip("set CHATBANG_LIVE_IMAGE=1 to try image generation")
	}
	browser := os.Getenv("CHATBANG_LIVE_BROWSER")
	profile := os.Getenv("CHATBANG_LIVE_PROFILE")
	images := os.Getenv("CHATBANG_LIVE_IMAGES")
	if browser == "" || profile == "" || images == "" {
		t.Fatal("CHATBANG_LIVE_BROWSER, CHATBANG_LIVE_PROFILE, and CHATBANG_LIVE_IMAGES are required")
	}
	sess, err := New(browser, profile, images, true, "https://chatgpt.com")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sess.Close)

	out, _, err := sess.runTurn("Generate an image of a cat sitting in a garden. Create the image now; do not ask questions.")
	t.Logf("err=%v out=%s", err, out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[image] ") {
		t.Fatalf("expected saved image path, got %q", out)
	}
}

func TestVisibleAssistantText(t *testing.T) {
	if visibleAssistantText("ChatGPT said:Edit") != "" {
		t.Fatal("image chrome should be ignored")
	}
	if visibleAssistantText("Thinking") != "" {
		t.Fatal("thinking chrome should be ignored")
	}
	if visibleAssistantText("ChatGPT said:Thinking") != "" {
		t.Fatal("thinking prefix should be ignored")
	}
	if visibleAssistantText("Hello") != "Hello" {
		t.Fatal("real text should stay")
	}
	got := visibleAssistantText("Stopping thinking\n\nI'm doing well, thanks for asking!\nChatGPT said:\nEdit")
	if got != "I'm doing well, thanks for asking!" {
		t.Fatalf("got %q", got)
	}
}

func TestStripImageGenJSON(t *testing.T) {
	raw := `{"size":"1024x1024","n":1,"transparent_background":false,"is_style_transfer":false,"prompt`
	if visibleAssistantText(raw) != "" {
		t.Fatalf("image-gen json should be dropped, got %q", visibleAssistantText(raw))
	}
	if visibleAssistantText("Hello") != "Hello" {
		t.Fatal("plain text should stay")
	}
}

func TestImageGenFailureText(t *testing.T) {
	msg := "We're so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt."
	if !isImageGenFailureText(msg) {
		t.Fatal("expected guardrail text")
	}
	if !imageGenFailed(responseStatus{ImageFailed: true, Tail: msg}) {
		t.Fatal("ImageFailed flag")
	}
	if isImageGenFailureText("Hello") {
		t.Fatal("plain hello is not a failure")
	}
	want := "We're so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt."
	cases := []string{
		"Worked for 51s We're so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt.",
		"Worked for 51s We’re so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt.",
		"Worked for 51s\n\nWe're so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt.",
		"ChatGPT said: Worked for 51s We're so sorry, but the image we created may violate our guardrails concerning similarity to third-party content. If you think we got it wrong, please retry or edit your prompt.",
	}
	for _, in := range cases {
		if got := visibleAssistantText(in); got != want {
			t.Fatalf("got %q want %q from %q", got, want, in)
		}
	}
}

func TestStatusKeyTreatsEllipsisAsSame(t *testing.T) {
	if statusKey("Generating image...") != statusKey("Generating image") {
		t.Fatal("ellipsis should not count as a new status")
	}
}

func TestImageOpenCommand(t *testing.T) {
	cmd := imageOpenCommand("/tmp/x.png")
	if cmd == nil {
		t.Skip("no image opener on this system")
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "/tmp/x.png") {
		t.Fatalf("opener args = %q", joined)
	}
}

func TestFormatTurnImageOnly(t *testing.T) {
	out, err := formatTurn("", []savedImage{{Path: "/tmp/cat.png"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[image] /tmp/cat.png") {
		t.Fatalf("got %q", out)
	}
}
