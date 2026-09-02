package session

import (
	"context"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/config"
)

func stopBrowser(ctx context.Context, ctxCancel, allocCancel context.CancelFunc, profileDir string) {
	if ctx != nil {
		closeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = chromedp.Run(closeCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return browser.Close().Do(ctx)
		}))
		cancel()
	}
	if ctxCancel != nil {
		ctxCancel()
	}
	config.ReleaseProfile(profileDir)
	if allocCancel == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		allocCancel()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}
