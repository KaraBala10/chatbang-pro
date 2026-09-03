package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/KaraBala10/chatbang-pro/internal/chaturl"
	"github.com/KaraBala10/chatbang-pro/internal/config"
)

// Session drives a Chromium tab for one ChatGPT conversation target.
type Session struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	ctxCancel   context.CancelFunc
	chatURL     string
	profileDir  string
	imagesDir   string
	lastPeak    int
	statusLine  string
	shownChat   bool
	knownChats  map[string]bool
	convURL     string
	capture     *conversationCapture
}

// New opens a browser session and waits until the chat page is ready.
func New(browser, profileDir, imagesDir string, headless bool, chatTarget string) (*Session, error) {
	if err := config.PrepareProfile(profileDir); err != nil {
		return nil, err
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		config.AllocatorOptions(browser, profileDir, headless)...,
	)
	s := &Session{allocCtx: allocCtx, allocCancel: allocCancel, chatURL: chatTarget, profileDir: profileDir, imagesDir: imagesDir, knownChats: map[string]bool{}}
	if err := s.openTab(); err != nil {
		stopBrowser(s.ctx, s.ctxCancel, allocCancel, profileDir)
		return nil, err
	}
	return s, nil
}

// Close shuts down the browser session.
func (s *Session) Close() {
	stopBrowser(s.ctx, s.ctxCancel, s.allocCancel, s.profileDir)
	s.ctxCancel = nil
	s.allocCancel = nil
}

// RunTurn sends one prompt and prints the assistant reply to stdout.
func (s *Session) RunTurn(prompt string) {
	fmt.Fprintln(os.Stderr, "[Thinking...]")

	if err := s.prepareForPrompt(); err != nil {
		fatalChatErr(err)
	}
	if err := ensureCustomGPTPage(s.ctx, s.chatURL, chaturl.CustomGPTPathPrefix(s.chatURL)); err != nil {
		fatalChatErr(err)
	}

	result, peak, err := s.runTurn(prompt)
	s.lastPeak = peak
	if err != nil {
		if sendFailed(err) {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fatalChatErr(err)
	}
	fmt.Print(formatReplyBlock(string(result)))
	s.announceConversationURL()
}

func sendFailed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "could not send prompt") ||
		strings.Contains(msg, "composer not ready") ||
		strings.Contains(msg, "rate-limiting")
}

func (s *Session) runTurn(prompt string) ([]byte, int, error) {
	baseline, _ := evaluateResponseStatus(s.ctx)
	turn, captured, err := s.submitAndCapture(prompt, baseline)
	if err != nil {
		return nil, 0, err
	}

	var text string
	var images []savedImage
	var peak int
	if captured {
		images = s.collectImages(turn)
		text = turn.Text
		peak = max(len(turn.Text), 1)
	} else {
		expectImage := looksLikeImagePrompt(prompt) || (s.capture != nil && s.capture.sawImageGen())
		raw, p, waitErr := waitForResponse(s.ctx, expectImage, baseline, &s.statusLine)
		peak = p
		after, _ := readResponseStatus(s.ctx, baseline)
		images = s.collectImages(parsedTurn{HasImage: expectImage || newImageThisTurn(baseline, after)})
		if waitErr != nil && len(images) == 0 {
			return nil, peak, waitErr
		}
		text = string(raw)
		err = waitErr
	}
	if copied := copyAssistantReply(s.ctx); copied != "" {
		text = copied
	}
	out, ferr := formatTurn(text, images)
	if ferr != nil {
		if err != nil {
			return nil, peak, err
		}
		return nil, peak, ferr
	}
	if len(images) > 0 {
		_ = waitComposerSendable(s.ctx, 6*time.Second)
	}
	return out, max(peak, len(text), 1), nil
}

