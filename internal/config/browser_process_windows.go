//go:build windows

package config

import "github.com/chromedp/chromedp"

func BrowserProcessOptions() []chromedp.ExecAllocatorOption {
	return nil
}
