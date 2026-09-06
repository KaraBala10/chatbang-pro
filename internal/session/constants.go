package session

import "time"

const (
	navTimeout             = 60 * time.Second
	pollIntervalActive     = 200 * time.Millisecond
	pollIntervalDone       = 100 * time.Millisecond
	stablePollsDefault     = 3
	stablePollsLarge       = 4
	confirmDelayDefault    = 250 * time.Millisecond
	confirmDelayLarge      = 400 * time.Millisecond
	replySettleWait        = 250 * time.Millisecond
	fastReplySettleWait    = 120 * time.Millisecond
	copyNotRequiredMinLen  = 80
	partialMinGap          = 15 * time.Second
	captureSeenTimeout     = 1500 * time.Millisecond
	startWaitTimeout       = 45 * time.Second
	imageSettleTimeout     = 90 * time.Second
	minGeneratedImageBytes = 8 * 1024
	sendAfterFillWait      = 400 * time.Millisecond
	composerIdleWait       = 800 * time.Millisecond
)

const jsAssistantNodes = `
		let nodes = document.querySelectorAll('section[data-testid^="conversation-turn"][data-turn="assistant"]');
		if (!nodes.length) nodes = document.querySelectorAll('section[data-turn="assistant"]');
		if (!nodes.length) nodes = document.querySelectorAll('article[data-turn="assistant"]');
		if (!nodes.length) nodes = document.querySelectorAll('[data-message-author-role="assistant"]');
		if (!nodes.length) nodes = document.querySelectorAll('[data-testid="assistant-message"]');`

const jsReplyChrome = `
		function isReplyChrome(text) {
			let t = (text || "").replace(/\s+/g, " ").trim().toLowerCase();
			if (!t) return true;
			if (t.startsWith("chatgpt said:")) t = t.slice(13).trim();
			t = t.replace(/^(stopping thinking|thinking)+/, "").trim();
			t = t.replace(/[.…]+$/, "");
			return t === "" || t === "thinking" || t === "stopping thinking" || t === "edit" || t === "share" || t === "like" || t === "copy" ||
				t === "searching" || t === "working" || t === "analyzing" || t === "generating" ||
				t === "generating image" || t === "searching the web";
		}`