func (s *Session) submitAndCapture(prompt string, baseline responseStatus) (parsedTurn, bool, error) {
	if err := s.ensureComposerReady(); err != nil {
		return parsedTurn{}, false, err
	}
	pending := s.beginCapture()
	err := submitPrompt(s.ctx, prompt)
	if isSessionDead(err) {
		if recErr := s.recover(); recErr != nil {
			return parsedTurn{}, false, recErr
		}
		pending = s.beginCapture()
		err = submitPrompt(s.ctx, prompt)
	}
	if err != nil && strings.Contains(err.Error(), "could not send prompt") {
		if recErr := s.reloadConversation(); recErr == nil {
			pending = s.beginCapture()
			err = submitPrompt(s.ctx, prompt)
		}
	}
	if err != nil {
		return parsedTurn{}, false, fmt.Errorf("submit prompt: %w", err)
	}
	if looksLikeImagePrompt(prompt) {
		nudgeImageGeneration(s.ctx)
	}
	if pending == nil {
		return parsedTurn{}, false, nil
	}
	turn, ok := s.capture.wait(s.ctx, pending, baseline, &s.statusLine, looksLikeImagePrompt(prompt))
	return turn, ok, nil
}

func (s *Session) collectImages(turn parsedTurn) []savedImage {
	if s.imagesDir == "" {
		return nil
	}
	seen := map[string]bool{}
	var images []savedImage
	save := func(b []byte) {
		img, ok, err := saveImageBytes(s.imagesDir, b, seen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not save image: %v\n", err)
			return
		}
		if ok {
			images = append(images, img)
			fmt.Fprintf(os.Stderr, "Saved image: %s\n", img.Path)
			openSavedImage(img.Path)
		}
	}

	sawGen := false
	if s.capture != nil {
		for _, body := range s.capture.drainImageBytes(s.ctx) {
			save(body)
		}
		sawGen = s.capture.sawImageGen()
	}

	want := turn.HasImage || sawGen || len(images) > 0
	if !want {
		return images
	}

	trySources := func() {
		if s.capture != nil {
			for _, body := range s.capture.drainImageBytes(s.ctx) {
				save(body)
			}
		}
		if len(images) > 0 {
			return
		}
		urls := append([]string{}, turn.ImageURLs...)
		urls = append(urls, scrapeAssistantImageURLs(s.ctx)...)
		seenURL := map[string]bool{}
		for _, u := range urls {
			if seenURL[u] {
				continue
			}
			seenURL[u] = true
			var b []byte
			switch {
			case strings.HasPrefix(u, "data:image/"):
				b = decodeDataImage(u)
			case strings.HasPrefix(u, "blob:"):
				b = fetchURLViaPage(s.ctx, u)
			case strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://"):
				if isStaticImageURL(u) {
					continue
				}
				if isGeneratedImageURL(u) && !strings.Contains(strings.ToLower(u), "chatgpt.com") {
					got, err := downloadHTTP(u)
					if err == nil {
						b = got
					}
				}
				if len(b) == 0 {
					b = fetchURLViaPage(s.ctx, u)
				}
			}
			if len(b) > 0 {
				save(b)
				if len(images) > 0 {
					return
				}
			}
		}
		for _, id := range turn.AssetIDs {
			u := fetchFileDownloadURL(s.ctx, id)
			if u == "" {
				continue
			}
			b, err := downloadHTTP(u)
			if err != nil || len(b) == 0 {
				b = fetchURLViaPage(s.ctx, u)
			}
			if len(b) > 0 {
				save(b)
			}
		}
	}

	trySources()
	if len(images) > 0 {
		return images
	}

	deadline := time.Now().Add(imageSettleTimeout)
	for time.Now().Before(deadline) {
		if s.ctx.Err() != nil {
			break
		}
		trySources()
		if len(images) > 0 {
			return images
		}
		time.Sleep(time.Second)
	}
	trySources()
	if len(images) == 0 {
		urls := scrapeAssistantImageURLs(s.ctx)
		if len(urls) > 0 {
			fmt.Fprintf(os.Stderr, "Found image URLs but could not download them (%d).\n", len(urls))
		} else {
			fmt.Fprintln(os.Stderr, "No generated image file was captured.")
		}
	}
	return images
}

