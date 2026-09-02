package chaturl

import (
	"strings"
	"testing"
)

func TestNormalizeCustomGPT(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "full URL",
			input: "https://chatgpt.com/g/g-abc123-my-gpt",
			want:  "https://chatgpt.com/g/g-abc123-my-gpt",
		},
		{
			name:  "path only",
			input: "/g/g-abc123-my-gpt",
			want:  "https://chatgpt.com/g/g-abc123-my-gpt",
		},
		{
			name:  "id only",
			input: "g-abc123-my-gpt",
			want:  "https://chatgpt.com/g/g-abc123-my-gpt",
		},
		{
			name:  "www host",
			input: "https://www.chatgpt.com/g/g-abc123",
			want:  "https://chatgpt.com/g/g-abc123",
		},
		{
			name:  "trailing slash",
			input: "https://chatgpt.com/g/g-abc123/",
			want:  "https://chatgpt.com/g/g-abc123",
		},
		{
			name:  "temporary chat in URL",
			input: "https://chatgpt.com/g/g-abc123?temporary-chat=true",
			want:  "https://chatgpt.com/g/g-abc123?temporary-chat=true",
		},
		{
			name:    "empty",
			input:   "  ",
			wantErr: true,
		},
		{
			name:    "wrong host",
			input:   "https://example.com/g/g-abc123",
			wantErr: true,
		},
		{
			name:    "not a GPT path",
			input:   "https://chatgpt.com/c/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCustomGPT(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithTemporaryChat(t *testing.T) {
	got := WithTemporaryChat("https://chatgpt.com/g/g-abc123")
	if !strings.Contains(got, "temporary-chat=true") {
		t.Fatalf("expected temporary-chat query param, got %q", got)
	}
}

func TestIsTemporary(t *testing.T) {
	if !IsTemporary(TempURL) {
		t.Fatal("TempURL should be detected as temporary")
	}
	if IsTemporary(DefaultURL) {
		t.Fatal("default chat URL should not be temporary")
	}
}

func TestConversationPermalink(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://chatgpt.com/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62", "https://chatgpt.com/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62"},
		{"https://chatgpt.com/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62?foo=1#bar", "https://chatgpt.com/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62"},
		{"https://chatgpt.com/c/WEB:7d9287bf-9150-4d37-8e98-799a1391aed5", ""},
		{"https://www.chatgpt.com/c/abc/", ""},
		{"https://chat.openai.com/c/abc", ""},
		{"https://chatgpt.com/g/g-81BdggBV3-site/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62", "https://chatgpt.com/g/g-81BdggBV3-site/c/6a97fa21-596c-83eb-9a58-553f4c1c1f62"},
		{"https://chatgpt.com/", ""},
		{"https://chatgpt.com/?temporary-chat=true", ""},
		{"https://example.com/c/abc", ""},
	}
	for _, tt := range tests {
		if got := ConversationPermalink(tt.in); got != tt.want {
			t.Fatalf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestCustomGPTPathPrefix(t *testing.T) {
	got := CustomGPTPathPrefix("https://chatgpt.com/g/g-abc123-my-gpt")
	want := "/g/g-abc123-my-gpt"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if CustomGPTPathPrefix(DefaultURL) != "" {
		t.Fatal("default chat URL should have empty prefix")
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name          string
		temporaryChat bool
		customGPT     string
		want          string
		wantErr       bool
	}{
		{name: "default", want: DefaultURL},
		{name: "temporary only", temporaryChat: true, want: TempURL},
		{name: "custom GPT", customGPT: "g-abc123", want: "https://chatgpt.com/g/g-abc123"},
		{
			name:          "custom GPT with temp flag",
			temporaryChat: true,
			customGPT:     "g-abc123",
			want:          "https://chatgpt.com/g/g-abc123?temporary-chat=true",
		},
		{name: "invalid GPT", customGPT: "not-a-gpt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.temporaryChat, tt.customGPT)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