const jsImageHelpers = `
		function isGeneratedSrc(src) {
			const u = (src || "").toLowerCase();
			if (!u || u.startsWith("data:image/svg")) return false;
			if (u.includes("favicon") || u.includes("/_next/") || u.includes("avatar") || u.includes("emoji") || u.includes("sprite")) return false;
			return u.includes("oaiusercontent") || u.includes("dalle") || u.includes("/backend-api/files/") || u.includes("/backend-api/estuary/") || u.includes("estuary/content") || u.startsWith("blob:") || (u.startsWith("data:image/") && u.includes("base64"));
		}
		function imageKey(src) {
			const u = src || "";
			let m = u.match(/[?&]id=(file_[^&]+)/i);
			if (m) return m[1];
			m = u.match(/(file[-_][A-Za-z0-9]+)/i);
			if (m) return m[1];
			return u.split("?")[0];
		}
		function imageRoot(node) {
			if (!node) return null;
			return node.closest('section[data-testid^="conversation-turn"]') || node.closest("article") || node;
		}
		function imageGenNodes(root) {
			if (!root) return [];
			const sel = '[class*="imagegen"], [data-testid*="image-gen"], [id^="image-"]';
			const out = [];
			const seen = new Set();
			const add = (el) => {
				if (!el || seen.has(el)) return;
				seen.add(el);
				out.push(el);
			};
			try {
				if (root.matches && root.matches(sel)) add(root);
			} catch (e) {}
			if (root.querySelectorAll) {
				for (const el of root.querySelectorAll(sel)) add(el);
			}
			return out;
		}
		function isImageGenDone(root) {
			if (!root) return false;
			if (root.querySelector('[aria-label="Edit image"], [aria-label="Share this image"], [data-testid="good-image-turn-action-button"], [data-testid="image-gen-overlay-actions"]')) return true;
			return false;
		}
		function imageGenBlockMessage(root) {
			if (!root) return "";
			const keys = [
				"guardrail", "third-party content", "violate our",
				"we're so sorry", "we are so sorry", "we are sorry",
				"couldn't create", "could not create", "couldn't generate",
				"could not generate", "unable to generate", "against our policies",
				"content policy", "can't generate this", "cannot generate this",
				"image generation isn't available", "image generation is not available",
				"isn't available in this temporary chat", "not available in this temporary chat",
				"switch to a regular chat"
			];
			let best = "";
			const consider = (t) => {
				t = (t || "").replace(/[\u2018\u2019]/g, "'").replace(/\s+/g, " ").trim();
				if (!t) return;
				const low = t.toLowerCase();
				if (!keys.some((k) => low.includes(k))) return;
				if (t.length > best.length) best = t;
			};
			consider(root.innerText || root.textContent || "");
			for (const el of imageGenNodes(root)) {
				consider(el.innerText || el.textContent || "");
			}
			best = best.replace(/[\u2018\u2019]/g, "'");
			best = best.replace(/^(worked for|thought for|thinking for)\s+\d+\s*(s|sec|secs|seconds|m|min|mins|minutes)\.?\b\s*/i, "").trim();
			const lowBest = best.toLowerCase();
			let cut = -1;
			for (const start of ["we're so sorry", "we are so sorry", "we are sorry"]) {
				const i = lowBest.indexOf(start);
				if (i >= 0 && (cut < 0 || i < cut)) cut = i;
			}
			if (cut > 0) best = best.slice(cut).trim();
			return best;
		}
		function isImageGenPending(root) {
			if (!root || isImageGenDone(root) || imageGenBlockMessage(root)) return false;
			return imageGenNodes(root).length > 0;
		}
		function imageGenStatusLine(root) {
			for (const el of imageGenNodes(root)) {
				if (isImageGenDone(el)) continue;
				for (const line of (el.innerText || "").split("\n")) {
					const s = line.trim();
					if (!s) continue;
					const l = s.toLowerCase().replace(/\s+/g, " ");
					if (l === "thinking" || l === "edit" || l === "share" || l === "like" || l === "copy") continue;
					return s;
				}
			}
			return isImageGenPending(root) ? "Generating image..." : "";
		}
		function assistantTextWithoutImageGen(node) {
			if (!node) return "";
			const clone = node.cloneNode(true);
			for (const el of imageGenNodes(clone)) el.remove();
			const t = (clone.textContent || "").replace(/\s+/g, " ").trim();
			const blocked = imageGenBlockMessage(imageRoot(node) || node);
			if (!blocked) return t;
			if (t.length >= blocked.length) return t;
			return blocked;
		}
		function imageSrcs(root) {
			if (!root) return [];
			const urls = [];
			const seen = new Set();
			const add = (u) => {
				if (!u || seen.has(u)) return;
				seen.add(u);
				urls.push(u);
			};
			for (const img of root.querySelectorAll("img")) {
				add(img.currentSrc || img.src || "");
			}
			for (const el of root.querySelectorAll("source[srcset], img[srcset]")) {
				const set = el.getAttribute("srcset") || "";
				for (const part of set.split(",")) {
					add(part.trim().split(/\s+/)[0]);
				}
			}
			for (const a of root.querySelectorAll("a[href*='oaiusercontent'], a[href*='dalle'], a[href*='estuary']")) {
				add(a.href);
			}
			return urls.filter(isGeneratedSrc);
		}
		function hasFinishedImage(root) {
			if (!root) return false;
			if (isImageGenDone(root)) {
				const urls = imageSrcs(root);
				if (urls.length) return true;
				for (const img of root.querySelectorAll("img")) {
					if ((img.naturalWidth || 0) >= 32 && (img.naturalHeight || 0) >= 32) return true;
				}
			}
			const urls = imageSrcs(root);
			if (urls.some((u) => u.includes("oaiusercontent") || u.includes("dalle") || u.includes("/backend-api/files/") || u.includes("estuary"))) {
				return true;
			}
			for (const img of root.querySelectorAll("img")) {
				const src = img.currentSrc || img.src || "";
				if (!isGeneratedSrc(src)) continue;
				if (img.naturalWidth >= 200 && img.naturalHeight >= 200) return true;
			}
			return false;
		}`

