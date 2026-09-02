package browsers

import "testing"

func TestFilterChatGPT(t *testing.T) {
	in := []Cookie{
		{Name: "keep", Domain: ".chatgpt.com", Value: "1"},
		{Name: "keep2", Domain: "auth.openai.com", Value: "2"},
		{Name: "drop", Domain: "example.com", Value: "3"},
		{Name: "keep3", Domain: ".www.chatgpt.com", Value: "4"},
	}
	got := FilterChatGPT(in)
	if len(got) != 3 {
		t.Fatalf("got %d cookies, want 3", len(got))
	}
}

func TestHasChatGPTSession(t *testing.T) {
	if HasChatGPTSession([]Cookie{{Name: "cf_clearance", Domain: ".chatgpt.com", Value: "x"}}) {
		t.Fatal("cloudflare cookie alone should not count as a session")
	}
	if !HasChatGPTSession([]Cookie{{Name: "__Secure-next-auth.session-token", Domain: ".chatgpt.com", Value: "tok"}}) {
		t.Fatal("session-token should count as a session")
	}
	if HasChatGPTSession([]Cookie{{Name: "__Secure-next-auth.session-token", Domain: ".chatgpt.com", Value: ""}}) {
		t.Fatal("empty session-token should not count")
	}
	if !HasChatGPTSession([]Cookie{{Name: "__Secure-next-auth.session-token.0", Domain: ".chatgpt.com", Value: "chunk"}}) {
		t.Fatal("chunked session-token should count")
	}
}
