package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type savedImage struct {
	Path string
	Sum  string
}

func isStaticImageURL(raw string) bool {
	u := strings.ToLower(raw)
	for _, n := range []string{"favicon", "/_next/", "sprite", "icon-", "emoji", "avatar"} {
		if strings.Contains(u, n) {
			return true
		}
	}
	return false
}

func isLikelyGeneratedImage(raw, mime string) bool {
	if isStaticImageURL(raw) {
		return false
	}
	if isGeneratedImageURL(raw) || strings.HasPrefix(raw, "blob:") {
		return true
	}
	m := strings.ToLower(mime)
	return strings.HasPrefix(m, "image/png") || strings.HasPrefix(m, "image/jpeg") || strings.HasPrefix(m, "image/webp") || strings.HasPrefix(m, "image/gif")
}

func isGeneratedImageURL(raw string) bool {
	u := strings.ToLower(strings.TrimSpace(raw))
	if u == "" {
		return false
	}
	switch {
	case strings.Contains(u, "oaiusercontent"):
		return true
	case strings.Contains(u, "dalle"):
		return true
	case strings.Contains(u, "/backend-api/estuary/"):
		return true
	case strings.Contains(u, "estuary/content"):
		return true
	case strings.Contains(u, "/backend-api/files/"):
		return true
	default:
		return false
	}
}

func fileIDFromPointer(p string) string {
	p = strings.TrimSpace(p)
	i := strings.LastIndex(p, "file-")
	if i < 0 {
		return ""
	}
	id := p[i:]
	if j := strings.IndexAny(id, "?#&"); j >= 0 {
		id = id[:j]
	}
	id = strings.TrimSuffix(id, "/")
	if !strings.HasPrefix(id, "file-") {
		return ""
	}
	return id
}

func imageExt(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0x89, 0x50, 0x4E, 0x47}):
		return ".png"
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return ".webp"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return ".gif"
	default:
		return ".img"
	}
}

func imageSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func existingSavedImage(dir, sum string) string {
	if dir == "" || len(sum) < 8 {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*-"+sum[:8]+"*"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func isValidImageBytes(b []byte) bool {
	return imageExt(b) != ".img"
}

func saveImageBytes(dir string, b []byte, seen map[string]bool) (savedImage, bool, error) {
	if len(b) < minGeneratedImageBytes || !isValidImageBytes(b) {
		return savedImage{}, false, nil
	}
	sum := imageSum(b)
	if seen[sum] {
		return savedImage{}, false, nil
	}
	if existing := existingSavedImage(dir, sum); existing != "" {
		seen[sum] = true
		return savedImage{Path: existing, Sum: sum}, false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return savedImage{}, false, err
	}
	name := time.Now().Format("20060102-150405") + "-" + sum[:8] + imageExt(b)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return savedImage{}, false, err
	}
	seen[sum] = true
	return savedImage{Path: path, Sum: sum}, true, nil
}

func imageOpenCommand(path string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path)
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", path)
		}
		for _, bin := range []string{"gio", "eog", "feh", "gwenview", "loupe", "ristretto"} {
			if _, err := exec.LookPath(bin); err == nil {
				if bin == "gio" {
					return exec.Command("gio", "open", path)
				}
				return exec.Command(bin, path)
			}
		}
		return nil
	}
}