func (s *Session) beginCapture() *pendingConv {
	if s.capture == nil {
		return nil
	}
	return s.capture.begin()
}

// LoginProfile opens a visible browser for first-time setup.
func LoginProfile(browser, profileDir string) {
	if err := config.PrepareProfile(profileDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Opening browser for ChatGPT setup...")

	allocatorCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		config.AllocatorOptions(browser, profileDir, false)...,
	)
	ctx, ctxCancel := chromedp.NewContext(allocatorCtx, chromedp.WithErrorf(suppressChromedpNoise))
	var once sync.Once
	stop := func() {
		once.Do(func() { stopBrowser(ctx, ctxCancel, allocCancel, profileDir) })
	}
	defer stop()

	if err := chromedp.Run(ctx, chromedp.Navigate(chaturl.DefaultURL)); err != nil {
		log.Fatalf("Could not open ChatGPT in browser: %v", err)
	}

	fmt.Println()
	fmt.Println("A browser window should be open.")
	fmt.Println("  1. Log in to ChatGPT (if needed)")
	fmt.Println("  2. Start a chat so the page is ready")
	fmt.Println("  3. Return here and press Enter to save and close the browser")
	waitEnter()
	stop()
	fmt.Println("Configuration saved.")
}

func waitEnter() {
	fmt.Print("\nPress Enter when finished: ")
	reader := bufio.NewReader(os.Stdin)
	if _, err := reader.ReadString('\n'); err != nil {
		log.Fatal(err)
	}
}

func (s *Session) openTab() error {
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	s.ctx, s.ctxCancel = chromedp.NewContext(s.allocCtx, chromedp.WithErrorf(suppressChromedpNoise))
	s.capture = listenConversation(s.ctx)
	fmt.Fprintf(os.Stderr, "Opening %s…\n", s.chatURL)
	if err := chromedp.Run(s.ctx, enableNetwork(), chromedp.Navigate(s.chatURL)); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Waiting for chat to start…")
	if err := waitForChatReady(s.ctx, s.chatURL); err != nil {
		return err
	}
	s.snapshotChats()
	return nil
}

func isSessionDead(err error) bool {
	return err != nil && (errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "target closed"))
}

func (s *Session) recover() error {
	fmt.Fprintln(os.Stderr, "Reconnecting browser...")
	if err := s.openTab(); err != nil {
		return fmt.Errorf("could not reconnect browser: %w", err)
	}
	return nil
}

func (s *Session) announceConversationURL() {
	if s.shownChat {
		return
	}
	link := s.conversationURL()
	if link == "" {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && link == "" {
			time.Sleep(100 * time.Millisecond)
			link = s.conversationURL()
		}
	}
	if link == "" {
		return
	}
	s.shownChat = true
	s.convURL = link
	if i := strings.LastIndex(link, "/c/"); i >= 0 {
		s.knownChats[strings.ToLower(link[i+3:])] = true
	}
	fmt.Fprintf(os.Stderr, "Conversation: %s\n", link)
}

func (s *Session) snapshotChats() {
	if s.knownChats == nil {
		s.knownChats = map[string]bool{}
	}
	for _, id := range pageConversationIDs(s.ctx) {
		s.knownChats[id] = true
	}
}

func (s *Session) conversationURL() string {
	var loc string
	if err := chromedp.Run(s.ctx, chromedp.Location(&loc)); err == nil {
		if p := chaturl.ConversationPermalink(loc); p != "" {
			return p
		}
	}
	for _, id := range pageConversationIDs(s.ctx) {
		if s.knownChats[id] {
			continue
		}
		return "https://chatgpt.com/c/" + id
	}
	return ""
}

func pageConversationIDs(ctx context.Context) []string {
	js := `(() => {
		const re = /\/c\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/i;
		const ids = [];
		const seen = new Set();
		const add = (u) => {
			const m = String(u || "").match(re);
			if (!m) return;
			const id = m[1].toLowerCase();
			if (seen.has(id)) return;
			seen.add(id);
			ids.push(id);
		};
		add(location.href);
		for (const a of document.querySelectorAll('a[href*="/c/"]')) {
			add(a.getAttribute("href") || a.href);
		}
		return ids;
	})()`
	var ids []string
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &ids))
	return ids
}

