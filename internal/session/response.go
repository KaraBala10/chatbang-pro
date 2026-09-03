package session

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type responseStatus struct {
	Generating      bool   `json:"generating"`
	HasImage        bool   `json:"hasImage"`
	ImageCount      int    `json:"imageCount"`
	ImageGenerating bool   `json:"imageGenerating"`
	ImagePending    bool   `json:"imagePending"`
	ImageFailed     bool   `json:"imageFailed"`
	LastImage       string `json:"lastImage"`
	StatusLine      string `json:"statusLine"`
	Len             int    `json:"len"`
	Tail            string `json:"tail"`
	NodeCount       int    `json:"nodeCount"`
	UserCount       int    `json:"userCount"`
}

func (s responseStatus) signature() string {
	return fmt.Sprintf("%d:%s", s.Len, s.Tail)
}

func completionThresholds(contentLen int) (stableNeeded int, confirmWait time.Duration) {
	if contentLen > 8000 {
		return stablePollsLarge, confirmDelayLarge
	}
	return stablePollsDefault, confirmDelayDefault
}

func evaluateResponseStatus(ctx context.Context) (responseStatus, error) {
	statusJS := `(() => {
		` + jsIsStreaming + `
		` + jsStopVisible + `
		` + jsImageHelpers + `
		` + jsReplyChrome + `
		` + jsAssistantNodes + `
		` + jsUserNodes + `
		const userCount = userNodes.length;
		const stop = isStopVisible();
		const streamingNode = document.querySelector('[data-is-streaming="true"]');
		if (!nodes.length) {
			const pendingRoot = document.body;
			const blockedMsg = imageGenBlockMessage(pendingRoot);
			if (blockedMsg) {
				return {generating: false, hasImage: false, imageGenerating: false, imagePending: false, imageFailed: true, lastImage: "", statusLine: "", nodeCount: 0, userCount: userCount, imageCount: 0, len: Math.max(blockedMsg.length, 1), tail: blockedMsg};
			}
			const pending = isImageGenPending(pendingRoot);
			const line = pending ? (imageGenStatusLine(pendingRoot) || "Generating image...") : "";
			return {generating: stop || pending, imageGenerating: pending, imagePending: pending, lastImage: "", statusLine: line, nodeCount: 0, userCount: userCount, imageCount: 0};
		}
		const last = nodes[nodes.length - 1];
		const root = imageRoot(last);
		const imgs = imageSrcs(root);
		const lastImage = imageKey(imgs[0] || "");
		const finished = hasFinishedImage(root);
		const blockedMsg = imageGenBlockMessage(root);
		const imagePending = !blockedMsg && isImageGenPending(root);
		const waitingOnImageFollowUp = !blockedMsg && stop && userCount > nodes.length && finished;
		const pending = imagePending || waitingOnImageFollowUp;
		const hasImage = !blockedMsg && (finished || imgs.length > 0);
		const streaming = isStillStreaming(last) || isStillStreaming(root) || !!streamingNode;
		const statusLine = pending ? (imageGenStatusLine(root) || "Generating image...") : "";
		const rawTc = assistantTextWithoutImageGen(last);
		const chrome = !blockedMsg && isReplyChrome(rawTc);
		const tc = chrome ? "" : rawTc;
		const len = tc.length;
		if (blockedMsg) {
			return {generating: false, hasImage: false, imageGenerating: false, imagePending: false, imageFailed: true, lastImage: lastImage, statusLine: "", imageCount: 0, len: Math.max(blockedMsg.length, 1), tail: blockedMsg, nodeCount: nodes.length, userCount: userCount};
		}
		if (finished && !pending) {
			return {generating: false, hasImage: true, imageGenerating: false, imagePending: false, lastImage: lastImage, statusLine: "", imageCount: Math.max(imgs.length, 1), len: Math.max(len, 1), tail: tc.substring(Math.max(0, len - 400)), nodeCount: nodes.length, userCount: userCount};
		}
		const stillWriting = streaming || imagePending || waitingOnImageFollowUp || chrome;
		if (stillWriting || pending || (stop && !len && !hasImage)) {
			return {generating: true, hasImage: hasImage, imageGenerating: pending, imagePending: imagePending, lastImage: lastImage, statusLine: statusLine || (chrome ? "Thinking..." : ""), imageCount: imgs.length, nodeCount: nodes.length, userCount: userCount};
		}
		if (!len && !hasImage) {
			return {generating: true, imageGenerating: false, imagePending: imagePending, lastImage: lastImage, statusLine: statusLine, nodeCount: nodes.length, userCount: userCount, imageCount: 0};
		}
		return {generating: false, hasImage: hasImage, imageGenerating: false, imagePending: imagePending, lastImage: lastImage, statusLine: "", imageCount: hasImage ? Math.max(imgs.length, 1) : imgs.length, len: hasImage ? Math.max(len, 1) : len, tail: tc.substring(Math.max(0, len - 400)), nodeCount: nodes.length, userCount: userCount};
	})()`

	var status responseStatus
	err := chromedp.Run(ctx, chromedp.Evaluate(statusJS, &status))
	return status, err
}

