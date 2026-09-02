package browsers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"

	"github.com/KaraBala10/chatbang-pro/internal/config"
)

func extractChromium(src Source) ([]Cookie, error) {
	if src.ExecPath == "" {
		return nil, fmt.Errorf("%s is not installed (no executable found); cannot decrypt its cookies", src.Name)
	}

	tmp, err := snapshotChromiumProfile(src)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		config.ExecPathOptions(src.ExecPath)...,
	)
	allocOpts = append(allocOpts,
		chromedp.UserDataDir(tmp),
		chromedp.Flag("profile-directory", src.ProfileDir),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quietErrorf))
	defer ctxCancel()

	runCtx, runCancel := context.WithTimeout(ctx, 45*time.Second)
	defer runCancel()

	if err := chromedp.Run(runCtx, chromedp.Navigate("about:blank")); err != nil {
		return nil, fmt.Errorf("open %s profile: %w (close %s if it is running and try again)", src.Name, err, src.Name)
	}

	raw, err := storage.GetCookies().Do(runCtx)
	if err != nil {
		return nil, fmt.Errorf("read %s cookies: %w", src.Name, err)
	}

	cookies := make([]Cookie, 0, len(raw))
	for _, c := range raw {
		cookies = append(cookies, Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: string(c.SameSite),
			Session:  c.Session,
		})
	}
	return cookies, nil
}

func snapshotChromiumProfile(src Source) (string, error) {
	tmp, err := os.MkdirTemp("", "chatbang-import-*")
	if err != nil {
		return "", err
	}

	if err := copyIfExists(filepath.Join(src.UserDataDir, "Local State"), filepath.Join(tmp, "Local State")); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("copy Local State: %w", err)
	}

	srcProfile := filepath.Join(src.UserDataDir, src.ProfileDir)
	dstProfile := filepath.Join(tmp, src.ProfileDir)
	for _, name := range []string{"Preferences", "Secure Preferences"} {
		if err := copyIfExists(filepath.Join(srcProfile, name), filepath.Join(dstProfile, name)); err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
	}

	bases := []string{
		filepath.Join(srcProfile, "Network", "Cookies"),
		filepath.Join(srcProfile, "Cookies"),
	}
	copied := false
	for _, base := range bases {
		rel, err := filepath.Rel(srcProfile, base)
		if err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
		for _, side := range []string{"", "-wal", "-shm", "-journal"} {
			if err := copyIfExists(base+side, filepath.Join(dstProfile, rel+side)); err != nil {
				os.RemoveAll(tmp)
				return "", fmt.Errorf("copy cookies: %w", err)
			}
			if side == "" && fileExists(base) {
				copied = true
			}
		}
	}
	if !copied {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("no cookie database found in %s", src.Label())
	}
	return tmp, nil
}

func quietErrorf(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	if strings.Contains(msg, "unhandled node event") || strings.Contains(msg, "unhandled page event") {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
