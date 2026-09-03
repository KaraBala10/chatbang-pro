# Changelog

All notable changes to Chatbang Pro are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.3.0] - 2026-09-02

**Chatbang Pro — image generation, cleaner replies, and more reliable sessions**

### Added

- Image generation — waits for ChatGPT's image widget, saves files under `~/chatbang/images`, and prints `[image] path` in the reply
- Copy-button reply capture — reads ChatGPT's Copy action for clean markdown (does not write the OS clipboard)
- Machine-readable reply block (`<<<chatbang-pro` … `>>>`); TTY sessions render markdown in the terminal
- Import a ChatGPT session from installed Chromium or Firefox profiles via `--config`
- Conversation URL after the first reply (`Conversation: https://chatgpt.com/c/…`)

### Changed

- Follow-up prompts no longer reload the page on every turn
- Rate-limit and "too many requests" dialogs are dismissed so replies can still be copied
- Leftover Snap Chromium sessions close quietly (no `permission denied` spam)
- `build.sh` installs to `~/.local/bin` by default; set `SYSTEM_INSTALL=1` to also copy into `/usr/bin`

### Fixed

- Truncated or chrome-only replies (`Thinking`, leftover Stop button)
- Image turns treated as finished before the image was ready
- Follow-up send failing when a disabled Stop control stayed in the composer
- After several image generations, the next text prompt could fail and exit the CLI; the composer is waited on, and a send failure stays in the session

### Install (Linux amd64)

```bash
curl -L https://github.com/KaraBala10/chatbang-pro/releases/download/v1.3.0/chatbang-pro.tar.gz | tar -xz
chmod +x chatbang-pro
mkdir -p ~/.local/bin
mv chatbang-pro ~/.local/bin/chatbang-pro
chatbang-pro --config
```

### Examples

```bash
chatbang-pro --no-headless
chatbang-pro -m "generate an image of a cute cat"
chatbang-pro --gpt g-xxx --message "Summarize this in one line"
```

## [1.2.0] - 2026-06-08

**Chatbang Pro — one-shot prompts, temp chat with custom GPTs, and faster startup**

### Added

- Non-interactive mode (`--message`, `-m`) — send one prompt, print the reply to stdout, and exit (great for scripts and pipes)
- Temporary chat + custom GPT (`--temp` with `--gpt`) — private sessions with a custom GPT; no longer ignored when both flags are set

### Changed

- Faster custom GPT startup — direct URL navigation, shorter readiness waits (500ms), and simplified activation flow
- Script-friendly output — `[Thinking...]` goes to stderr so stdout stays clean for piping
- `build.sh` — installs with `cp` instead of `mv` so rebuilds don't remove your local binary

### Install (Linux amd64)

```bash
curl -L https://github.com/KaraBala10/chatbang-pro/releases/download/v1.2.0/chatbang-pro -o chatbang-pro
chmod +x chatbang-pro
sudo mv chatbang-pro /usr/bin/chatbang-pro
chatbang-pro --config
```

### Examples

```bash
chatbang-pro -m "What is 2+2?"
chatbang-pro --gpt https://chatgpt.com/g/g-xxx --temp
chatbang-pro --gpt g-xxx --message "Summarize this in one line"
echo "Explain JSON" | xargs chatbang-pro -m
```

## [1.1.0] - 2026-06-07

**Chatbang Pro — custom GPT, temporary chat, and faster replies**

### Added

- Custom GPT support (`--gpt`, `--custom-gpt`, `-g`) — full URL, `/g/g-...` path, or `g-...` id
- Temporary chat mode (`--temporary-chat`, `--temp`) — conversations not saved to history
- Rich markdown `--help` and version shown at startup
- `build.sh` — build with embedded git version, `go vet`, and install to `/usr/bin/chatbang-pro`

### Changed

- Improved terminal prompt with placeholder, UTF-8 input, and clean exit via `exit` / `quit`
- Faster reply detection with streaming-aware polling and shorter completion waits
- Binary renamed to `chatbang-pro` (replaces `chatbang`)

### Install (Linux amd64)

```bash
curl -L https://github.com/KaraBala10/chatbang-pro/releases/download/v1.1.0/chatbang-pro -o chatbang-pro
chmod +x chatbang-pro
sudo mv chatbang-pro /usr/bin/chatbang-pro
chatbang-pro --config
```

## [1.0.0] - 2026-06-03

**Chatbang Pro — first release**

### Added

- DOM-based reply extraction (no clipboard)
- Improved Chromium browser detection and session profile
- Long replies (up to 15 min wait); auto fresh chat after very large responses
- Unicode and RTL prompt support via JavaScript
- Headless mode by default (`--headless` / `--no-headless`)
- Reconnect attempt and partial reply recovery on browser disconnect

### Install (Linux amd64)

```bash
curl -L https://github.com/KaraBala10/chatbang-pro/releases/download/v1.0.0/chatbang -o chatbang
chmod +x chatbang
sudo mv chatbang /usr/bin/chatbang
chatbang --config
```

---

## Requirements (all releases)

- Chromium-based browser (Chrome, Edge, Brave, etc.) on PATH or configured in `~/.config/chatbang/chatbang`
- Go 1.24+ only if building from source

## Build from source

```bash
git clone https://github.com/KaraBala10/chatbang-pro.git
cd chatbang-pro
./build.sh
```