func readResponseStatus(ctx context.Context, baseline responseStatus) (responseStatus, error) {
	status, err := evaluateResponseStatus(ctx)
	if err != nil {
		return status, err
	}
	return refineResponseStatus(baseline, status), nil
}

// refineResponseStatus keeps image follow-ups (e.g. "remove background") in
// generating state until the last image file id changes, even if ChatGPT
// still shows Stop answering on a previously finished image.
func refineResponseStatus(baseline, s responseStatus) responseStatus {
	newImg := s.LastImage != "" && s.LastImage != baseline.LastImage
	onOld := baseline.LastImage != "" && s.LastImage == baseline.LastImage
	waiting := s.UserCount > baseline.UserCount && (s.NodeCount <= baseline.NodeCount || onOld)
	if waiting && !newImg && (onOld || s.LastImage == "") && (baseline.HasImage || baseline.LastImage != "" || baseline.ImageCount > 0) {
		s.ImageGenerating = true
		s.Generating = true
		if strings.TrimSpace(s.StatusLine) == "" {
			s.StatusLine = "Generating image..."
		}
	}
	if newImg && !s.ImagePending {
		s.Generating = false
		s.ImageGenerating = false
		s.StatusLine = ""
		s.HasImage = true
		if s.ImageCount == 0 {
			s.ImageCount = 1
		}
	}
	return s
}

func fetchFullResponse(ctx context.Context) (string, error) {
	textJS := `(() => {
		` + jsImageHelpers + `
		` + jsAssistantNodes + `
		if (!nodes.length) return "";
		return assistantTextWithoutImageGen(nodes[nodes.length - 1]);
	})()`

	var text string
	if err := chromedp.Run(ctx, chromedp.Evaluate(textJS, &text)); err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func renderResponse(text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("empty response from ChatGPT")
	}
	return []byte(strings.TrimSpace(text) + "\n"), nil
}

func confirmFullResponse(ctx context.Context, confirmWait time.Duration) (string, bool, error) {
	first, err := fetchFullResponse(ctx)
	if err != nil {
		return "", false, err
	}
	if err := chromedp.Run(ctx, chromedp.Sleep(confirmWait)); err != nil {
		return first, true, nil
	}
	second, err := fetchFullResponse(ctx)
	if err != nil {
		return first, true, nil
	}
	if second == first {
		return second, true, nil
	}
	if len(second) > len(first) {
		return second, false, nil
	}
	return second, true, nil
}

func maybeSavePartial(ctx context.Context, statusLen int, lastPartial *string, lastFetch *time.Time) {
	if statusLen <= len(*lastPartial) {
		return
	}
	if time.Since(*lastFetch) < partialMinGap && len(*lastPartial) > 0 {
		return
	}
	if text, err := fetchFullResponse(ctx); err == nil && len(text) > len(*lastPartial) {
		*lastPartial = text
		*lastFetch = time.Now()
	}
}

func responseStarted(baseline, status responseStatus, expectImage bool) bool {
	if status.Generating {
		return true
	}
	if status.UserCount > baseline.UserCount {
		return true
	}
	if status.ImageCount > baseline.ImageCount {
		return true
	}
	if status.NodeCount > baseline.NodeCount {
		return true
	}
	if expectImage {
		return false
	}
	if status.Len > 0 && status.signature() != baseline.signature() {
		return true
	}
	return false
}