func (s *Session) prepareForPrompt() error {
	if s.lastPeak <= largeResponseThreshold {
		return nil
	}
	fmt.Fprintln(os.Stderr, "Starting a fresh chat (last reply was large)...")
	s.lastPeak = 0
	s.shownChat = false
	s.convURL = ""
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.chatURL)); err != nil {
		return err
	}
	return waitForChatReady(s.ctx, s.chatURL)
}

func composerSendable(ctx context.Context) bool {
	js := `(() => {
		` + jsComposer + `
		return isSendableControl(composerButton());
	})()`
	var ok bool
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &ok))
	return ok
}

func (s *Session) conversationTarget() string {
	if p := chaturl.ConversationPermalink(s.convURL); p != "" {
		return p
	}
	if p := s.conversationURL(); p != "" {
		return p
	}
	var loc string
	if err := chromedp.Run(s.ctx, chromedp.Location(&loc)); err != nil {
		return ""
	}
	return chaturl.ConversationPermalink(loc)
}

func waitComposerSendable(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = dismissBlockingDialogs(ctx)
		if composerSendable(ctx) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func (s *Session) ensureComposerReady() error {
	_ = dismissBlockingDialogs(s.ctx)
	if waitComposerSendable(s.ctx, 5*time.Second) {
		return nil
	}
	_ = dismissStaleStop(s.ctx)
	if waitComposerSendable(s.ctx, 3*time.Second) {
		return nil
	}
	if err := s.reloadConversation(); err != nil {
		return err
	}
	if waitComposerSendable(s.ctx, 8*time.Second) {
		return nil
	}
	return fmt.Errorf("composer not ready after reload")
}

func (s *Session) reloadConversation() error {
	target := s.conversationTarget()
	if target == "" {
		return fmt.Errorf("no conversation url")
	}
	fmt.Fprintln(os.Stderr, "Composer was stuck; reloading the chat…")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(target)); err != nil {
		return err
	}
	if err := waitForChatReady(s.ctx, target); err != nil {
		return err
	}
	_ = waitForSendButton(s.ctx, 3*time.Second)
	return nil
}

func submitPrompt(ctx context.Context, prompt string) error {
	_ = dismissChatDialogs(ctx)
	if err := dismissStaleStop(ctx); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.WaitVisible(`#prompt-textarea`, chromedp.ByID)); err != nil {
		return err
	}
	before := pageUserCount(ctx)
	if err := fillPrompt(ctx, prompt); err != nil {
		return err
	}
	_ = waitForSendButton(ctx, 2*time.Second)
	if err := sendComposer(ctx); err != nil {
		return err
	}
	if promptSubmitted(ctx, prompt, before) {
		return nil
	}
	if err := sendComposerEnter(ctx); err != nil {
		return err
	}
	if promptSubmitted(ctx, prompt, before) {
		return nil
	}
	if err := sendComposer(ctx); err != nil {
		return err
	}
	if promptSubmitted(ctx, prompt, before) {
		return nil
	}
	if dismissChatDialogs(ctx) {
		_ = waitForSendButton(ctx, 2*time.Second)
		if err := fillPrompt(ctx, prompt); err != nil {
			return err
		}
		if err := sendComposer(ctx); err != nil {
			return err
		}
		if promptSubmitted(ctx, prompt, before) {
			return nil
		}
		return fmt.Errorf("ChatGPT is rate-limiting (too many requests); wait a few minutes and try again")
	}
	return fmt.Errorf("could not send prompt to ChatGPT")
}

