package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
)

type savedFile struct {
	Path string
	Sum  string
}

var (
	safeFilename         = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	sandboxPathRe        = regexp.MustCompile(`sandbox:/mnt/data/[^\s\)\]\>"']+`)
	sandboxMarkdownLink  = regexp.MustCompile(`(?i)\[[^\]]*\]\(\s*sandbox:/mnt/data/[^)]+\)`)
	sandboxBareLink      = regexp.MustCompile(`sandbox:/mnt/data/\S+`)
	filePlaceholder      = regexp.MustCompile(`\{\{file:([^}]+)\}\}`)
	codeInterpreterLine  = regexp.MustCompile(`(?i)(/mnt/data/|pathlib|write_text|open\s*\(|Path\s*\(|encoding\s*=\s*["']utf)`)
)

type fileMeta struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type assistantFileRefs struct {
	MessageID     string   `json:"messageId"`
	SandboxPaths  []string `json:"sandboxPaths"`
	FileIDs       []string `json:"fileIds"`
	DownloadURLs  []string `json:"downloadUrls"`
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func validSandboxPath(p string) bool {
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "sandbox:") {
		return false
	}
	rest := strings.TrimPrefix(p, "sandbox:")
	if !strings.HasPrefix(rest, "/mnt/data/") {
		return false
	}
	base := filepath.Base(rest)
	return base != "" && base != "." && base != "data" && base != "mnt"
}

func extractSandboxPaths(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	for _, m := range sandboxPathRe.FindAllString(text, -1) {
		if validSandboxPath(m) {
			out = append(out, strings.TrimSpace(m))
		}
	}
	return out
}

func extractFilePlaceholders(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	for _, m := range filePlaceholder.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}

// cleanFileAttachmentText removes code-interpreter boilerplate when a file was saved locally.
func cleanFileAttachmentText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = sandboxMarkdownLink.ReplaceAllString(text, "")
	text = sandboxBareLink.ReplaceAllString(text, "")
	text = filePlaceholder.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)

	if looksLikeCodeInterpreterOutput(text) {
		var kept []string
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if isCodeInterpreterNoise(trimmed) {
				continue
			}
			kept = append(kept, line)
		}
		text = strings.TrimSpace(strings.Join(kept, "\n"))
		if looksLikeCodeInterpreterOutput(text) {
			return ""
		}
	}
	return text
}

func looksLikeCodeInterpreterOutput(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "/mnt/data/") ||
		strings.Contains(low, "pathlib") ||
		strings.Contains(low, "write_text") ||
		strings.Contains(low, "sandbox:") ||
		codeInterpreterLine.MatchString(text)
}

func isCodeInterpreterNoise(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	low := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(low, "from pathlib"):
		return true
	case strings.HasPrefix(trimmed, "path = "):
		return true
	case strings.HasPrefix(trimmed, "import ") && strings.Contains(low, "path"):
		return true
	case codeInterpreterLine.MatchString(trimmed):
		return true
	case strings.Contains(low, "download the") && strings.Contains(low, "file"):
		return true
	default:
		return false
	}
}

