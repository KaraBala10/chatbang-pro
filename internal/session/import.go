package session

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/browsers"
	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
	"github.com/KaraBala10/chatbang-pro/internal/config"
)

// Setup imports a ChatGPT session from another browser, or falls back to manual login.
func Setup(browser, profileDir, homeDir string) {
	sources := browsers.Discover(homeDir)
	fmt.Println("Chatbang Pro setup")
	fmt.Println()
	if len(sources) == 0 {
		fmt.Println("No other browser profiles found on this system.")
		fmt.Println()
		LoginProfile(browser, profileDir)
		return
	}

	fmt.Println("Import a ChatGPT session from another browser, or log in manually.")
	fmt.Println()
	src, err := browsers.PromptSelect(os.Stdout, os.Stdin, sources)
	if err != nil {
		log.Fatal(err)
	}
	if src == nil {
		LoginProfile(browser, profileDir)
		return
	}

	fmt.Printf("Reading session from %s…\n", src.Label())
	cookies, err := browsers.Extract(*src)
	if err != nil {
		fmt.Printf("Could not import that session: %v\n", err)
		fmt.Println("Falling back to manual login.")
		fmt.Println()
		LoginProfile(browser, profileDir)
		return
	}

	fmt.Printf("Found a ChatGPT session (%d cookies). Opening it in %s…\n", len(cookies), browser)
	if err := importProfile(browser, profileDir, cookies); err != nil {
		log.Fatal(err)
	}
}

func importProfile(browser, profileDir string, cookies []browsers.Cookie) error {
	if err := config.PrepareProfile(profileDir); err != nil {
		return err
	}

	allocatorCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		config.AllocatorOptions(browser, profileDir, false)...,
	)
	ctx, ctxCancel := chromedp.NewContext(allocatorCtx, chromedp.WithErrorf(suppressChromedpNoise))
	var once sync.Once
	stop := func() {
		once.Do(func() { stopBrowser(ctx, ctxCancel, allocCancel, profileDir) })
	}
	defer stop()

	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		return fmt.Errorf("could not open Chatbang browser: %w", err)
	}
	if err := applyCookies(ctx, cookies); err != nil {
		return fmt.Errorf("could not apply imported cookies: %w", err)
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(chaturl.DefaultURL)); err != nil {
		return fmt.Errorf("could not open ChatGPT: %w", err)
	}

	if err := waitForChatReady(ctx, chaturl.DefaultURL); err != nil {
		fmt.Println()
		fmt.Println("Cookies were imported, but ChatGPT is not ready yet.")
		fmt.Println("  1. Finish login in the open window if asked")
		fmt.Println("  2. Start a chat so the page is ready")
		fmt.Println("  3. Return here and press Enter to save")
	} else {
		fmt.Println()
		fmt.Println("Session imported. ChatGPT is ready in this window.")
		fmt.Println("Press Enter to save and close the browser.")
	}
	waitEnter()
	stop()
	fmt.Println("Configuration saved.")
	return nil
}

func applyCookies(ctx context.Context, cookies []browsers.Cookie) error {
	params := make([]*network.CookieParam, 0, len(cookies))
	for _, c := range cookies {
		params = append(params, toCookieParam(c))
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return storage.SetCookies(params).Do(ctx)
	}))
}

func toCookieParam(c browsers.Cookie) *network.CookieParam {
	p := &network.CookieParam{
		Name:     c.Name,
		Value:    c.Value,
		Path:     c.Path,
		Secure:   c.Secure,
		HTTPOnly: c.HTTPOnly,
		SameSite: cookieSameSite(c.SameSite),
		URL:      cookieURL(c),
	}
	if p.Path == "" {
		p.Path = "/"
	}
	if strings.HasPrefix(c.Name, "__Host-") {
		p.Domain = ""
		p.Path = "/"
		p.Secure = true
	} else {
		p.Domain = c.Domain
	}
	if c.Expires > 0 && !c.Session {
		exp := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
		p.Expires = &exp
	}
	return p
}

func cookieURL(c browsers.Cookie) string {
	host := strings.TrimPrefix(c.Domain, ".")
	path := c.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	scheme := "http"
	if c.Secure || strings.HasPrefix(c.Name, "__Secure-") || strings.HasPrefix(c.Name, "__Host-") {
		scheme = "https"
	}
	return scheme + "://" + host + path
}

func cookieSameSite(v string) network.CookieSameSite {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "strict":
		return network.CookieSameSiteStrict
	case "lax":
		return network.CookieSameSiteLax
	case "none":
		return network.CookieSameSiteNone
	default:
		return ""
	}
}
