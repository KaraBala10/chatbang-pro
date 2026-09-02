package session

import "testing"

func TestParseConversationSSEMessage(t *testing.T) {
	raw := "data: {\"conversation_id\":\"c-1\",\"message\":{\"id\":\"m-1\",\"author\":{\"role\":\"assistant\"},\"content\":{\"content_type\":\"text\",\"parts\":[\"pong\"]}}}\n\ndata: [DONE]\n"
	if got := parseConversationSSE(raw); got.Text != "pong" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseConversationSSEDeltas(t *testing.T) {
	raw := "data: {\"v\":\"po\"}\ndata: {\"v\":\"ng\"}\ndata: [DONE]\n"
	if got := parseConversationSSE(raw); got.Text != "pong" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseConversationSSEPatches(t *testing.T) {
	raw := "data: {\"v\":[{\"p\":\"/message/content/parts/0\",\"o\":\"append\",\"v\":\"hello\"}]}\n"
	if got := parseConversationSSE(raw); got.Text != "hello" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseConversationSSEIgnoresUser(t *testing.T) {
	raw := "data: {\"message\":{\"author\":{\"role\":\"user\"},\"content\":{\"parts\":[\"secret\"]}}}\n"
	if got := parseConversationSSE(raw); got.Text != "" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseConversationSSEImageAsset(t *testing.T) {
	raw := "data: {\"message\":{\"author\":{\"role\":\"assistant\"},\"content\":{\"content_type\":\"multimodal_text\",\"parts\":[{\"content_type\":\"image_asset_pointer\",\"asset_pointer\":\"file-service://file-abc123\",\"width\":1024,\"height\":1024}]}}}\n"
	got := parseConversationSSE(raw)
	if !got.HasImage {
		t.Fatalf("expected image turn: %#v", got)
	}
	if len(got.AssetIDs) != 1 || got.AssetIDs[0] != "file-abc123" {
		t.Fatalf("assets = %#v", got.AssetIDs)
	}
}