const jsUserNodes = `
		let userNodes = document.querySelectorAll('section[data-testid^="conversation-turn"][data-turn="user"]');
		if (!userNodes.length) userNodes = document.querySelectorAll('section[data-turn="user"]');
		if (!userNodes.length) userNodes = document.querySelectorAll('[data-message-author-role="user"]');
		if (!userNodes.length) userNodes = document.querySelectorAll('article[data-turn="user"]');`

const jsStopVisible = `
		function isStopVisible() {
			const dead = (b) => !b || b.disabled || b.getAttribute('aria-disabled') === 'true';
			const b = document.querySelector('[data-testid="stop-button"]');
			if (b && !dead(b)) return true;
			for (const el of document.querySelectorAll('button[aria-label]')) {
				if (dead(el)) continue;
				const a = (el.getAttribute('aria-label') || '').toLowerCase();
				if (a.includes('stop streaming') || a.includes('stop generating') || a.includes('stop answering')) return true;
			}
			return false;
		}`

const jsIsStreaming = `
		function isStillStreaming(node) {
			if (!node) return false;
			if (node.getAttribute('data-is-streaming') === 'true') return true;
			if (node.querySelector('[data-is-streaming="true"]')) return true;
			if (node.querySelector('.result-streaming')) return true;
			return false;
		}
		function hasTurnCopyButton(node) {
			if (!node) return false;
			const labeled = node.querySelector('[data-testid="copy-turn-action-button"], button[aria-label="Copy"], button[aria-label="Copy message"], button[title="Copy"]');
			if (labeled && !labeled.closest("pre, code")) return true;
			for (const b of node.querySelectorAll("button")) {
				if (b.closest("pre, code")) continue;
				const t = ((b.getAttribute("data-testid") || "") + " " + (b.getAttribute("aria-label") || "") + " " + (b.getAttribute("title") || "") + " " + (b.textContent || "")).toLowerCase();
				if (t.includes("copy-turn") || (/\bcopy\b/.test(t) && !t.includes("copied") && !t.includes("code"))) return true;
			}
			return false;
		}
		function isTurnThinking(node) {
			if (!node) return false;
			if (node.querySelector('[data-testid*="thinking"], [data-testid*="reasoning"]')) return true;
			const t = (node.innerText || "").replace(/\s+/g, " ").trim().toLowerCase();
			if (!t) return false;
			if (t === "thinking" || t.startsWith("thinking ") || t.startsWith("thinking…")) return true;
			if (t.includes("stopping thinking")) return true;
			return false;
		}`

const jsComposer = `
		function composerCandidates() {
			const seen = new Set();
			const out = [];
			const add = (el) => {
				if (!el || seen.has(el)) return;
				seen.add(el);
				out.push(el);
			};
			add(document.querySelector('#composer-submit-button'));
			add(document.querySelector('button[data-testid="send-button"]'));
			add(document.querySelector('button[aria-label="Send prompt"]'));
			add(document.querySelector('button[aria-label="Send"]'));
			add(document.querySelector('button[data-testid="stop-button"]'));
			for (const el of document.querySelectorAll('button[aria-label]')) {
				const a = (el.getAttribute('aria-label') || '').toLowerCase();
				if (a.includes('send prompt') || a === 'send') add(el);
			}
			return out;
		}
		function isStopControl(b) {
			if (!b) return false;
			if (b.disabled || b.getAttribute('aria-disabled') === 'true') return false;
			if ((b.getAttribute('data-testid') || '') === 'stop-button') return true;
			const a = (b.getAttribute('aria-label') || '').toLowerCase();
			return a.includes('stop answering') || a.includes('stop streaming') || a.includes('stop generating');
		}
		function isSendableControl(b) {
			if (!b) return false;
			if (b.disabled || b.getAttribute('aria-disabled') === 'true') return false;
			const t = (b.getAttribute('data-testid') || '').toLowerCase();
			const a = (b.getAttribute('aria-label') || '').toLowerCase();
			if (t === 'stop-button' || a.includes('stop')) return false;
			return true;
		}
		function composerButton() {
			const all = composerCandidates();
			for (const b of all) {
				if (isSendableControl(b)) return b;
			}
			for (const b of all) {
				if (isStopControl(b)) return b;
			}
			return all[0] || null;
		}
		function composerText() {
			const el = document.querySelector('#prompt-textarea');
			return ((el && (el.innerText || el.value || el.textContent)) || '').replace(/\u200b/g, '').trim();
		}
		function composerCanAcceptPrompt() {
			if (!document.querySelector('#prompt-textarea')) return false;
			return !isStopControl(composerButton());
		}
		function setComposerText(text) {
			const el = document.querySelector('#prompt-textarea');
			if (!el) return '';
			el.focus();
			if (el.tagName === 'TEXTAREA' || el.tagName === 'INPUT') {
				const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
				const desc = Object.getOwnPropertyDescriptor(proto, 'value');
				if (desc && desc.set) desc.set.call(el, text); else el.value = text;
				el.dispatchEvent(new Event('input', { bubbles: true }));
				return composerText();
			}
			const sel = window.getSelection();
			const range = document.createRange();
			range.selectNodeContents(el);
			sel.removeAllRanges();
			sel.addRange(range);
			document.execCommand('selectAll', false, null);
			document.execCommand('insertText', false, text);
			const shown = composerText();
			if (text && !shown) {
				el.textContent = text;
				el.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: text }));
			}
			return composerText();
		}
		function clickSend() {
			const b = composerButton();
			if (!isSendableControl(b)) return false;
			b.click();
			return true;
		}`

