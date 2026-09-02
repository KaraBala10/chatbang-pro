package session

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

func copyAssistantReply(ctx context.Context) string {
	_ = dismissChatDialogs(ctx)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if text := tryCopyAssistantReply(ctx); text != "" {
			return text
		}
		if dismissChatDialogs(ctx) {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		time.Sleep(150 * time.Millisecond)
	}
	return ""
}

func tryCopyAssistantReply(ctx context.Context) string {
	prepJS := `(() => {
		` + jsCopyTurn + `
		installCopyHook();
		window.__chatbangCopied = "";
		document.querySelectorAll("[data-chatbang-copy]").forEach((el) => el.removeAttribute("data-chatbang-copy"));
		const btn = lastTurnCopyButton();
		if (!btn) return false;
		btn.setAttribute("data-chatbang-copy", "1");
		return true;
	})()`
	var found bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(prepJS, &found)); err != nil || !found {
		return ""
	}

	_ = chromedp.Run(ctx, chromedp.Click(`[data-chatbang-copy="1"]`, chromedp.ByQuery))
	clickJS := `(() => {
		` + jsCopyTurn + `
		installCopyHook();
		const b = document.querySelector("[data-chatbang-copy]") || lastTurnCopyButton();
		if (!b) return false;
		b.click();
		return true;
	})()`
	_ = chromedp.Run(ctx, chromedp.Evaluate(clickJS, nil))

	readJS := `(() => (window.__chatbangCopied || "").trim())()`
	wait := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(wait) {
		var text string
		if err := chromedp.Run(ctx, chromedp.Evaluate(readJS, &text)); err != nil {
			return ""
		}
		if text != "" {
			return text
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}
