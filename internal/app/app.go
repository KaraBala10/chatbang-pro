package app

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"strings"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
	"github.com/KaraBala10/chatbang-pro/internal/cli"
	"github.com/KaraBala10/chatbang-pro/internal/config"
	"github.com/KaraBala10/chatbang-pro/internal/help"
	"github.com/KaraBala10/chatbang-pro/internal/prompt"
	"github.com/KaraBala10/chatbang-pro/internal/session"
)

// Run is the application entry point.
func Run(version string, args []string) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Error fetching user info:", err)
		return
	}

	paths := config.PathsForHome(usr.HomeDir)

	opts := cli.Parse(args, true)
	if opts.WantHelp {
		help.Print(paths.File)
		return
	}

	if err = os.MkdirAll(paths.Dir, 0o755); err != nil {
		fmt.Println("Error creating config directory:", err)
		return
	}

	configFile, err := os.OpenFile(paths.File, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return
	}
	defer configFile.Close()

	if _, err = configFile.Seek(0, io.SeekStart); err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}
	settings := config.Parse(configFile)
	resolved, updated, err := config.ResolveBrowser(settings.Browser, usr.HomeDir)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Install Chromium, Chrome, Edge, or Brave, or edit the config at", paths.File)
		return
	}
	profile := config.ProfileDir(usr.HomeDir, resolved, settings.Profile)
	if updated || settings.Browser != resolved || settings.Profile != profile {
		fmt.Fprintf(os.Stderr, "Using browser %s\n", resolved)
		if profile != paths.Profile {
			fmt.Fprintf(os.Stderr, "Using profile %s\n", profile)
		}
		if _, err = configFile.Seek(0, io.SeekStart); err != nil {
			fmt.Println("Error writing config file:", err)
			return
		}
		if err = configFile.Truncate(0); err != nil {
			fmt.Println("Error writing config file:", err)
			return
		}
		if _, err = io.WriteString(configFile, config.Format(config.Settings{
			Browser:  resolved,
			Headless: settings.Headless,
			Profile:  profile,
		})); err != nil {
			fmt.Println("Error writing config file:", err)
			return
		}
	}
	defaultBrowser := resolved

	opts = cli.Parse(args, settings.Headless)
	if opts.WantConfig {
		session.Setup(defaultBrowser, profile, usr.HomeDir)
		return
	}

	headless := opts.Headless
	chatTarget, err := chaturl.Resolve(opts.TemporaryChat, opts.CustomGPT)
	if err != nil {
		log.Fatal(err)
	}
	if opts.CustomGPT != "" {
		fmt.Fprintf(os.Stderr, "Custom GPT: %s\n", chatTarget)
	}
	if chaturl.IsTemporary(chatTarget) {
		fmt.Fprintln(os.Stderr, "Temporary chat mode — conversations are not saved to history.")
	}

	fmt.Fprintf(os.Stderr, "chatbang-pro %s\n", version)
	fmt.Fprintln(os.Stderr, "Starting browser and opening ChatGPT…")
	sess, err := session.New(defaultBrowser, profile, paths.Images, headless, chatTarget)
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	if opts.MessageFlag && strings.TrimSpace(opts.Message) == "" {
		log.Fatal("--message requires a value")
	}
	if msg := strings.TrimSpace(opts.Message); msg != "" {
		sess.RunTurn(msg)
		return
	}

	fmt.Fprintln(os.Stderr, "Ready — start chatting below.")
	prompt.Loop(cli.IsExitCommand, sess.RunTurn)
}