const jsDismissDialogs = `
		function rateLimitDismissButton() {
			const keys = [
				"too many requests", "requests too quickly", "temporarily limited",
				"limited access to your conversations", "high demand", "at capacity",
				"usage limit", "rate limit", "you've reached", "try again later",
				"please wait a few minutes", "making requests too quickly"
			];
			const okLabel = (t) => {
				t = (t || "").replace(/\s+/g, " ").trim().toLowerCase();
				return t === "got it" || t === "ok" || t === "okay" || t === "close" || t === "dismiss" || t === "continue";
			};
			for (const d of document.querySelectorAll('[role="dialog"], [role="alertdialog"]')) {
				const state = (d.getAttribute("data-state") || "open").toLowerCase();
				if (state === "closed" || state === "hidden") continue;
				const text = (d.innerText || d.textContent || "").toLowerCase();
				const buttons = [...d.querySelectorAll("button")];
				const gotIt = buttons.find((b) => okLabel(b.innerText || b.textContent));
				if (gotIt && (keys.some((k) => text.includes(k)) || okLabel(gotIt.innerText || gotIt.textContent))) {
					return gotIt;
				}
				if (!keys.some((k) => text.includes(k))) continue;
				return buttons.find((b) => okLabel(b.innerText || b.textContent))
					|| buttons.find((b) => ((b.className || "").toString().includes("btn-primary")))
					|| buttons[buttons.length - 1]
					|| null;
			}
			return null;
		}`

const jsClipboardHookInner = `
		window.__chatbangCopied = window.__chatbangCopied || "";
		const grab = (text) => { window.__chatbangCopied = String(text ?? ""); };
		const writeText = function(text) { grab(text); return Promise.resolve(); };
		const write = function(items) {
			try {
				const item = items && items[0];
				if (item && item.getType) item.getType("text/plain").then((b) => b.text()).then(grab).catch(() => {});
			} catch (e) {}
			return Promise.resolve();
		};
		try {
			if (window.Clipboard && Clipboard.prototype) {
				Clipboard.prototype.writeText = writeText;
				Clipboard.prototype.write = write;
			}
		} catch (e) {}
		try {
			const c = navigator.clipboard;
			if (c) {
				c.writeText = writeText;
				c.write = write;
			}
		} catch (e) {}
		if (!window.__chatbangCopyListener) {
			window.__chatbangCopyListener = true;
			document.addEventListener("copy", function(e) {
				try {
					const t = (e.clipboardData && e.clipboardData.getData("text/plain")) || "";
					if (t) grab(t);
				} catch (err) {}
			}, true);
		}`

const jsClipboardHookBoot = `(() => {` + jsClipboardHookInner + `})()`