func scrapeAssistantFileRefs(ctx context.Context) assistantFileRefs {
	js := `(() => {
		` + jsAssistantNodes + jsFileDownloadHelpers + `
		const root = chatbangAssistantRoot(nodes);
		if (!root) return { messageId: "", sandboxPaths: [], fileIds: [], downloadUrls: [] };
		const refs = { messageId: "", sandboxPaths: [], fileIds: [], downloadUrls: [] };
		const seenPath = new Set();
		const seenID = new Set();
		const seenURL = new Set();
		const validSandbox = (p) => {
			p = (p || "").trim();
			if (!p.startsWith("sandbox:/mnt/data/")) return false;
			const base = p.split("/").pop();
			return !!base && base !== "data" && base !== "mnt";
		};
		const addPath = (p) => {
			p = (p || "").trim();
			if (!validSandbox(p) || seenPath.has(p)) return;
			seenPath.add(p);
			refs.sandboxPaths.push(p);
		};
		const addID = (id) => {
			id = (id || "").trim();
			if (!id || seenID.has(id)) return;
			seenID.add(id);
			refs.fileIds.push(id);
		};
		const addURL = (u) => {
			u = (u || "").trim();
			if (!u || seenURL.has(u)) return;
			seenURL.add(u);
			refs.downloadUrls.push(u);
		};
		const ingestHref = (h) => {
			h = (h || "").trim();
			if (!h) return;
			if (h.includes("sandbox:")) addPath(h);
			let m = h.match(/\/backend-api\/files\/(?:download\/)?(file[-_][^/?#]+)/i);
			if (m) addID(m[1]);
			m = h.match(/[?&]id=(file_[^&]+)/i);
			if (m) addID(m[1]);
			if (h.includes("interpreter/download") || h.includes("estuary/content") || h.includes("/backend-api/files/")) {
				try { addURL(new URL(h, location.origin).href); } catch (e) {}
			}
			try {
				const u = new URL(h, location.origin);
				if (u.pathname.includes("interpreter/download")) {
					const mid = u.searchParams.get("message_id");
					if (mid && !refs.messageId) refs.messageId = mid;
					const sp = u.searchParams.get("sandbox_path");
					if (sp) addPath(sp.startsWith("sandbox:") ? sp : "sandbox:" + sp);
				}
			} catch (e) {}
		};
		let el = root;
		for (let i = 0; i < 8 && el; i++) {
			for (const attr of ["data-message-id", "data-messageId", "data-turn-id"]) {
				const v = el.getAttribute && el.getAttribute(attr);
				if (v && !refs.messageId) refs.messageId = v;
			}
			el = el.parentElement;
		}
		for (const { card } of chatbangFindDownloadTargets(root, "")) {
			chatbangHover(card);
			for (const u of chatbangExtractDownloadURLs(card)) addURL(u);
		}
		for (const btn of root.querySelectorAll('button[aria-label="Download file"]')) {
			const card = btn.closest("div[class*='group']") || btn.closest("li") || btn.parentElement?.parentElement;
			if (card) {
				chatbangHover(card);
				for (const u of chatbangExtractDownloadURLs(card)) addURL(u);
			}
		}
		const blob = (root.innerText || "") + "\n" + (root.innerHTML || "");
		for (const m of blob.matchAll(/sandbox:\/mnt\/data\/[^\s"'<>)\]]+/g)) addPath(m[0]);
		for (const m of blob.matchAll(/\{\{file:([^}]+)\}\}/g)) addID(m[1]);
		for (const m of blob.matchAll(/\/backend-api\/conversation\/[^"'\\s]+?\/interpreter\/download\?[^"'\\s<>]+/g)) {
			try { addURL(new URL(m[0], location.origin).href); } catch (e) {}
		}
		for (const m of blob.matchAll(/\/backend-api\/estuary\/content\?[^"'\\s<>]+/g)) {
			try { addURL(new URL(m[0], location.origin).href); } catch (e) {}
		}
		return refs;
	})()`
	var refs assistantFileRefs
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &refs))
	return refs
}

func pageConversationID(ctx context.Context) string {
	var loc string
	if err := chromedp.Run(ctx, chromedp.Location(&loc)); err != nil {
		return ""
	}
	if p := chaturl.ConversationPermalink(loc); p != "" {
		if i := strings.LastIndex(p, "/c/"); i >= 0 {
			return strings.TrimSuffix(p[i+3:], "/")
		}
	}
	return ""
}

func (s *Session) conversationID(ctx context.Context) string {
	if p := chaturl.ConversationPermalink(s.convURL); p != "" {
		if i := strings.LastIndex(p, "/c/"); i >= 0 {
			return strings.TrimSuffix(p[i+3:], "/")
		}
	}
	return pageConversationID(ctx)
}

