//go:build !windows

package config

import (
	"os/exec"
	"syscall"

	"github.com/chromedp/chromedp"
)

// BrowserProcessOptions keeps the Chromium child in its own session so terminal
// Ctrl+C (SIGINT) stops chatbang-pro without closing the browser window.
func BrowserProcessOptions() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			cmd.SysProcAttr.Setsid = true
		}),
	}
}
