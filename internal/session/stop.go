package session

import (
	"context"

	"github.com/KaraBala10/chatbang-pro/internal/config"
)

func stopBrowser(_ context.Context, tabCancel, allocCancel context.CancelFunc, profileDir string) {
	if tabCancel != nil {
		tabCancel()
	}
	if allocCancel != nil {
		allocCancel()
	}
	config.ReleaseProfile(profileDir)
}
