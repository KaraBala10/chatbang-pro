package help

import (
	"fmt"

	markdown "github.com/MichaelMure/go-term-markdown"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
)

// Print renders CLI help to stdout.
func Print(configPath string) {
	helpMarkdown := fmt.Sprintf(`# Chatbang Pro

**ChatGPT in your terminal** — no API key, no API quotas. Chatbang drives the real ChatGPT web app in Chromium.

## Usage

`+"```"+`bash
chatbang-pro [flags]
`+"```"+`

Type a prompt at `+"`> `"+`, wait for `+"`[Thinking...]`"+`, then read the reply. **Ctrl+C** while it is generating stops ChatGPT and prints what was written so far. Type **exit** or **quit** to leave cleanly.

## Flags

| Flag | Description |
|------|-------------|
| `+"`-h`"+`, `+"`--help`"+` | Show this help |
| `+"`--config`"+` | Import a ChatGPT session from another browser, or log in in a visible window |
| `+"`--headless`"+` | Force headless mode (browser runs in the background) |
| `+"`--no-headless`"+` | Show the browser window while chatting |
| `+"`--temporary-chat`"+`, `+"`--temp`"+` | Use [temporary chat](%s) (works with `+"`--gpt`"+` too) |
| `+"`--gpt`"+`, `+"`--custom-gpt`"+`, `+"`-g`"+` | Chat with a [custom GPT](https://chatgpt.com/gpts) (full URL, `+"`/g/g-...`"+` path, or `+"`g-...`"+` id) |
| `+"`--message`"+`, `+"`-m`"+` | Send one prompt, print the reply, and exit (non-interactive) |

## First-time setup

1. Install a Chromium-based browser (Chromium, Chrome, Edge, Brave, etc.) under `+"`/bin`"+`, `+"`/usr/bin`"+`, or `+"`~/.local/bin`"+`.
2. Run `+"`chatbang-pro --config`"+` — pick a browser that's already logged in to ChatGPT, or log in manually, then press **Enter**.
3. Start chatting: `+"`chatbang-pro`"+`

## Configuration file

Path: `+"`%s`"+`

`+"```"+`
browser=/usr/bin/chromium-browser
headless=true
profile=/home/you/chatbang/profile_data
`+"```"+`

| Key | Description |
|-----|-------------|
| `+"`browser`"+` | Path to your Chromium-based browser executable |
| `+"`headless`"+` | `+"`true`"+` (default) hides the browser; `+"`false`"+` shows it |
| `+"`profile`"+` | Chromium user-data dir (Snap Chromium needs a non-hidden path such as `+"`~/chatbang/profile_data`"+`) |

CLI flags override `+"`headless`"+` for that run only.

## Examples

`+"```"+`bash
chatbang-pro                          # interactive chat (headless)
chatbang-pro --no-headless            # show the browser while chatting
chatbang-pro --temporary-chat         # private temporary chat session
chatbang-pro --temp --no-headless     # visible temporary chat
chatbang-pro --gpt https://chatgpt.com/g/g-81BdggBV3-website-mobile-app-builder-ui-ux-web-design
chatbang-pro --gpt https://chatgpt.com/g/g-xxx --temp
chatbang-pro -g g-81BdggBV3-website-mobile-app-builder-ui-ux-web-design
chatbang-pro --gpt https://chatgpt.com/g/g-xxx --message "كيفك"
chatbang-pro -m "What is 2+2?"
chatbang-pro --config                 # import session from another browser, or log in
`+"```"+`

## Tips

- Ctrl+C while a reply is generating stops ChatGPT and prints the partial reply.
`, chaturl.TempURL, configPath)
	fmt.Println(string(markdown.Render(helpMarkdown, 100, 2)))
}
