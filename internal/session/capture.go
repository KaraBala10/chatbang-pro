package session

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type conversationCapture struct {
	mu              sync.Mutex
	pending         *pendingConv
	imageGen        bool
	pendingImageIDs map[network.RequestID]string
	finishedImages  []network.RequestID
}

type pendingConv struct {
	once      sync.Once
	seenOnce  sync.Once
	done      chan struct{}
	seen      chan struct{}
	requestID network.RequestID
	status    int64
	stream    bool
	body      string
	err       error
}

func listenConversation(ctx context.Context) *conversationCapture {
	cap := &conversationCapture{
		pendingImageIDs: map[network.RequestID]string{},
	}
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.Request == nil {
				return
			}
			if isConversationPOST(e.Request.Method, e.Request.URL) {
				cap.noteConversation(e.RequestID, false, 0)
				return
			}
			if isLikelyGeneratedImage(e.Request.URL, "") {
				cap.mu.Lock()
				cap.imageGen = true
				cap.pendingImageIDs[e.RequestID] = e.Request.URL
				cap.mu.Unlock()
			}
		case *network.EventResponseReceived:
			mime := ""
			status := int64(0)
			respURL := ""
			if e.Response != nil {
				mime = e.Response.MimeType
				status = e.Response.Status
				respURL = e.Response.URL
			}
			stream := strings.Contains(strings.ToLower(mime), "event-stream")
			if stream && isConversationPOST("POST", respURL) {
				cap.noteConversation(e.RequestID, true, status)
			} else {
				cap.mu.Lock()
				p := cap.pending
				if p != nil && p.requestID == e.RequestID {
					p.status = status
				}
				cap.mu.Unlock()
			}
			cap.mu.Lock()
			if isLikelyGeneratedImage(respURL, mime) {
				cap.imageGen = true
				cap.pendingImageIDs[e.RequestID] = respURL
			}
			cap.mu.Unlock()
		case *network.EventLoadingFinished:
			cap.noteImageFinished(e.RequestID)
			if cap.matchesPending(e.RequestID) {
				id := e.RequestID
				go cap.complete(ctx, id, nil)
			}
		case *network.EventLoadingFailed:
			if !cap.matchesPending(e.RequestID) {
				return
			}
			msg := e.ErrorText
			if msg == "" {
				msg = "conversation request failed"
			}
			id := e.RequestID
			go cap.complete(ctx, id, errors.New(msg))
		}
	})
	return cap
}

func (c *conversationCapture) noteConversation(id network.RequestID, stream bool, status int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pending
	if p == nil {
		return
	}
	if p.requestID != "" && p.requestID != id && (p.stream && !stream) {
		return
	}
	p.requestID = id
	if status != 0 {
		p.status = status
	}
	if stream {
		p.stream = true
	}
	p.seenOnce.Do(func() { close(p.seen) })
}

func (c *conversationCapture) matchesPending(id network.RequestID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pending != nil && c.pending.requestID != "" && c.pending.requestID == id
}

func (c *conversationCapture) noteImageFinished(id network.RequestID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.pendingImageIDs[id]; !ok {
		return
	}
	c.finishedImages = append(c.finishedImages, id)
}

func (c *conversationCapture) begin() *pendingConv {
	p := &pendingConv{done: make(chan struct{}), seen: make(chan struct{})}
	c.mu.Lock()
	c.pending = p
	c.imageGen = false
	c.pendingImageIDs = map[network.RequestID]string{}
	c.finishedImages = nil
	c.mu.Unlock()
	return p
}

func (c *conversationCapture) complete(ctx context.Context, id network.RequestID, err error) {
	c.mu.Lock()
	p := c.pending
	if p == nil || p.requestID != id {
		c.mu.Unlock()
		return
	}
	if err != nil {
		p.err = err
		c.mu.Unlock()
		p.once.Do(func() { close(p.done) })
		return
	}
	c.mu.Unlock()

	raw, readErr := readNetworkBody(ctx, id)

	c.mu.Lock()
	if c.pending != p || p.requestID != id {
		c.mu.Unlock()
		return
	}
	p.body = raw
	if readErr != nil {
		p.err = readErr
	}
	c.mu.Unlock()
	p.once.Do(func() { close(p.done) })
}

func (c *conversationCapture) sawImageGen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.imageGen
}

