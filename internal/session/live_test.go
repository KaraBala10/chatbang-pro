package session

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
)

func TestLivePingReplyAndURL(t *testing.T) {
	ws := strings.TrimSpace(os.Getenv("CHATBANG_LIVE_CDP"))
	if ws == "" {
		t.Skip("set CHATBANG_LIVE_CDP to the browser websocket URL")
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), ws)
	t.Cleanup(allocCancel)
	opts := []chromedp.ContextOption{}
	if id := strings.TrimSpace(os.Getenv("CHATBANG_LIVE_TARGET")); id != "" {
		opts = append(opts, chromedp.WithTargetID(target.ID(id)))
	}
	ctx, ctxCancel := chromedp.NewContext(allocCtx, opts...)
	t.Cleanup(ctxCancel)
	ctx, timeoutCancel := context.WithTimeout(ctx, 120*time.Second)
	t.Cleanup(timeoutCancel)

	if err := chromedp.Run(ctx, chromedp.Navigate(chaturl.DefaultURL)); err != nil {
		t.Fatal(err)
	}
	if err := waitForChatReady(ctx, chaturl.DefaultURL); err != nil {
		t.Fatal(err)
	}

	known := map[string]bool{}
	for _, id := range pageConversationIDs(ctx) {
		known[id] = true
	}
	baseline, _ := evaluateResponseStatus(ctx)
	if err := submitPrompt(ctx, "ping"); err != nil {
		t.Fatal(err)
	}
	text, _, err := waitForResponse(ctx, false, baseline, new(string))
	if err != nil {
		t.Fatal(err)
	}
	vis := visibleAssistantText(string(text))
	if vis == "" || isStatusChrome(vis) {
		t.Fatalf("expected a real reply, got %q", vis)
	}
	t.Logf("reply: %s", vis)

	deadline := time.Now().Add(5 * time.Second)
	var link string
	for time.Now().Before(deadline) {
		var loc string
		_ = chromedp.Run(ctx, chromedp.Location(&loc))
		if p := chaturl.ConversationPermalink(loc); p != "" {
			link = p
			break
		}
		for _, id := range pageConversationIDs(ctx) {
			if !known[id] {
				link = "https://chatgpt.com/c/" + id
				break
			}
		}
		if link != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if link == "" || strings.Contains(strings.ToLower(link), "web:") {
		t.Fatalf("expected a real conversation URL, got %q", link)
	}
	t.Logf("url: %s", link)

	_ = waitForSendButton(ctx, 3*time.Second)
	s := &Session{ctx: ctx, convURL: link, knownChats: known}
	if err := s.ensureComposerReady(); err != nil {
		t.Fatal(err)
	}
	baseline, _ = evaluateResponseStatus(ctx)
	if err := submitPrompt(ctx, "how are you?"); err != nil {
		t.Fatal(err)
	}
	text, _, err = waitForResponse(ctx, false, baseline, new(string))
	if err != nil {
		t.Fatal(err)
	}
	vis = visibleAssistantText(string(text))
	if vis == "" || isStatusChrome(vis) || strings.EqualFold(strings.TrimSpace(vis), "pong") {
		t.Fatalf("expected a how-are-you reply, got %q", vis)
	}
	if len([]rune(vis)) < 40 {
		t.Fatalf("reply looks truncated: %q", vis)
	}
	t.Logf("how-are-you reply: %s", vis)
}