const jsCopyTurn = `
		function lastAssistantTurn() {
			` + jsAssistantNodes + `
			if (!nodes.length) return null;
			return nodes[nodes.length - 1];
		}
		function lastTurnCopyButton() {
			const last = lastAssistantTurn();
			if (!last) return null;
			const labeled = last.querySelector('[data-testid="copy-turn-action-button"], button[aria-label="Copy"], button[aria-label="Copy message"], button[title="Copy"]');
			if (labeled && !labeled.closest("pre, code")) return labeled;
			for (const b of last.querySelectorAll("button")) {
				if (b.closest("pre, code")) continue;
				const t = ((b.getAttribute("data-testid") || "") + " " + (b.getAttribute("aria-label") || "") + " " + (b.getAttribute("title") || "") + " " + (b.textContent || "")).toLowerCase();
				if (/\bcopy\b/.test(t) && !t.includes("copied") && !t.includes("code")) return b;
			}
			return null;
		}
		function installCopyHook() {
` + jsClipboardHookInner + `
		}
		function dismissCopyFailureToast() {
			const keys = ["lost focus", "copy failed", "document is not focused", "couldn't copy", "could not copy"];
			for (const el of document.querySelectorAll('[role="status"], [role="alert"], [role="log"]')) {
				const t = (el.innerText || "").replace(/\s+/g, " ").trim();
				if (!t || t.length > 240) continue;
				const low = t.toLowerCase();
				if (!keys.some((k) => low.includes(k))) continue;
				el.remove();
			}
		}`

const jsChatGPTAuth = `
async function chatbangAuthHeaders() {
	const h = {};
	try {
		const r = await fetch("/api/auth/session", { credentials: "include" });
		if (r.ok) {
			const d = await r.json();
			if (d.accessToken) h.authorization = "Bearer " + d.accessToken;
			const acc = d.account?.id || d.user?.id;
			if (acc) h["chatgpt-account-id"] = acc;
		}
	} catch (e) {}
	if (!h["chatgpt-account-id"]) {
		const m = document.cookie.match(/(?:^|;\\s*)_account=([^;]+)/);
		if (m) h["chatgpt-account-id"] = decodeURIComponent(m[1].trim());
	}
	return h;
}`