func fillPrompt(ctx context.Context, prompt string) error {
	if err := chromedp.Run(ctx, chromedp.Click(`#prompt-textarea`, chromedp.ByID)); err != nil {
		return err
	}
	_ = chromedp.Run(ctx, chromedp.SendKeys(`#prompt-textarea`, kb.Control+"a"+kb.Backspace, chromedp.ByID))
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.InsertText(prompt).Do(ctx)
	})); err != nil {
		return err
	}
	want := strings.TrimSpace(prompt)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(pageComposerText(ctx), want) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return err
	}
	setPromptJS := fmt.Sprintf(`(() => {
		const text = %s;
		const el = document.querySelector('#prompt-textarea');
		if (!el) throw new Error('prompt textarea not found');
		el.focus();
		if (el.tagName === 'TEXTAREA' || el.tagName === 'INPUT') {
			const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
			const desc = Object.getOwnPropertyDescriptor(proto, 'value');
			if (desc && desc.set) desc.set.call(el, text); else el.value = text;
			el.dispatchEvent(new Event('input', { bubbles: true }));
			return 'input';
		}
		const sel = window.getSelection();
		const range = document.createRange();
		range.selectNodeContents(el);
		sel.removeAllRanges();
		sel.addRange(range);
		const ok = document.execCommand('insertText', false, text);
		const shown = (el.innerText || el.textContent || '').trim();
		if (!ok || (text && !shown)) {
			el.textContent = text;
			el.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: text }));
		}
		return 'editable';
	})()`, promptJSON)
	return chromedp.Run(ctx, chromedp.Evaluate(setPromptJS, nil))
}

func dismissStaleStop(ctx context.Context) error {
	clickJS := `(() => {
		` + jsComposer + `
		const b = composerButton();
		if (!isStopControl(b)) return 'ready';
		b.click();
		return 'clicked';
	})()`
	var st string
	if err := chromedp.Run(ctx, chromedp.Evaluate(clickJS, &st)); err != nil {
		return err
	}
	if st == "ready" {
		return nil
	}
	return waitForSendButton(ctx, 2*time.Second)
}

func waitForSendButton(ctx context.Context, timeout time.Duration) error {
	readyJS := `(() => {
		` + jsComposer + `
		return isSendableControl(composerButton());
	})()`
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ready bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(readyJS, &ready)); err != nil {
			return err
		}
		if ready {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func sendComposer(ctx context.Context) error {
	readyJS := `(() => {
		` + jsComposer + `
		return isSendableControl(composerButton());
	})()`
	var ready bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(readyJS, &ready)); err != nil {
		return err
	}
	if ready {
		return chromedp.Run(ctx, chromedp.Click(`#composer-submit-button`, chromedp.ByID))
	}
	return sendComposerEnter(ctx)
}

func sendComposerEnter(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.Click(`#prompt-textarea`, chromedp.ByID),
		chromedp.SendKeys(`#prompt-textarea`, kb.Enter, chromedp.ByID),
	)
}

func pageUserCount(ctx context.Context) int {
	js := `(() => {` + jsUserNodes + `
		return userNodes.length;
	})()`
	var n int
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &n))
	return n
}

func pageComposerText(ctx context.Context) string {
	js := `(() => {` + jsComposer + `
		return composerText();
	})()`
	var t string
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &t))
	return t
}

func promptSubmitted(ctx context.Context, prompt string, usersBefore int) bool {
	deadline := time.Now().Add(2 * time.Second)
	want := strings.TrimSpace(prompt)
	for time.Now().Before(deadline) {
		js := `(() => {
			` + jsComposer + jsUserNodes + `
			return {users: userNodes.length, text: composerText()};
		})()`
		var st struct {
			Users int    `json:"users"`
			Text  string `json:"text"`
		}
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &st)); err != nil {
			return false
		}
		if st.Users > usersBefore {
			return true
		}
		shown := strings.TrimSpace(st.Text)
		if shown == "" || (want != "" && shown != want && !strings.Contains(shown, want)) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func fatalChatErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "browser disconnected") {
		log.Fatal("browser session ended unexpectedly (Chrome disconnected); restart chatbang-pro and try again")
	}
	log.Fatal(err)
}