func (c *conversationCapture) wait(ctx context.Context, p *pendingConv, baseline responseStatus, lastStatus *string, expectImage bool) (parsedTurn, bool) {
	if c == nil || p == nil {
		return parsedTurn{}, false
	}

	if lastStatus == nil {
		var local string
		lastStatus = &local
	}
	seen := false
	streamDone := false
	select {
	case <-p.seen:
		seen = true
	default:
	}
	select {
	case <-p.done:
		streamDone = true
	default:
	}

	tick := func() (parsedTurn, bool) {
		status, err := readResponseStatus(ctx, baseline)
		if err != nil {
			return parsedTurn{}, false
		}
		printChatStatus(status, lastStatus)
		if newImageThisTurn(baseline, status) && !status.ImageGenerating && !status.Generating {
			return parsedTurn{HasImage: true}, true
		}
		waitingImage := expectImage || c.sawImageGen() || status.ImagePending || status.ImageGenerating
		if waitingImage && !newImageThisTurn(baseline, status) && !status.HasImage {
			if streamDone {
				if turn, ok := c.readTurn(ctx, p); ok && turn.HasImage {
					return turn, true
				}
			}
			return parsedTurn{}, false
		}
		if streamDone {
			if turn, ok := c.readTurn(ctx, p); ok {
				if turn.HasImage || !(status.ImageGenerating || status.ImagePending) {
					return turn, true
				}
			}
		}
		if replyFinished(baseline, status) {
			text, done, err := confirmFullResponse(ctx, replySettleWait)
			if err == nil && done {
				if vis := visibleAssistantText(text); vis != "" {
					select {
					case <-p.done:
						streamDone = true
					default:
					}
					if streamDone {
						if turn, ok := c.readTurn(ctx, p); ok {
							if turn.HasImage || !(status.ImageGenerating || status.ImagePending) {
								return turn, true
							}
						}
					}
					return parsedTurn{Text: vis}, true
				}
			}
		}
		return parsedTurn{}, false
	}

	if turn, ok := tick(); ok {
		return turn, true
	}
	seenTimer := time.NewTimer(captureSeenTimeout)
	defer seenTimer.Stop()
	ticker := time.NewTicker(pollIntervalDone)
	defer ticker.Stop()
	timer := time.NewTimer(responseTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return parsedTurn{}, false
		case <-timer.C:
			return parsedTurn{}, false
		case <-p.seen:
			seen = true
			if !seenTimer.Stop() {
				select {
				case <-seenTimer.C:
				default:
				}
			}
		case <-seenTimer.C:
			if !seen {
				return parsedTurn{}, false
			}
		case <-p.done:
			streamDone = true
			if turn, ok := tick(); ok {
				return turn, true
			}
		case <-ticker.C:
			if turn, ok := tick(); ok {
				return turn, true
			}
			if !streamDone {
				continue
			}
			status, err := readResponseStatus(ctx, baseline)
			if err == nil && (status.Generating || status.ImageGenerating) {
				continue
			}
			return c.readTurn(ctx, p)
		}
	}
}

func (c *conversationCapture) readTurn(ctx context.Context, p *pendingConv) (parsedTurn, bool) {
	c.mu.Lock()
	id := p.requestID
	status := p.status
	err := p.err
	body := p.body
	c.mu.Unlock()
	if err != nil || (status != 0 && status != 200) || id == "" {
		return parsedTurn{}, false
	}
	raw := body
	if strings.TrimSpace(raw) == "" {
		got, readErr := readNetworkBody(ctx, id)
		if readErr != nil || strings.TrimSpace(got) == "" {
			return parsedTurn{}, false
		}
		raw = got
	}
	turn := parseConversationSSE(raw)
	if turn.Text == "" && !turn.HasImage {
		return parsedTurn{}, false
	}
	return turn, true
}

func (c *conversationCapture) drainImageBytes(ctx context.Context) [][]byte {
	c.mu.Lock()
	ids := append([]network.RequestID(nil), c.finishedImages...)
	c.finishedImages = nil
	c.mu.Unlock()
	var out [][]byte
	for _, id := range ids {
		raw, err := readNetworkBody(ctx, id)
		if err != nil || len(raw) < minGeneratedImageBytes {
			continue
		}
		out = append(out, []byte(raw))
	}
	return out
}

func readNetworkBody(ctx context.Context, id network.RequestID) (string, error) {
	var body []byte
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		body, err = network.GetResponseBody(id).Do(ctx)
		return err
	}))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func enableNetwork() chromedp.Action {
	const buf = 64 * 1024 * 1024
	return network.Enable().
		WithMaxTotalBufferSize(buf).
		WithMaxResourceBufferSize(buf)
}

func isConversationPOST(method, rawURL string) bool {
	if !strings.EqualFold(method, "POST") {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "chatgpt.com" && host != "www.chatgpt.com" && !strings.HasSuffix(host, ".chatgpt.com") && host != "chat.openai.com" {
		return false
	}
	path := strings.ToLower(strings.TrimSuffix(u.EscapedPath(), "/"))
	for _, skip := range []string{"sentinel", "gen_title", "requirements", "/prepare", "/init", "autocomplet"} {
		if strings.Contains(path, skip) {
			return false
		}
	}
	return strings.HasSuffix(path, "/conversation")
}