func newImageThisTurn(baseline, status responseStatus) bool {
	if status.LastImage != "" && status.LastImage != baseline.LastImage {
		return true
	}
	if baseline.LastImage != "" && status.LastImage == baseline.LastImage {
		return false
	}
	if status.NodeCount > baseline.NodeCount && (status.HasImage || status.ImageCount > 0) {
		return true
	}
	return status.ImageCount > baseline.ImageCount
}

func printChatStatus(status responseStatus, last *string) {
	line := strings.Join(strings.Fields(status.StatusLine), " ")
	if line == "" && status.ImageGenerating {
		line = "Generating image..."
	}
	if line == "" {
		return
	}
	switch strings.ToLower(line) {
	case "thinking", "thinking...":
		return
	}
	if statusKey(line) == statusKey(*last) {
		return
	}
	if len([]rune(line)) > 80 {
		line = string([]rune(line)[:80])
	}
	fmt.Fprintf(os.Stderr, "[%s]\n", line)
	*last = line
}

func statusKey(line string) string {
	s := strings.ToLower(strings.Join(strings.Fields(line), " "))
	return strings.TrimRight(s, ".…")
}

func replyFinished(baseline, status responseStatus) bool {
	if imageGenFailed(status) {
		return true
	}
	if status.Generating || status.ImageGenerating || status.ImagePending {
		return false
	}
	if isStatusChrome(status.Tail) && !status.HasImage && status.ImageCount == 0 {
		return false
	}
	if newImageThisTurn(baseline, status) {
		return true
	}
	return responseAlreadyComplete(baseline, status, false)
}

func responseAlreadyComplete(baseline, status responseStatus, expectImage bool) bool {
	if status.Generating || status.ImageGenerating {
		return false
	}
	if status.NodeCount <= baseline.NodeCount {
		return expectImage && status.ImageCount > baseline.ImageCount
	}
	return status.Len > 0 || status.HasImage || status.ImageCount > 0
}

func grabFinishedReply(ctx context.Context, baseline, status responseStatus, imageMode bool) ([]byte, int, bool, error) {
	if imageGenFailed(status) {
		msg := visibleAssistantText(status.Tail)
		if msg == "" {
			text, _, _ := confirmFullResponse(ctx, 0)
			msg = visibleAssistantText(text)
		}
		if msg == "" {
			msg = visibleAssistantText(status.StatusLine)
		}
		if msg != "" {
			out, err := renderResponse(msg)
			return out, max(len(msg), 1), true, err
		}
	}
	if imageMode && newImageThisTurn(baseline, status) && !status.Generating && !status.ImageGenerating {
		return []byte(""), max(status.Len, status.ImageCount, 1), true, nil
	}
	if !replyFinished(baseline, status) {
		return nil, 0, false, nil
	}
	text, done, err := confirmFullResponse(ctx, replySettleWait)
	if err != nil {
		return nil, 0, false, err
	}
	if !done {
		return nil, 0, false, nil
	}
	if vis := visibleAssistantText(text); vis != "" {
		if imageMode && !newImageThisTurn(baseline, status) && !status.HasImage && !imageGenFailed(status) && !isImageGenFailureText(vis) {
			return nil, 0, false, nil
		}
		out, err := renderResponse(vis)
		return out, max(len(vis), 1), true, err
	}
	if imageMode && newImageThisTurn(baseline, status) {
		return []byte(""), max(status.Len, 1), true, nil
	}
	return nil, 0, false, nil
}

