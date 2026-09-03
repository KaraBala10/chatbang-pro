package session

import "testing"

func TestResponseStartedIgnoresBaselineImage(t *testing.T) {
	base := responseStatus{HasImage: true, ImageCount: 1, NodeCount: 1, Len: 10, Tail: "old"}
	if responseStarted(base, base, true) {
		t.Fatal("existing image should not count as a new reply")
	}
	if !responseStarted(base, responseStatus{Generating: true, ImageCount: 1, NodeCount: 1}, true) {
		t.Fatal("generating should start the wait")
	}
	if !responseStarted(base, responseStatus{HasImage: true, ImageCount: 2, NodeCount: 2}, true) {
		t.Fatal("new image should start the wait")
	}
	if !responseStarted(base, responseStatus{UserCount: 2, NodeCount: 1, Len: 10, Tail: "old"}, false) {
		t.Fatal("new user message should start the wait")
	}
}

func TestResponseAlreadyComplete(t *testing.T) {
	base := responseStatus{NodeCount: 1, Len: 10, Tail: "old", UserCount: 1}
	done := responseStatus{NodeCount: 2, Len: 24, Tail: "hello there", UserCount: 2}
	if !responseAlreadyComplete(base, done, false) {
		t.Fatal("finished new reply should be treated as complete")
	}
	if responseAlreadyComplete(base, base, false) {
		t.Fatal("unchanged page is not complete")
	}
	if responseAlreadyComplete(base, responseStatus{Generating: true, NodeCount: 2, UserCount: 2}, false) {
		t.Fatal("still generating is not complete")
	}
	imgDone := responseStatus{HasImage: true, ImageCount: 1, NodeCount: 2, Len: 1, UserCount: 2}
	if !responseAlreadyComplete(responseStatus{NodeCount: 1, UserCount: 1}, imgDone, true) {
		t.Fatal("finished image turn should be complete")
	}
	afterSend := responseStatus{HasImage: true, ImageCount: 1, NodeCount: 1, Len: 10, Tail: "old", UserCount: 2}
	if responseAlreadyComplete(responseStatus{HasImage: true, ImageCount: 1, NodeCount: 1, Len: 10, Tail: "old", UserCount: 1}, afterSend, false) {
		t.Fatal("follow-up send should not treat the previous image as the new reply")
	}
}

func TestNewImageThisTurn(t *testing.T) {
	base := responseStatus{HasImage: true, ImageCount: 1, NodeCount: 2, LastImage: "file_old"}
	if newImageThisTurn(base, base) {
		t.Fatal("same last image is not a new turn")
	}
	if newImageThisTurn(base, responseStatus{HasImage: true, ImageCount: 1, NodeCount: 3, LastImage: "file_old"}) {
		t.Fatal("cloned previous image is not a new turn")
	}
	if !newImageThisTurn(base, responseStatus{HasImage: true, ImageCount: 1, NodeCount: 3, LastImage: "file_new"}) {
		t.Fatal("new image file id is a new turn")
	}
}

func TestRefineResponseStatusFollowUp(t *testing.T) {
	base := responseStatus{HasImage: true, ImageCount: 1, NodeCount: 1, UserCount: 1, LastImage: "file_old"}
	waiting := refineResponseStatus(base, responseStatus{
		HasImage: true, ImageCount: 1, NodeCount: 1, UserCount: 2, LastImage: "file_old",
	})
	if !waiting.ImageGenerating || !waiting.Generating {
		t.Fatal("remove-background follow-up should show generating image")
	}
	if waiting.StatusLine != "Generating image..." {
		t.Fatalf("status %q", waiting.StatusLine)
	}

	cloned := refineResponseStatus(base, responseStatus{
		HasImage: true, ImageCount: 1, NodeCount: 2, UserCount: 2, LastImage: "file_old",
	})
	if !cloned.ImageGenerating {
		t.Fatal("cloned old image on a new turn is still generating")
	}

	done := refineResponseStatus(base, responseStatus{
		HasImage: true, ImageCount: 1, NodeCount: 2, UserCount: 2, LastImage: "file_new", Generating: true,
	})
	if done.Generating || done.ImageGenerating {
		t.Fatal("new file id should finish even if Stop answering is still visible")
	}
	if !newImageThisTurn(base, done) {
		t.Fatal("finished follow-up should count as a new image")
	}
}

func TestReplyFinishedIgnoresThinkingChrome(t *testing.T) {
	base := responseStatus{NodeCount: 0, UserCount: 0}
	thinking := responseStatus{NodeCount: 1, UserCount: 1, Len: 8, Tail: "Thinking"}
	if replyFinished(base, thinking) {
		t.Fatal("Thinking chrome is not a finished reply")
	}
	done := responseStatus{NodeCount: 1, UserCount: 1, Len: 40, Tail: "Hello! How can I help you today?", CopyReady: true}
	if !replyFinished(base, done) {
		t.Fatal("finished hello should be ready even if Stop answering is still visible")
	}
	heading := responseStatus{NodeCount: 1, UserCount: 1, Len: 22, Tail: "Listing Arabic numbers"}
	if replyFinished(base, heading) {
		t.Fatal("a heading without Copy is not a finished reply")
	}
	heading.CopyReady = true
	if !replyFinished(base, heading) {
		t.Fatal("heading with Copy should be finished")
	}
	if replyFinished(base, responseStatus{Generating: true, NodeCount: 1, UserCount: 1, Len: 5, Tail: "Hel"}) {
		t.Fatal("still generating is not finished")
	}
	if replyFinished(base, responseStatus{ImagePending: true, NodeCount: 1, UserCount: 1, Len: 40, Tail: "Hello"}) {
		t.Fatal("image gen div still pending is not a finished reply")
	}
}

func TestIsConversationPOST(t *testing.T) {
	cases := []struct {
		method, url string
		want        bool
	}{
		{"POST", "https://chatgpt.com/backend-anon/f/conversation", true},
		{"POST", "https://chatgpt.com/backend-api/conversation", true},
		{"GET", "https://chatgpt.com/backend-api/f/conversation", false},
		{"POST", "https://chatgpt.com/backend-api/conversation/gen_title", false},
		{"POST", "https://chatgpt.com/backend-api/sentinel/chat-requirements", false},
		{"POST", "https://chatgpt.com/backend-api/f/conversation?foo=1", true},
		{"POST", "https://chatgpt.com/backend-api/f/s/conversation", true},
		{"POST", "https://chatgpt.com/backend-api/f/conversation/prepare", false},
		{"POST", "https://chatgpt.com/backend-api/conversation/prepare", false},
		{"POST", "https://chatgpt.com/backend-api/conversation/init", false},
		{"POST", "https://chat.openai.com/backend-api/f/conversation", true},
	}
	for _, tc := range cases {
		if got := isConversationPOST(tc.method, tc.url); got != tc.want {
			t.Fatalf("%s %s: got %v want %v", tc.method, tc.url, got, tc.want)
		}
	}
}
