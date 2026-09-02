package session

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"

	"github.com/KaraBala10/chatbang-pro/internal/browsers"
)

func TestToCookieParam(t *testing.T) {
	c := browsers.Cookie{
		Name:     "__Secure-next-auth.session-token",
		Value:    "tok",
		Domain:   ".chatgpt.com",
		Path:     "/",
		Expires:  float64(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix()),
		Secure:   true,
		HTTPOnly: true,
		SameSite: "Lax",
	}
	p := toCookieParam(c)
	if p.Name != c.Name || p.Value != "tok" || p.Domain != ".chatgpt.com" {
		t.Fatalf("unexpected cookie param: %+v", p)
	}
	if p.URL != "https://chatgpt.com/" {
		t.Fatalf("url = %q", p.URL)
	}
	if p.SameSite != network.CookieSameSiteLax {
		t.Fatalf("samesite = %q", p.SameSite)
	}
	if p.Expires == nil {
		t.Fatal("expected expiry")
	}
}

func TestToCookieParamHostPrefix(t *testing.T) {
	p := toCookieParam(browsers.Cookie{
		Name:   "__Host-next-auth.csrf-token",
		Value:  "x",
		Domain: "chatgpt.com",
		Secure: true,
	})
	if p.Domain != "" {
		t.Fatalf("host-prefix cookies must not set Domain, got %q", p.Domain)
	}
	if p.Path != "/" || !p.Secure {
		t.Fatalf("host-prefix cookie path/secure: %+v", p)
	}
	if !strings.HasPrefix(p.URL, "https://chatgpt.com") {
		t.Fatalf("url = %q", p.URL)
	}
}