func waitForResponse(ctx context.Context, expectImage bool, baseline responseStatus, lastStatus *string) ([]byte, int, error) {
	deadline := time.Now().Add(responseTimeout)
	waitStart := time.Now()

	if lastStatus == nil {
		var local string
		lastStatus = &local
	}

	var lastSig string
	var lastPartial string
	var lastFetch time.Time
	var started bool
	var stableCount int
	var peakLen int
	var dismissedDialog bool

	returnPartial := func(warn string) ([]byte, int, error) {
		if lastPartial == "" {
			return nil, peakLen, fmt.Errorf("browser disconnected before any response was captured; restart chatbang-pro")
		}
		fmt.Fprintln(os.Stderr, warn)
		out, err := renderResponse(lastPartial)
		return out, max(peakLen, len(lastPartial)), err
	}

	imageMode := expectImage
	if status, err := readResponseStatus(ctx, baseline); err == nil {
		if out, n, ok, err := grabFinishedReply(ctx, baseline, status, imageMode); ok {
			return out, n, err
		}
	}

	var lastChatStatus *string = lastStatus
	announceImage := func(status responseStatus) {
		if status.ImageGenerating {
			imageMode = true
		}
		printChatStatus(status, lastChatStatus)
	}

	newContent := func(status responseStatus) bool {
		if status.ImageCount > baseline.ImageCount {
			return true
		}
		if status.NodeCount > baseline.NodeCount && status.signature() != baseline.signature() {
			return true
		}
		return false
	}

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return returnPartial("Warning: browser disconnected; showing last captured text.")
		}

		status, err := readResponseStatus(ctx, baseline)
		if err != nil {
			if ctx.Err() != nil {
				return returnPartial("Warning: browser disconnected; showing last captured text.")
			}
			time.Sleep(pollIntervalDone)
			continue
		}
		if dismissBlockingDialogs(ctx) && !dismissedDialog {
			fmt.Fprintln(os.Stderr, "Dismissed ChatGPT's Too many requests dialog.")
			dismissedDialog = true
		}

		if status.Len > peakLen {
			peakLen = status.Len
		}

		announceImage(status)

		if out, n, ok, err := grabFinishedReply(ctx, baseline, status, imageMode); ok {
			return out, n, err
		}
		if replyFinished(baseline, status) && !imageMode {
			stableCount = 0
			time.Sleep(pollIntervalActive)
			continue
		}

		pollSleep := pollIntervalDone
		if !started {
			if responseStarted(baseline, status, imageMode) {
				started = true
			} else {
				if time.Since(waitStart) > startWaitTimeout {
					return nil, peakLen, fmt.Errorf("ChatGPT did not start a reply; try sending again")
				}
				time.Sleep(pollIntervalDone)
				continue
			}
		}

		if status.Generating {
			stableCount = 0
			pollSleep = pollIntervalActive
		} else if imageMode && !newImageThisTurn(baseline, status) {
			stableCount = 0
			pollSleep = pollIntervalActive
		} else if imageMode && !newContent(status) {
			stableCount = 0
		} else if status.Len == 0 && !status.HasImage {
			stableCount = 0
		} else if started {
			if imageMode && !newImageThisTurn(baseline, status) {
				stableCount = 0
				time.Sleep(pollIntervalActive)
				continue
			}
			stableNeeded, confirmWait := completionThresholds(peakLen)
			sig := status.signature()
			if imageMode {
				sig = fmt.Sprintf("%s:%d:%d", sig, status.ImageCount, status.NodeCount)
			}
			if sig == lastSig {
				stableCount++
				if stableCount >= stableNeeded {
					text, done, err := confirmFullResponse(ctx, confirmWait)
					if err != nil {
						return returnPartial("Warning: could not fetch full reply; showing last captured text.")
					}
					if !done {
						lastSig = fmt.Sprintf("%d:%s:%d", len(text), text[max(0, len(text)-400):], status.ImageCount)
						stableCount = 0
						if len(text) > len(lastPartial) {
							lastPartial = text
						}
						time.Sleep(pollIntervalDone)
						continue
					}
					if imageMode && !newImageThisTurn(baseline, status) {
						stableCount = 0
						time.Sleep(pollIntervalActive)
						continue
					}
					if strings.TrimSpace(text) == "" && newImageThisTurn(baseline, status) {
						return []byte(""), max(peakLen, 1), nil
					}
					out, err := renderResponse(text)
					return out, max(peakLen, len(text)), err
				}
			} else {
				lastSig = sig
				stableCount = 1
				maybeSavePartial(ctx, status.Len, &lastPartial, &lastFetch)
			}
		}

		time.Sleep(pollSleep)
	}

	if lastPartial != "" {
		return returnPartial("Warning: timed out waiting for reply to finish; showing partial response.")
	}

	return nil, peakLen, fmt.Errorf("timed out after %s waiting for ChatGPT (very long replies may need several minutes)", responseTimeout)
}
