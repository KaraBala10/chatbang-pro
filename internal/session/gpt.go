package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
)

func isOnCustomGPTPathJS(gptPrefix string) (string, error) {
	prefixJSON, err := json.Marshal(gptPrefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`(() => {
		const expected = %s;
		if (!expected) return true;
		const p = location.pathname.replace(/\/$/, '');
		if (!p || p === '/') return false;
		return p === expected || p.startsWith(expected + '/');
	})()`, prefixJSON), nil
}

func isOnCustomGPTPath(ctx context.Context, gptPrefix string) (bool, error) {
	if gptPrefix == "" {
		return true, nil
	}
	js, err := isOnCustomGPTPathJS(gptPrefix)
	if err != nil {
		return false, err
	}
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return false, err
	}
	return ok, nil
}

func ensureCustomGPTPage(ctx context.Context, chatURL, gptPrefix string) error {
	if gptPrefix == "" {
		return nil
	}
	ok, err := isOnCustomGPTPath(ctx, gptPrefix)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	fmt.Fprintln(os.Stderr, "ChatGPT left the custom GPT — reopening…")
	if err := chromedp.Run(ctx, chromedp.Navigate(chatURL)); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond)); err != nil {
		return err
	}
	if err := tryActivateCustomGPT(ctx, gptPrefix); err != nil {
		return err
	}
	ok, err = isOnCustomGPTPath(ctx, gptPrefix)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("could not return to custom GPT page %q", gptPrefix)
}

func customGPTActivateJS(gptPrefix string) (string, error) {
	prefixJSON, err := json.Marshal(gptPrefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`(() => {
		const expected = %s;
		const p = location.pathname.replace(/\/$/, '');
		if (!p || p === '/') return false;
		if (p !== expected && !p.startsWith(expected + '/')) return false;
		if (document.querySelector('#prompt-textarea')) return true;
		if (p !== expected) return false;

		const buttons = document.querySelectorAll('button, a[role="button"]');
		for (const el of buttons) {
			const t = (el.textContent || '').trim().toLowerCase();
			if (t === 'start chat' || t.includes('start chat')) {
				el.click();
				return false;
			}
		}
		for (const el of document.querySelectorAll('a[href]')) {
			const href = el.getAttribute('href') || '';
			if (href.includes(expected) && href.includes('/c/')) {
				el.click();
				return false;
			}
		}
		return false;
	})()`, prefixJSON), nil
}

func tryActivateCustomGPT(ctx context.Context, gptPrefix string) error {
	clickJS, err := customGPTActivateJS(gptPrefix)
	if err != nil {
		return err
	}
	var done bool
	return chromedp.Run(ctx, chromedp.Evaluate(clickJS, &done))
}

func waitForChatReady(ctx context.Context, chatURL string) error {
	gptPrefix := chaturl.CustomGPTPathPrefix(chatURL)

	pathCheckJS, err := isOnCustomGPTPathJS(gptPrefix)
	if err != nil {
		return err
	}
	readyJS := fmt.Sprintf(`(() => {
		if (document.title.includes('Just a moment')) return false;
		if (!document.querySelector('#prompt-textarea')) return false;
		return %s;
	})()`, pathCheckJS)

	deadline := time.Now().Add(navTimeout)
	nextNotice := time.Now().Add(8 * time.Second)
	notifiedDialog := false
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if gptPrefix != "" {
			if err := ensureCustomGPTPage(ctx, chatURL, gptPrefix); err != nil {
				return err
			}
		}
		if dismissBlockingDialogs(ctx) && !notifiedDialog {
			fmt.Fprintln(os.Stderr, "Dismissed ChatGPT's Too many requests dialog.")
			notifiedDialog = true
		}
		var ready bool
		err := chromedp.Run(ctx,
			chromedp.Evaluate(readyJS, &ready),
		)
		if err != nil {
			return err
		}
		if ready {
			return chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
		}
		if gptPrefix != "" {
			if err := tryActivateCustomGPT(ctx, gptPrefix); err != nil {
				return err
			}
		}
		if time.Now().After(nextNotice) {
			fmt.Fprintln(os.Stderr, "Still waiting for chat to start…")
			nextNotice = time.Now().Add(10 * time.Second)
		}
		time.Sleep(pollIntervalDone)
	}
	return fmt.Errorf("chatgpt.com did not become ready within %s", navTimeout)
}
