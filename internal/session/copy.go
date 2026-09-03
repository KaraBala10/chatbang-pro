package session

import (
	"context"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func copyAssistantReply(ctx context.Context) string {
	if text := tryCopyAssistantReply(ctx); text != "" {
		return text
	}
	if dismissChatDialogs(ctx) {
		if text := tryCopyAssistantReply(ctx); text != "" {
			return text
		}
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(40 * time.Millisecond)
		if text := tryCopyAssistantReply(ctx); text != "" {
			return text
		}
	}
	return ""
}

func tryCopyAssistantReply(ctx context.Context) string {
	js := `(() => {
		` + jsCopyTurn + `
		installCopyHook();
		window.__chatbangCopied = "";
		const btn = lastTurnCopyButton();
		if (!btn) return "";
		const realHasFocus = Document.prototype.hasFocus;
		Document.prototype.hasFocus = function() { return true; };
		const restore = () => { Document.prototype.hasFocus = realHasFocus; };
		try { btn.click(); } catch (e) {}
		dismissCopyFailureToast();
		const now = (window.__chatbangCopied || "").trim();
		if (now) { restore(); return now; }
		return new Promise((resolve) => {
			const t0 = performance.now();
			const tick = () => {
				dismissCopyFailureToast();
				const t = (window.__chatbangCopied || "").trim();
				if (t || performance.now() - t0 > 150) {
					restore();
					resolve(t);
				} else requestAnimationFrame(tick);
			};
			queueMicrotask(tick);
		});
	})()`
	var text string
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &text, awaitPromise)); err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func awaitPromise(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

func enableCopyHook() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := page.Enable().Do(ctx); err != nil {
			return err
		}
		_, err := page.AddScriptToEvaluateOnNewDocument(jsClipboardHookBoot).Do(ctx)
		return err
	})
}