func fetchFileMeta(ctx context.Context, fileID string) fileMeta {
	idJSON, err := json.Marshal(fileID)
	if err != nil {
		return fileMeta{}
	}
	js := fmt.Sprintf(`(async () => {
		%s
		const id = %s;
		const headers = await chatbangAuthHeaders();
		const paths = [
			"/backend-api/files/download/" + id + "?inline=false",
			"/backend-api/files/" + id,
		];
		for (const path of paths) {
			try {
				const r = await fetch(path, chatbangFetchOpts(headers));
				if (!r.ok) continue;
				const ct = (r.headers.get("content-type") || "").toLowerCase();
				if (ct.includes("json")) {
					const j = await r.json();
					const url = j.download_url || j.downloadUrl || j.url || "";
					const name = j.file_name || j.filename || j.name || "";
					if (url) return { url, name };
				}
			} catch (e) {}
		}
		try {
			const r = await fetch("/backend-api/files/" + id + "/download", { ...chatbangFetchOpts(headers), redirect: "follow" });
			if (r.ok && r.url) return { url: r.url, name: "" };
		} catch (e) {}
		return { url: "", name: "" };
	})()`, jsChatGPTAuth, idJSON)
	var meta fileMeta
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &meta, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	return meta
}

func fetchPageURLBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	uJSON, err := json.Marshal(rawURL)
	if err != nil {
		return nil, "", err
	}
	js := fmt.Sprintf(`(async () => {
		%s
		const url = %s;
		const headers = await chatbangAuthHeaders();
		const r = await fetch(url, chatbangFetchOpts(headers));
		if (!r.ok) return { error: "HTTP " + r.status };
		const ct = (r.headers.get("content-type") || "").toLowerCase();
		const buf = await r.arrayBuffer();
		const bytes = new Uint8Array(buf);
		if (bytes.length === 0) return { error: "empty body" };
		const toB64 = (arr) => {
			let bin = "";
			const chunk = 0x8000;
			for (let i = 0; i < arr.length; i += chunk) {
				bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
			}
			return btoa(bin);
		};
		const nameFromDisposition = () => {
			const disp = r.headers.get("content-disposition") || "";
			let m = disp.match(/filename\*=UTF-8''([^;]+)/i);
			if (m) return decodeURIComponent(m[1]);
			m = disp.match(/filename="?([^";]+)"?/i);
			return m ? m[1] : "";
		};
		if (ct.includes("json") || (bytes[0] === 0x7b && bytes[1] === 0x22)) {
			let j;
			try { j = JSON.parse(new TextDecoder().decode(bytes)); } catch (e) { j = null; }
			if (j) {
				const next = j.download_url || j.downloadUrl || j.url || j.signed_url || j.signedUrl || "";
				if (next) {
					const dl = await fetch(next, chatbangFetchOpts(headers));
					if (!dl.ok) return { error: "download HTTP " + dl.status };
					const body = new Uint8Array(await dl.arrayBuffer());
					if (!body.length) return { error: "empty download" };
					return { data: toB64(body), name: j.file_name || j.filename || j.name || nameFromDisposition() || "" };
				}
				const b64 = j.data || j.content || j.file_content || j.fileContent || "";
				if (b64) return { data: b64, name: j.file_name || j.filename || j.name || "" };
			}
			return { error: "no download_url" };
		}
		return { data: toB64(bytes), name: nameFromDisposition() || "" };
	})()`, jsChatGPTAuth, uJSON)
	var out struct {
		Data  string `json:"data"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	if out.Error != "" {
		return nil, "", fmt.Errorf("%s", out.Error)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("empty file body")
	}
	name := strings.TrimSpace(out.Name)
	return data, name, nil
}

func downloadSandboxFile(ctx context.Context, conversationID, messageID, sandboxPath string, downloadURLs []string) ([]byte, string, error) {
	sandboxPath = strings.TrimSpace(sandboxPath)
	conversationID = strings.TrimSpace(conversationID)
	messageID = strings.TrimSpace(messageID)
	path := strings.TrimPrefix(sandboxPath, "sandbox:")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	wantBase := strings.ToLower(filepath.Base(path))

	if b, name, err := downloadSandboxViaLatestCard(ctx, wantBase, path); err == nil && len(b) > 0 {
		if name == "" {
			name = filepath.Base(path)
		}
		return b, name, nil
	}

	for _, raw := range downloadURLs {
		if !urlMatchesSandbox(raw, wantBase, path) {
			continue
		}
		if b, name, err := fetchPageURLBytes(ctx, raw); err == nil && len(b) > 0 {
			if name == "" {
				name = filepath.Base(path)
			}
			return b, name, nil
		}
	}

	if b, name, err := fetchSandboxFromFileCard(ctx, wantBase, path); err == nil && len(b) > 0 {
		if name == "" {
			name = filepath.Base(path)
		}
		return b, name, nil
	}

	if conversationID != "" && messageID != "" {
		if b, name, err := downloadSandboxAPI(ctx, conversationID, messageID, path); err == nil && len(b) > 0 {
			return b, name, nil
		}
	}

	if b, name, err := downloadSandboxTryAllMessageIDs(ctx, path); err == nil && len(b) > 0 {
		if name == "" {
			name = filepath.Base(path)
		}
		return b, name, nil
	}

	return nil, "", fmt.Errorf("download failed")
}

func urlMatchesSandbox(rawURL, wantBase, sandboxPath string) bool {
	rawURL = strings.ToLower(rawURL)
	if wantBase != "" && strings.Contains(rawURL, wantBase) {
		return true
	}
	enc := strings.ToLower(strings.TrimPrefix(sandboxPath, "/"))
	return enc != "" && strings.Contains(rawURL, enc)
}

func downloadSandboxViaLatestCard(ctx context.Context, wantBase, sandboxPath string) ([]byte, string, error) {
	wantJSON, err := json.Marshal(wantBase)
	if err != nil {
		return nil, "", err
	}
	pathJSON, err := json.Marshal(sandboxPath)
	if err != nil {
		return nil, "", err
	}
	js := fmt.Sprintf(`(async () => {
		%s
		%s
		%s
		const want = %s.toLowerCase();
		const sandboxPath = %s;
		const root = chatbangAssistantRoot(nodes);
		if (!root) return { error: "no assistant root" };
		const headers = await chatbangAuthHeaders();
		const opts = chatbangFetchOpts(headers);
		const convId = (location.pathname.match(/\/c\/([^/?#]+)/) || [])[1] || "";
		const target = chatbangLatestDownloadTarget(root, want);
		const urls = [];
		const seen = new Set();
		const add = (u) => {
			u = (u || "").trim();
			if (!u || seen.has(u)) return;
			seen.add(u);
			urls.push(u);
		};
		if (target) {
			chatbangHover(target.card);
			for (const u of chatbangExtractDownloadURLs(target.card)) add(u);
			const mid = chatbangMessageIDFrom(target.card);
			if (convId && mid && sandboxPath) {
				add("/backend-api/conversation/" + convId + "/interpreter/download?message_id=" + encodeURIComponent(mid) + "&sandbox_path=" + encodeURIComponent(sandboxPath) + "&t=" + Date.now());
			}
		}
		const toB64 = (arr) => {
			let bin = "";
			const chunk = 0x8000;
			for (let i = 0; i < arr.length; i += chunk) {
				bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
			}
			return btoa(bin);
		};
		const nameFromDisposition = (r) => {
			const disp = r.headers.get("content-disposition") || "";
			let m = disp.match(/filename\*=UTF-8''([^;]+)/i);
			if (m) return decodeURIComponent(m[1]);
			m = disp.match(/filename="?([^";]+)"?/i);
			return m ? m[1] : "";
		};
		for (const raw of urls) {
			try {
				const url = new URL(raw, location.origin).href;
				const r = await fetch(url, opts);
				if (!r.ok) continue;
				const ct = (r.headers.get("content-type") || "").toLowerCase();
				const buf = await r.arrayBuffer();
				const bytes = new Uint8Array(buf);
				if (!bytes.length) continue;
				if (ct.includes("json") || (bytes[0] === 0x7b && bytes[1] === 0x22)) {
					let j;
					try { j = JSON.parse(new TextDecoder().decode(bytes)); } catch (e) { continue; }
					const next = chatbangFindDownloadURL(j);
					if (!next) continue;
					const dl = await fetch(next, opts);
					if (!dl.ok) continue;
					const body = new Uint8Array(await dl.arrayBuffer());
					if (!body.length) continue;
					return { data: toB64(body), name: j.file_name || j.filename || j.name || nameFromDisposition(dl) || want };
				}
				return { data: toB64(bytes), name: nameFromDisposition(r) || want };
			} catch (e) {}
		}
		return { error: "latest card fetch failed" };
	})()`, jsChatGPTAuth, jsFileDownloadHelpers, jsAssistantNodes, string(wantJSON), string(pathJSON))
	var out struct {
		Data  string `json:"data"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	if out.Error != "" {
		return nil, "", fmt.Errorf("%s", out.Error)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("empty file body")
	}
	return data, strings.TrimSpace(out.Name), nil
}

func fetchSandboxFromFileCard(ctx context.Context, wantBase, sandboxPath string) ([]byte, string, error) {
	wantJSON, err := json.Marshal(wantBase)
	if err != nil {
		return nil, "", err
	}
	pathJSON, err := json.Marshal(sandboxPath)
	if err != nil {
		return nil, "", err
	}
	js := fmt.Sprintf(`(async () => {
		%s
		%s
		%s
		const want = %s.toLowerCase();
		const sandboxPath = %s;
		const root = chatbangAssistantRoot(nodes);
		if (!root) return { error: "no assistant root" };
		const headers = await chatbangAuthHeaders();
		const opts = chatbangFetchOpts(headers);
		const urls = [];
		const seen = new Set();
		const add = (u) => {
			u = (u || "").trim();
			if (!u || seen.has(u)) return;
			seen.add(u);
			urls.push(u);
		};
		const target = chatbangLatestDownloadTarget(root, want);
		if (target) {
			chatbangHover(target.card);
			for (const u of chatbangExtractDownloadURLs(target.card)) add(u);
		}
		const convId = (location.pathname.match(/\/c\/([^/?#]+)/) || [])[1] || "";
		const messageIds = chatbangMessageIDs(root).slice().reverse();
		if (convId && sandboxPath) {
			for (const mid of messageIds) {
				add("/backend-api/conversation/" + convId + "/interpreter/download?message_id=" + encodeURIComponent(mid) + "&sandbox_path=" + encodeURIComponent(sandboxPath) + "&t=" + Date.now());
			}
		}
		const toB64 = (arr) => {
			let bin = "";
			const chunk = 0x8000;
			for (let i = 0; i < arr.length; i += chunk) {
				bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
			}
			return btoa(bin);
		};
		const nameFromDisposition = (r) => {
			const disp = r.headers.get("content-disposition") || "";
			let m = disp.match(/filename\\*=UTF-8''([^;]+)/i);
			if (m) return decodeURIComponent(m[1]);
			m = disp.match(/filename="?([^";]+)"?/i);
			return m ? m[1] : "";
		};
		for (const raw of urls) {
			try {
				const url = new URL(raw, location.origin).href;
				const r = await fetch(url, opts);
				if (!r.ok) continue;
				const ct = (r.headers.get("content-type") || "").toLowerCase();
				const buf = await r.arrayBuffer();
				const bytes = new Uint8Array(buf);
				if (!bytes.length) continue;
				if (ct.includes("json") || (bytes[0] === 0x7b && bytes[1] === 0x22)) {
					let j;
					try { j = JSON.parse(new TextDecoder().decode(bytes)); } catch (e) { continue; }
					const next = chatbangFindDownloadURL(j);
					if (next) {
						const dl = await fetch(next, chatbangFetchOpts(headers));
						if (!dl.ok) continue;
						const body = new Uint8Array(await dl.arrayBuffer());
						if (!body.length) continue;
						return { data: toB64(body), name: j.file_name || j.filename || j.name || nameFromDisposition(dl) || want };
					}
					continue;
				}
				return { data: toB64(bytes), name: nameFromDisposition(r) || want };
			} catch (e) {}
		}
		return { error: "no file card url" };
	})()`, jsChatGPTAuth, jsFileDownloadHelpers, jsAssistantNodes, string(wantJSON), string(pathJSON))
	var out struct {
		Data  string `json:"data"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	if out.Error != "" {
		return nil, "", fmt.Errorf("%s", out.Error)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("empty file body")
	}
	return data, strings.TrimSpace(out.Name), nil
}

func downloadSandboxAPI(ctx context.Context, conversationID, messageID, path string) ([]byte, string, error) {
	args, err := json.Marshal([]string{conversationID, messageID, path})
	if err != nil {
		return nil, "", err
	}
	js := fmt.Sprintf(`(async () => {
		%s
		const [conversationId, messageId, sandboxPath] = %s;
		const headers = await chatbangAuthHeaders();
		const params = new URLSearchParams({ message_id: messageId, sandbox_path: sandboxPath });
		const r = await fetch("/backend-api/conversation/" + conversationId + "/interpreter/download?" + params.toString() + "&t=" + Date.now(), chatbangFetchOpts(headers));
		if (!r.ok) return { error: "HTTP " + r.status };
		const ct = (r.headers.get("content-type") || "").toLowerCase();
		const buf = await r.arrayBuffer();
		const bytes = new Uint8Array(buf);
		if (bytes.length === 0) return { error: "empty body" };
		const toB64 = (arr) => {
			let bin = "";
			const chunk = 0x8000;
			for (let i = 0; i < arr.length; i += chunk) {
				bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
			}
			return btoa(bin);
		};
		const nameFromDisposition = () => {
			const disp = r.headers.get("content-disposition") || "";
			let m = disp.match(/filename\*=UTF-8''([^;]+)/i);
			if (m) return decodeURIComponent(m[1]);
			m = disp.match(/filename="?([^";]+)"?/i);
			return m ? m[1] : "";
		};
		if (ct.includes("json") || (bytes[0] === 0x7b && bytes[1] === 0x22)) {
			let j;
			try { j = JSON.parse(new TextDecoder().decode(bytes)); } catch (e) { j = null; }
			if (j) {
				const url = chatbangFindDownloadURL(j);
				if (url) {
					const dl = await fetch(url, chatbangFetchOpts(headers));
					if (!dl.ok) return { error: "download HTTP " + dl.status };
					const body = new Uint8Array(await dl.arrayBuffer());
					if (!body.length) return { error: "empty download" };
					return { data: toB64(body), name: j.file_name || j.filename || j.name || nameFromDisposition() || "" };
				}
				const b64 = j.data || j.content || j.file_content || j.fileContent || "";
				if (b64) return { data: b64, name: j.file_name || j.filename || j.name || "" };
			}
			return { error: "no download_url" };
		}
		return { data: toB64(bytes), name: nameFromDisposition() || "" };
	})()`, jsChatGPTAuth, string(args))
	var out struct {
		Data  string `json:"data"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	if out.Error != "" {
		return nil, "", fmt.Errorf("%s", out.Error)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("empty sandbox file")
	}
	name := strings.TrimSpace(out.Name)
	if name == "" {
		name = filepath.Base(path)
	}
	return data, name, nil
}

func downloadSandboxTryAllMessageIDs(ctx context.Context, sandboxPath string) ([]byte, string, error) {
	pathJSON, err := json.Marshal(sandboxPath)
	if err != nil {
		return nil, "", err
	}
	js := fmt.Sprintf(`(async () => {
		%s
		%s
		%s
		const sandboxPath = %s;
		const root = chatbangAssistantRoot(nodes);
		if (!root) return { error: "no assistant root" };
		const convId = (location.pathname.match(/\/c\/([^/?#]+)/) || [])[1] || "";
		if (!convId) return { error: "no conversation id" };
		const headers = await chatbangAuthHeaders();
		const toB64 = (arr) => {
			let bin = "";
			const chunk = 0x8000;
			for (let i = 0; i < arr.length; i += chunk) {
				bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
			}
			return btoa(bin);
		};
		const ids = chatbangMessageIDs(root).slice().reverse();
		for (const mid of ids) {
			try {
				const params = new URLSearchParams({ message_id: mid, sandbox_path: sandboxPath });
				const r = await fetch("/backend-api/conversation/" + convId + "/interpreter/download?" + params.toString() + "&t=" + Date.now(), chatbangFetchOpts(headers));
				if (!r.ok) continue;
				const ct = (r.headers.get("content-type") || "").toLowerCase();
				const buf = await r.arrayBuffer();
				const bytes = new Uint8Array(buf);
				if (!bytes.length) continue;
				if (ct.includes("json") || (bytes[0] === 0x7b && bytes[1] === 0x22)) {
					let j;
					try { j = JSON.parse(new TextDecoder().decode(bytes)); } catch (e) { continue; }
					const next = chatbangFindDownloadURL(j);
					if (!next) continue;
					const dl = await fetch(next, chatbangFetchOpts(headers));
					if (!dl.ok) continue;
					const body = new Uint8Array(await dl.arrayBuffer());
					if (!body.length) continue;
					return { data: toB64(body), name: j.file_name || j.filename || j.name || "" };
				}
				return { data: toB64(bytes), name: chatbangNameFromDisposition(r) || "" };
			} catch (e) {}
		}
		return { error: "no message id worked" };
	})()`, jsChatGPTAuth, jsFileDownloadHelpers, jsAssistantNodes, string(pathJSON))
	var out struct {
		Data  string `json:"data"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	if out.Error != "" {
		return nil, "", fmt.Errorf("%s", out.Error)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("empty sandbox file")
	}
	name := strings.TrimSpace(out.Name)
	if name == "" {
		name = filepath.Base(sandboxPath)
	}
	return data, name, nil
}

func downloadFileBytes(ctx context.Context, fileID string) ([]byte, string, error) {
	meta := fetchFileMeta(ctx, fileID)
	if meta.URL == "" {
		return nil, "", fmt.Errorf("no download url for %s", fileID)
	}
	b, err := downloadHTTP(meta.URL)
	if err != nil || len(b) == 0 {
		b = fetchURLViaPage(ctx, meta.URL)
	}
	if len(b) == 0 {
		return nil, "", fmt.Errorf("empty file body for %s", fileID)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = fileID + fileExt(b)
	}
	return b, name, nil
}

func fileExt(b []byte) string {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return ".txt"
	}
	switch {
	case bytesLooksLikeText(b):
		return ".txt"
	default:
		return ".bin"
	}
}

func bytesLooksLikeText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	limit := len(b)
	if limit > 512 {
		limit = 512
	}
	for _, c := range b[:limit] {
		if c == 0 {
			return false
		}
		if c < 9 || (c > 13 && c < 32 && c != 27) {
			return false
		}
	}
	return true
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = safeFilename.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._")
	if name == "" {
		return "download"
	}
	return name
}

func saveFileBytes(dir string, b []byte, name string, seen map[string]bool) (savedFile, bool, error) {
	if len(b) == 0 || dir == "" {
		return savedFile{}, false, nil
	}
	sum := imageSum(b)
	if seen[sum] {
		return savedFile{}, false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return savedFile{}, false, err
	}
	base := sanitizeFilename(name)
	if ext := filepath.Ext(base); ext == "" {
		base += fileExt(b)
	}
	stamp := time.Now().Format("20060102-150405")
	path := filepath.Join(dir, stamp+"-"+sum[:8]+"-"+base)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return savedFile{}, false, err
	}
	seen[sum] = true
	return savedFile{Path: path, Sum: sum}, true, nil
}

func (s *Session) collectFiles(turn parsedTurn, replyText string) []savedFile {
	if s.filesDir == "" {
		return nil
	}
	var files []savedFile
	seen := map[string]bool{}
	seenID := map[string]bool{}
	seenSandbox := map[string]bool{}
	loggedErr := map[string]bool{}
	added, pending := s.tryCollectFiles(turn, replyText, seen, seenID, seenSandbox, loggedErr, &files)
	if added > 0 || !pending {
		return files
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(350 * time.Millisecond)
		added, pending = s.tryCollectFiles(turn, replyText, seen, seenID, seenSandbox, loggedErr, &files)
		if added > 0 || !pending {
			break
		}
	}
	return files
}

func (s *Session) tryCollectFiles(turn parsedTurn, replyText string, seen, seenID, seenSandbox, loggedErr map[string]bool, files *[]savedFile) (int, bool) {
	var added int
	save := func(b []byte, name string) {
		if isImageBytes(b) {
			return
		}
		f, ok, err := saveFileBytes(s.filesDir, b, name, seen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not save file: %v\n", err)
			return
		}
		if ok {
			*files = append(*files, f)
			added++
			fmt.Fprintf(os.Stderr, "Saved file: %s\n", f.Path)
		}
	}

	refs := scrapeAssistantFileRefs(s.ctx)
	messageID := turn.MessageID
	if messageID == "" {
		messageID = refs.MessageID
	}
	convID := s.conversationID(s.ctx)
	downloadURLs := uniqueStrings(refs.DownloadURLs)

	sandboxPaths := uniqueStrings(append(append(turn.SandboxPaths, refs.SandboxPaths...), extractSandboxPaths(replyText)...))
	fileIDs := uniqueStrings(append(append(turn.AssetIDs, refs.FileIDs...), extractFilePlaceholders(replyText)...))
	pending := len(sandboxPaths) > 0 || len(fileIDs) > 0 || len(downloadURLs) > 0

	for _, raw := range downloadURLs {
		if seenSandbox[raw] {
			continue
		}
		seenSandbox[raw] = true
		b, name, err := fetchPageURLBytes(s.ctx, raw)
		if err != nil {
			continue
		}
		save(b, name)
	}

	for _, id := range fileIDs {
		if seenID[id] {
			continue
		}
		seenID[id] = true
		b, name, err := downloadFileBytes(s.ctx, id)
		if err != nil {
			continue
		}
		save(b, name)
	}
	for _, sp := range sandboxPaths {
		if !validSandboxPath(sp) || seenSandbox[sp] {
			continue
		}
		seenSandbox[sp] = true
		b, name, err := downloadSandboxFile(s.ctx, convID, messageID, sp, downloadURLs)
		if err != nil {
			if !loggedErr[sp] {
				loggedErr[sp] = true
				fmt.Fprintf(os.Stderr, "Could not download sandbox file %s: %v\n", sp, err)
			}
			continue
		}
		save(b, name)
	}
	return added, pending && added == 0
}

func isImageBytes(b []byte) bool {
	switch imageExt(b) {
	case ".png", ".jpg", ".webp", ".gif":
		return true
	default:
		return false
	}
}