func openSavedImage(path string) {
	cmd := imageOpenCommand(path)
	if cmd == nil {
		fmt.Fprintln(os.Stderr, "Saved image, but no image viewer was found.")
		return
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open image: %v\n", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func decodeDataImage(raw string) []byte {
	if !strings.HasPrefix(raw, "data:image/") || strings.Contains(raw, "svg") {
		return nil
	}
	i := strings.Index(raw, ",")
	if i < 0 {
		return nil
	}
	if !strings.Contains(strings.ToLower(raw[:i]), "base64") {
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(raw[i+1:])
	if err != nil {
		return nil
	}
	return b
}

func keepFullerText(primary, copied string) string {
	a := visibleAssistantText(primary)
	b := visibleAssistantText(copied)
	switch {
	case b == "":
		return a
	case a == "":
		return b
	case len(b) > len(a):
		return b
	case strings.Contains(b, a):
		return b
	default:
		return a
	}
}

func needsCopyMerge(text string, after responseStatus) bool {
	text = visibleAssistantText(text)
	if text == "" {
		return true
	}
	if imageGenFailed(after) || isImageGenFailureText(text) {
		return true
	}
	return len(text) < copyNotRequiredMinLen
}

func visibleAssistantText(text string) string {
	t := stripLeadingReplyChrome(strings.TrimSpace(text))
	if t == "" {
		return ""
	}
	lines := strings.Split(t, "\n")
	for len(lines) > 0 && isStatusChrome(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && isStatusChrome(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	t = strings.TrimSpace(strings.Join(lines, "\n"))
	t = stripImageGenJSON(t)
	t = cleanImageGenFailureText(t)
	if t == "" || isStatusChrome(t) {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(t), "chatgpt can make mistakes") {
		return ""
	}
	return t
}

func stripImageGenJSON(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	if strings.Contains(t, `"transparent_background"`) || strings.Contains(t, `"is_style_transfer"`) {
		if strings.HasPrefix(t, "{") {
			return ""
		}
	}
	return t
}

var imageWorkedForPrefix = regexp.MustCompile(`(?i)^(worked for|thought for|thinking for)\s+\d+\s*(s|sec|secs|seconds|m|min|mins|minutes)\.?\b\s*`)
var imageWorkedForLine = regexp.MustCompile(`(?i)^(worked for|thought for|thinking for)\s+\d+\s*(s|sec|secs|seconds|m|min|mins|minutes)\.?$`)

func foldQuotes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\u2018' || r == '\u2019' {
			return '\''
		}
		return r
	}, s)
}

func cleanImageGenFailureText(s string) string {
	t := foldQuotes(strings.TrimSpace(s))
	if t == "" || !isImageGenFailureText(t) {
		return t
	}
	t = strings.Join(strings.Fields(t), " ")
	t = strings.TrimSpace(imageWorkedForPrefix.ReplaceAllString(t, ""))
	low := strings.ToLower(t)
	cut := -1
	for _, start := range []string{"we're so sorry", "we are so sorry", "we are sorry"} {
		if i := strings.Index(low, start); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut > 0 {
		t = strings.TrimSpace(t[cut:])
	}
	return t
}

func stripLeadingReplyChrome(s string) string {
	t := strings.TrimSpace(s)
	for {
		low := strings.ToLower(t)
		switch {
		case strings.HasPrefix(low, "chatgpt said:"):
			t = strings.TrimSpace(t[len("chatgpt said:"):])
		case strings.HasPrefix(low, "stopping thinking"):
			t = strings.TrimSpace(t[len("stopping thinking"):])
		case strings.HasPrefix(low, "thinking"):
			t = strings.TrimSpace(t[len("thinking"):])
		default:
			return t
		}
	}
}

func isStatusChrome(text string) bool {
	t := strings.ToLower(strings.Join(strings.Fields(text), " "))
	t = strings.TrimPrefix(t, "chatgpt said:")
	t = strings.TrimSpace(t)
	t = strings.TrimRight(t, ".…")
	if imageWorkedForLine.MatchString(t) {
		return true
	}
	switch t {
	case "", "thinking", "stopping thinking", "edit", "share", "like", "copy", "searching", "working", "analyzing",
		"generating", "generating image", "searching the web", "chatgpt said":
		return true
	default:
		return false
	}
}

func looksLikeImagePrompt(prompt string) bool {
	p := strings.ToLower(prompt)
	for _, k := range []string{"image", "picture", "photo", "dall-e", "dalle", "background", "transparent", "صورة", "ارسم", "رسمة", "خلفية"} {
		if strings.Contains(p, k) {
			return true
		}
	}
	return false
}

func imageGenFailed(status responseStatus) bool {
	return status.ImageFailed || isImageGenFailureText(status.Tail) || isImageGenFailureText(status.StatusLine)
}

func isImageGenFailureText(s string) bool {
	l := strings.ToLower(foldQuotes(s))
	for _, k := range []string{
		"guardrail", "third-party content", "violate our",
		"we're so sorry", "we are so sorry",
		"couldn't create", "could not create",
		"couldn't generate", "could not generate",
		"unable to generate", "against our policies",
		"content policy",
		"image generation isn't available", "image generation is not available",
		"isn't available in this temporary chat", "not available in this temporary chat",
		"switch to a regular chat",
	} {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}

func shouldCollectImages(text string, turn parsedTurn, baseline, after responseStatus) bool {
	if imageGenFailed(after) || isImageGenFailureText(text) || isImageGenFailureText(turn.Text) {
		return false
	}
	return turn.HasImage ||
		newImageThisTurn(baseline, after) ||
		after.HasImage ||
		after.ImageCount > baseline.ImageCount
}

func nudgeImageGeneration(ctx context.Context) {
	const js = `(() => {
		if (document.querySelector('[data-testid="stop-button"]')) return 'sent';
		for (const el of document.querySelectorAll('button[aria-label]')) {
			const a = (el.getAttribute('aria-label') || '').toLowerCase();
			if (a.includes('stop streaming') || a.includes('stop generating')) return 'sent';
		}
		for (const el of document.querySelectorAll('button, [role="menuitem"]')) {
			const t = ((el.getAttribute('aria-label') || '') + ' ' + (el.textContent || '')).replace(/\s+/g, ' ').trim().toLowerCase();
			if (!t) continue;
			if (t.includes('create image') || t.includes('generate image') || t.includes('image generation')) {
				el.click();
				return 'tool';
			}
		}
		const submit = document.querySelector('#composer-submit-button');
		if (submit && !submit.disabled && submit.getAttribute('data-testid') !== 'stop-button') {
			submit.click();
			return 'resubmit';
		}
		return 'idle';
	})()`
	for i := 0; i < 4; i++ {
		var step string
		_ = chromedp.Run(ctx, chromedp.Evaluate(js, &step), chromedp.Sleep(700*time.Millisecond))
		if step == "sent" {
			return
		}
	}
}

func scrapeAssistantImageURLs(ctx context.Context) []string {
	js := `(() => {
		` + jsImageHelpers + `
		` + jsAssistantNodes + `
		const last = nodes.length ? nodes[nodes.length - 1] : null;
		return imageSrcs(imageRoot(last));
	})()`
	var urls []string
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &urls))
	return urls
}

