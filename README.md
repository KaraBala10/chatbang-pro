<div align="center">
  <img src="docs/chatbang-pro-logo.png" alt="chatbang-pro logo" width="128" />
  <h1>chatbang-pro</h1>
  <p>
    ChatGPT in the terminal.<br />
    No API key. Drives the ChatGPT web app in Chromium, including custom GPTs and image generation.
  </p>
  <p>
    <a href="https://github.com/KaraBala10/chatbang-pro/releases/latest"><img src="https://img.shields.io/github/v/release/KaraBala10/chatbang-pro?style=for-the-badge&logo=github&label=latest%20release" alt="Latest release" /></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-a3e635?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="MIT License" /></a>
    <a href="CHANGELOG.md"><img src="https://img.shields.io/badge/changelog-Keep%20a%20Changelog-E05735?style=for-the-badge&logo=keepachangelog&logoColor=white" alt="Changelog" /></a>
    <a href="https://semver.org/spec/v2.0.0.html"><img src="https://img.shields.io/badge/versioning-SemVer-3F9FD7?style=for-the-badge&logo=semver&logoColor=white" alt="Semantic Versioning" /></a>
    <img src="https://img.shields.io/badge/platform-Linux-111827?style=for-the-badge&logo=linux&logoColor=white" alt="Linux" />
  </p>
</div>

---

## Table of contents

