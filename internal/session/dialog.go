package session

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/chromedp"
)

func dismissBlockingDialogs(ctx context.Context) bool {
	markJS := `(() => {
		` + jsDismissDialogs + `
		document.querySelectorAll("[data-chatbang-dismiss]").forEach((el) => el.removeAttribute("data-chatbang-dismiss"));
		const btn = rateLimitDismissButton();
		if (!btn) return false;
		btn.setAttribute("data-chatbang-dismiss", "1");
		return true;
	})()`
	var found bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(markJS, &found)); err != nil || !found {
		return false
	}
	_ = chromedp.Run(ctx, chromedp.Click(`[data-chatbang-dismiss="1"]`, chromedp.ByQuery))
	clickJS := `(() => {
		const b = document.querySelector("[data-chatbang-dismiss]");
		if (!b) return false;
		b.click();
		b.removeAttribute("data-chatbang-dismiss");
		return true;
	})()`
	_ = chromedp.Run(ctx, chromedp.Evaluate(clickJS, nil))
	return true
}

func dismissChatDialogs(ctx context.Context) bool {
	if !dismissBlockingDialogs(ctx) {
		return false
	}
	fmt.Fprintln(os.Stderr, "Dismissed a ChatGPT dialog.")
	return true
}
