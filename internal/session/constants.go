package session

import "time"

const (
	navTimeout             = 60 * time.Second
	responseTimeout        = 15 * time.Minute
	pollIntervalActive     = 200 * time.Millisecond
	pollIntervalDone       = 100 * time.Millisecond
	stablePollsDefault     = 3
	stablePollsLarge       = 4
	confirmDelayDefault    = 400 * time.Millisecond
	confirmDelayLarge      = 600 * time.Millisecond
	replySettleWait        = 500 * time.Millisecond
	partialMinGap          = 15 * time.Second
	largeResponseThreshold = 6000
	captureSeenTimeout     = 1500 * time.Millisecond
	startWaitTimeout       = 45 * time.Second
	imageSettleTimeout     = 90 * time.Second
	minGeneratedImageBytes = 8 * 1024
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
		function isImageGenPending(root) {
			if (!root || isImageGenDone(root)) return false;
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
			return clone.textContent || "";
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
		}`

const jsComposer = `
		function composerButton() {
			return document.querySelector('#composer-submit-button')
				|| document.querySelector('button[data-testid="send-button"]')
				|| document.querySelector('button[data-testid="stop-button"]')
				|| document.querySelector('button[aria-label="Send prompt"]')
				|| document.querySelector('button[aria-label="Send"]');
		}
		function isStopControl(b) {
			if (!b) return false;
			if (b.disabled || b.getAttribute('aria-disabled') === 'true') return false;
			if ((b.getAttribute('data-testid') || '') === 'stop-button') return true;
			const a = (b.getAttribute('aria-label') || '').toLowerCase();
			return a.includes('stop answering') || a.includes('stop streaming') || a.includes('stop generating');
		}
		function isSendableControl(b) {
			if (!b || isStopControl(b)) return false;
			if (b.disabled || b.getAttribute('aria-disabled') === 'true') return false;
			const t = (b.getAttribute('data-testid') || '').toLowerCase();
			const a = (b.getAttribute('aria-label') || '').toLowerCase();
			if (t === 'stop-button' || a.includes('stop')) return false;
			return true;
		}
		function composerText() {
			const el = document.querySelector('#prompt-textarea');
			return ((el && (el.innerText || el.value || el.textContent)) || '').replace(/\u200b/g, '').trim();
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
			if (window.__chatbangCopyHook) return;
			window.__chatbangCopyHook = true;
			window.__chatbangCopied = "";
			const grab = (text) => { window.__chatbangCopied = String(text ?? ""); };
			try {
				if (window.Clipboard && Clipboard.prototype.writeText) {
					Clipboard.prototype.writeText = function(text) {
						grab(text);
						return Promise.resolve();
					};
				}
			} catch (e) {}
			try {
				if (navigator.clipboard && navigator.clipboard.writeText) {
					navigator.clipboard.writeText = function(text) {
						grab(text);
						return Promise.resolve();
					};
				}
			} catch (e) {}
			document.addEventListener("copy", function(e) {
				try {
					const t = (e.clipboardData && e.clipboardData.getData("text/plain")) || "";
					if (t) grab(t);
				} catch (err) {}
			}, true);
		}`