- [Features](#features)
- [Install](#install)
- [Setup](#setup)
- [Usage](#usage)
- [Output](#output)
- [How it works](#how-it-works)
- [Config](#config)
- [Limits](#limits)
- [Versions](#versions)
- [License](#license)
- [Build from source](#build-from-source)

## Features

- **Terminal ChatGPT** — interactive chat at `>`, or one prompt with `-m` and exit
- **No API key** — uses your ChatGPT account in a local Chromium-based browser
- **Custom GPTs** — `-g` / `--gpt` with a full URL, `/g/g-...` path, or `g-...` id
- **Temporary chat** — `--temp` (also works with `--gpt`)
- **Image generation** — waits for ChatGPT's image widget, saves files under `~/chatbang/images`, and prints `[image] path` in the reply
- **File attachments** — code-interpreter files (`.py`, `.txt`, …) are saved under `~/chatbang/files` and printed as `[file] path`
- **Markdown replies** — TTY sessions render markdown; pipes keep raw markdown for scripts
- **Reply block** — every answer is wrapped in `<<<chatbang-pro` … `>>>` so it is easy to parse
- **Conversation URL** — after the first reply, stderr prints `Conversation: https://chatgpt.com/c/…`; follow-ups stay on that thread
- **Session import** — `--config` copies a ChatGPT login from installed Chrome, Chromium, Edge, Brave, or Firefox, or you log in once in a visible window
- **Headless or visible** — default is background browser; `--no-headless` shows the window
- **Unicode / RTL** — Arabic and other scripts work in the composer
- **Ctrl+C** — while a reply is generating, stops ChatGPT and prints what was written so far; the browser stays open

## Install

Download a ready binary from the latest GitHub Release. You do not need Go, and you do not need to clone this repository.

**[Latest release](https://github.com/KaraBala10/chatbang-pro/releases/latest)**

On that page, under **Assets**, download `chatbang-pro.tar.gz`.

### 1. Install a Chromium-based browser

Chatbang Pro drives a local **Chromium-based** browser (Chromium, Google Chrome, Edge, Brave, Vivaldi, …). Install one if you do not have it yet:

**Ubuntu / Debian (apt)**

```bash
sudo apt update
sudo apt install -y chromium-browser
# or Google Chrome from https://www.google.com/chrome/
```

**Ubuntu (Snap — common on fresh installs)**

```bash
sudo snap install chromium
```

Snap Chromium cannot write under hidden paths such as `~/.config/...`. Chatbang uses `~/chatbang/profile_data` automatically when it detects Snap.

**Fedora**

```bash
sudo dnf install -y chromium
```

**Arch Linux**

```bash
sudo pacman -S chromium
```

Check that a browser is found:

```bash
command -v chromium-browser || command -v chromium || command -v google-chrome-stable
```

If it lives outside `/usr/bin`, set `browser=/full/path` in `~/.config/chatbang/chatbang`.

### 2. Install chatbang-pro

```bash
curl -fL https://github.com/KaraBala10/chatbang-pro/releases/latest/download/chatbang-pro.tar.gz | tar -xz
chmod +x chatbang-pro
mkdir -p ~/.local/bin
mv chatbang-pro ~/.local/bin/chatbang-pro
```

Make sure `~/.local/bin` is on your `PATH`. Then run setup once:

```bash
chatbang-pro --config
```

You need a ChatGPT login in that browser profile (import from another browser or sign in during `--config`).

## Setup

`--config` lists browsers with saved profiles. Pick one that is already logged in to ChatGPT to import that session, or log in in the visible window and press Enter.

Optional config at `$HOME/.config/chatbang/chatbang`:

```
browser=/usr/bin/google-chrome
headless=true
profile=/home/you/.config/chatbang/profile_data
```

| Key | Description |
| --- | --- |
| `browser` | Path to the Chromium-based executable |
| `headless` | `true` (default) hides the browser; `false` shows it |
| `profile` | Chromium user-data dir. Default is `~/.config/chatbang/profile_data`. Snap Chromium cannot use a hidden path; it uses `~/chatbang/profile_data` instead |

`--headless` and `--no-headless` override `headless` for that run only.

## Usage

```bash
chatbang-pro --help
```

```bash
chatbang-pro
chatbang-pro --no-headless
chatbang-pro --config
chatbang-pro -m "What is 2+2?"
chatbang-pro -g g-XXXX
chatbang-pro --gpt https://chatgpt.com/g/g-xxxx --temp
```

Type a prompt at `>`. Empty lines are ignored. While a reply is generating, **Ctrl+C** stops ChatGPT and prints what was written so far. Type `exit` or `quit` to leave.

| Flag | Description |
| --- | --- |
| `-h`, `--help` | Show help |
| `--config` | Import a ChatGPT session from another browser, or log in in a visible window |
| `--headless` | Run the browser in the background |
| `--no-headless` | Show the browser window |
| `--temporary-chat`, `--temp` | Temporary chat ([chatgpt.com/?temporary-chat=true](https://chatgpt.com/?temporary-chat=true)); works with `--gpt` |
| `--gpt`, `--custom-gpt`, `-g` | Custom GPT: full URL, `/g/g-...` path, or `g-...` id |
| `--message`, `-m` | Send one prompt, print the reply, and exit |
| `--instances` | Print how many chatbang-pro instances are currently running |

## Output

stdout is only the reply block:

```text
<<<chatbang-pro
Hello! How can I help?
[image] /home/you/chatbang/images/20260902-153455-e640168b.png
[file] /home/you/chatbang/files/20260903-171023-07219cd9-hello_world.py
>>>
```

On a TTY, the text inside the block is rendered as terminal markdown. When stdout is a pipe, the block keeps the raw markdown so scripts can parse it. `[image]` and `[file]` lines stay as-is and are not run through the markdown renderer.

| Stream | Content |
| --- | --- |
| stdout | Reply block (`<<<chatbang-pro` … `>>>`) |
| stderr | Status (`[Thinking...]`, `[Generating image...]`, `Saved image:`, `Saved file:`, `Conversation:`) |

`Conversation: https://chatgpt.com/c/…` is printed on stderr after the first reply. Generated images go under `~/chatbang/images`; code-interpreter files go under `~/chatbang/files`.

## How it works

Chatbang Pro automates a real Chromium tab with [chromedp](https://github.com/chromedp/chromedp). It does not call the OpenAI API.

1. Opens [chatgpt.com](https://chatgpt.com) (or a custom GPT / temporary-chat URL) with a dedicated profile.
2. Fills the composer and sends the prompt.
3. Waits until streaming finishes, an image is ready, or ChatGPT shows an image-policy error.
4. Reads the assistant reply from ChatGPT's Copy action without writing the OS clipboard. Falls back to the page DOM if Copy is unavailable.
5. Prints the reply block. Image files are saved under `~/chatbang/images`; code-interpreter attachments under `~/chatbang/files`.

Follow-up prompts stay on the same conversation. The page is not reloaded on every turn.

## Config

| Path | Purpose |
| --- | --- |
| `~/.config/chatbang/chatbang` | Browser path, headless flag, profile dir |
| `~/.config/chatbang/profile_data` | Default Chromium user-data dir |
| `~/chatbang/profile_data` | Profile dir used with Snap Chromium |
| `~/chatbang/images` | Saved generated images |
| `~/chatbang/files` | Saved code-interpreter attachments |

## Limits

- Unofficial automation of chatgpt.com. OpenAI can change the page, throttle, or block it.
- Needs a local Chromium-based browser and a ChatGPT account. There is no official API.
- Snap Chromium cannot keep its profile under a hidden directory such as `~/.config/...`.

## Versions

This project follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). User-facing changes are listed in [`CHANGELOG.md`](CHANGELOG.md).

| Resource | Link |
| --- | --- |
| Latest release | [releases/latest](https://github.com/KaraBala10/chatbang-pro/releases/latest) |
| All releases | [releases](https://github.com/KaraBala10/chatbang-pro/releases) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| License | [LICENSE](LICENSE) (MIT) |
| Upstream | [ahmedhosssam/chatbang](https://github.com/ahmedhosssam/chatbang) |

## License

[MIT](LICENSE) © 2025 Ahmed Hossam, 2026 [KaraBala](https://github.com/KaraBala10)

## Build from source

Only needed if you are changing the code. Users should install from the [latest release](https://github.com/KaraBala10/chatbang-pro/releases/latest).

Requires [Go 1.26+](https://go.dev/dl/).

```bash
git clone https://github.com/KaraBala10/chatbang-pro.git
cd chatbang-pro
./build.sh
```

`./build.sh` installs to `~/.local/bin/chatbang-pro`. Set `SYSTEM_INSTALL=1` to also copy into `/usr/bin`. Set `SKIP_INSTALL=1` to build in the repo only.
