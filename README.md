# ⚡ Chatbang Pro

> **ChatGPT from your terminal** — the full web experience. No API key. No quotas. No limits.

Chatbang Pro automates the official [ChatGPT](https://chatgpt.com) app in Chrome — every model, custom GPT, image generation, and feature OpenAI ships, from a fast, scriptable CLI.

| | |
|:---:|:---:|
| 🔑 **No API key** | 🆓 **No quotas** |
| 🧠 **Full ChatGPT** | ⚡ **Terminal-native** |
| 🖼️ **Image generation** | 🌍 **Unicode & RTL** |

Enhanced, actively maintained fork of [chatbang](https://github.com/ahmedhosssam/chatbang) — built for power users who want reliability, long replies, and headless operation.

---

## 📦 Installation

Download and install from the **[Releases](https://github.com/KaraBala10/chatbang-pro/releases)** page.

From source:

```bash
git clone https://github.com/KaraBala10/chatbang-pro.git
cd chatbang-pro
./build.sh
```

`./build.sh` installs to `~/.local/bin/chatbang-pro`. Use `SYSTEM_INSTALL=1 ./build.sh` to also copy into `/usr/bin`.

## 🌐 Requirements

A Chromium-based browser (Google Chrome, Chromium, Edge, Brave, etc.). On Debian/Ubuntu amd64, Google Chrome:

```bash
curl -fL https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb \
  -o /tmp/google-chrome-stable_current_amd64.deb

sudo apt install /tmp/google-chrome-stable_current_amd64.deb
```

Ubuntu Snap Chromium works; its profile must be a non-hidden folder such as `~/chatbang/profile_data`.

## ⚙️ Setup

```bash
chatbang-pro --config
```

Pick a browser that's already logged in to ChatGPT to import that session, or log in in the visible window and press **Enter**.

Optional config at `$HOME/.config/chatbang/chatbang`:

```
browser=/usr/bin/google-chrome
headless=true
profile=/home/you/chatbang/profile_data
```

## 💬 Usage

```bash
chatbang-pro              # 🚀 start chat
chatbang-pro --config     # 🔐 import login / refresh
chatbang-pro -g g-XXXX    # 🎯 custom GPT (full URL or g-... id)
chatbang-pro -m "hello"   # 📜 one prompt, then exit
chatbang-pro --help       # 📖 full CLI reference (-h)
```

Replies are wrapped in a `<<<chatbang-pro` … `>>>` block. Generated images are saved under `~/chatbang/images` and listed as `[image] path`. After the first reply, the conversation URL is printed as `Conversation: https://chatgpt.com/c/…`.

Type `exit` or `quit` to leave. Run `chatbang-pro --help` for all flags and options.

---

<p align="center">
  <strong>Built for the terminal. Powered by ChatGPT.</strong> 🖥️✨
</p>