const jsFileDownloadHelpers = `
function chatbangHover(el) {
	if (!el) return;
	for (const ev of ["mouseenter", "mouseover", "pointerenter"]) {
		el.dispatchEvent(new MouseEvent(ev, { bubbles: true, cancelable: true, view: window }));
	}
}
function chatbangAssistantRoot(nodes) {
	if (!nodes || !nodes.length) return null;
	return nodes[nodes.length - 1].closest('section[data-testid^="conversation-turn"]')
		|| nodes[nodes.length - 1].closest('article')
		|| nodes[nodes.length - 1];
}
function chatbangIsDownloadFileButton(btn) {
	if (!btn || btn.tagName !== "BUTTON") return false;
	const label = (btn.getAttribute("aria-label") || "").trim().toLowerCase();
	return label === "download file";
}
function chatbangMatchesFileName(text, wantLower) {
	text = (text || "").toLowerCase();
	if (!wantLower) return true;
	if (text.includes(wantLower)) return true;
	const alt = wantLower.replace(/_/g, " ");
	if (text.includes(alt)) return true;
	const stem = wantLower.replace(/\.[^.]+$/, "").replace(/_/g, " ");
	return stem.length > 2 && text.includes(stem);
}
function chatbangMessageIDs(root) {
	const ids = [];
	const seen = new Set();
	const add = (v) => {
		v = (v || "").trim();
		if (!v || seen.has(v)) return;
		seen.add(v);
		ids.push(v);
	};
	if (!root) return ids;
	for (const el of root.querySelectorAll("[data-message-id], [data-messageId], [data-turn-id]")) {
		add(el.getAttribute("data-message-id") || el.getAttribute("data-messageId") || el.getAttribute("data-turn-id"));
	}
	let el = root;
	for (let i = 0; i < 12 && el; i++) {
		for (const attr of ["data-message-id", "data-messageId", "data-turn-id"]) {
			add(el.getAttribute && el.getAttribute(attr));
		}
		el = el.parentElement;
	}
	return ids;
}
function chatbangFindDownloadURL(obj, depth) {
	if (!obj || (depth = depth || 0) > 6) return "";
	if (typeof obj === "string") {
		if (obj.startsWith("http") || obj.startsWith("/backend-api")) return obj;
		return "";
	}
	if (typeof obj !== "object") return "";
	for (const k of ["download_url", "downloadUrl", "url", "signed_url", "signedUrl", "href", "link", "file_url", "fileUrl"]) {
		if (typeof obj[k] === "string" && obj[k]) return obj[k];
	}
	for (const v of Object.values(obj)) {
		const found = chatbangFindDownloadURL(v, depth + 1);
		if (found) return found;
	}
	return "";
}
function chatbangIsDownloadRequest(url) {
	url = (url || "").toLowerCase();
	return url.includes("interpreter/download") || url.includes("estuary/content") || url.includes("/backend-api/files/");
}
function chatbangToB64(arr) {
	let bin = "";
	const chunk = 0x8000;
	for (let i = 0; i < arr.length; i += chunk) {
		bin += String.fromCharCode.apply(null, arr.subarray(i, i + chunk));
	}
	return btoa(bin);
}
function chatbangNameFromDisposition(r) {
	const disp = (r.headers && r.headers.get("content-disposition")) || "";
	let m = disp.match(/filename\*=UTF-8''([^;]+)/i);
	if (m) return decodeURIComponent(m[1]);
	m = disp.match(/filename="?([^";]+)"?/i);
	return m ? m[1] : "";
}
function chatbangFindDownloadTargets(root, wantLower) {
	const out = [];
	const seenBtn = new Set();
	const add = (card, btn) => {
		if (!btn || !chatbangIsDownloadFileButton(btn) || seenBtn.has(btn)) return;
		seenBtn.add(btn);
		out.push({ card: card || btn.parentElement, btn });
	};
	for (const btn of root.querySelectorAll('button[aria-label="Download file"]')) {
		const card = btn.closest("div[class*='group']") || btn.closest("li") || btn.parentElement?.parentElement || btn.parentElement;
		if (wantLower && card && !chatbangMatchesFileName(card.textContent, wantLower)) continue;
		add(card, btn);
	}
	if (!out.length && wantLower) {
		for (const el of root.querySelectorAll("div, li, p, span, a")) {
			if (!chatbangMatchesFileName(el.textContent, wantLower)) continue;
			const card = el.closest("div[class*='group']") || el.closest("li") || el;
			chatbangHover(card);
			const btn = card.querySelector('button[aria-label="Download file"]');
			if (btn) add(card, btn);
		}
	}
	if (!out.length) {
		for (const btn of root.querySelectorAll('button[aria-label="Download file"]')) {
			const card = btn.closest("div[class*='group']") || btn.closest("li") || btn.parentElement?.parentElement || btn.parentElement;
			add(card, btn);
		}
	}
	return out;
}
function chatbangLatestDownloadTarget(root, wantLower) {
	const all = chatbangFindDownloadTargets(root, wantLower);
	return all.length ? all[all.length - 1] : null;
}
function chatbangMessageIDFrom(el) {
	let mid = "";
	for (let i = 0; i < 12 && el; i++) {
		for (const attr of ["data-message-id", "data-messageId", "data-turn-id"]) {
			const v = el.getAttribute && el.getAttribute(attr);
			if (v) { mid = v; break; }
		}
		if (mid) break;
		el = el.parentElement;
	}
	return mid;
}
function chatbangFetchOpts(headers) {
	return { credentials: "include", headers, cache: "no-store" };
}
function chatbangExtractDownloadURLs(container) {
	const urls = [];
	const seen = new Set();
	const add = (u) => {
		u = (u || "").trim();
		if (!u || seen.has(u)) return;
		seen.add(u);
		try { urls.push(new URL(u, location.origin).href); } catch (e) {}
	};
	if (!container) return urls;
	const blob = (container.innerHTML || "") + "\n" + (container.outerHTML || "");
	for (const m of blob.matchAll(/\/backend-api\/conversation\/[^"'\\s]+?\/interpreter\/download\?[^"'\\s<>)]+/g)) add(m[0]);
	for (const m of blob.matchAll(/\/backend-api\/estuary\/content\?[^"'\\s<>)]+/g)) add(m[0]);
	for (const a of container.querySelectorAll('a[href*="interpreter/download"], a[href*="estuary/content"]')) {
		add(a.getAttribute("href"));
	}
	return urls;
}
function chatbangClickDownloadFileButton(root, wantLower) {
	const target = chatbangLatestDownloadTarget(root, wantLower);
	if (!target) return false;
	chatbangHover(target.card);
	target.btn.click();
	return true;
}`