func downloadHTTP(rawURL string) ([]byte, error) {
	if !isGeneratedImageURL(rawURL) {
		return nil, fmt.Errorf("skip url")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 20<<20))
}

func fetchFileDownloadURL(ctx context.Context, fileID string) string {
	idJSON, err := json.Marshal(fileID)
	if err != nil {
		return ""
	}
	js := fmt.Sprintf(`(async () => {
		const id = %s;
		try {
			const r = await fetch("/backend-api/files/" + id, { credentials: "include" });
			if (r.ok) {
				const j = await r.json();
				return j.download_url || j.downloadUrl || j.url || "";
			}
		} catch (e) {}
		try {
			const r = await fetch("/backend-api/files/" + id + "/download", { credentials: "include", redirect: "follow" });
			if (r.ok) return r.url || "";
		} catch (e) {}
		return "";
	})()`, idJSON)
	var out string
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &out))
	return out
}

func fetchURLViaPage(ctx context.Context, rawURL string) []byte {
	uJSON, err := json.Marshal(rawURL)
	if err != nil {
		return nil
	}
	js := fmt.Sprintf(`(async () => {
		const url = %s;
		const r = await fetch(url, { credentials: "include" });
		if (!r.ok) return "";
		const buf = await r.arrayBuffer();
		const bytes = new Uint8Array(buf);
		let bin = "";
		const chunk = 0x8000;
		for (let i = 0; i < bytes.length; i += chunk) {
			bin += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
		}
		return btoa(bin);
	})()`, uJSON)
	var b64 string
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &b64)); err != nil || b64 == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	return data
}

func formatTurn(text string, images []savedImage, files []savedFile) ([]byte, error) {
	var b strings.Builder
	text = visibleAssistantText(text)
	if len(files) > 0 {
		text = cleanFileAttachmentText(text)
	}
	if text != "" {
		rendered, err := renderResponse(text)
		if err != nil {
			return nil, err
		}
		b.Write(rendered)
	}
	for _, img := range images {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[image] %s\n", img.Path)
	}
	for _, f := range files {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[file] %s\n", f.Path)
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("empty response from ChatGPT")
	}
	return []byte(b.String()), nil
}
