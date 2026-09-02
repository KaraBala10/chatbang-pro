package session

import (
	"encoding/json"
	"strings"
)

type parsedTurn struct {
	Text      string
	ImageURLs []string
	AssetIDs  []string
	HasImage  bool
}

func parseConversationSSE(raw string) parsedTurn {
	var turn parsedTurn
	seenURL := map[string]bool{}
	seenAsset := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err != nil {
			continue
		}
		applySSEObject(obj, &turn.Text)
		scanImages(obj, &turn, seenURL, seenAsset)
	}
	turn.Text = strings.TrimSpace(turn.Text)
	turn.HasImage = turn.HasImage || len(turn.ImageURLs) > 0 || len(turn.AssetIDs) > 0
	return turn
}

func applySSEObject(obj map[string]any, text *string) {
	if msg, _ := obj["message"].(map[string]any); msg != nil {
		author, _ := msg["author"].(map[string]any)
		role, _ := author["role"].(string)
		if role == "assistant" {
			if parts := assistantParts(msg); parts != "" {
				*text = parts
			}
		}
	}

	switch v := obj["v"].(type) {
	case string:
		*text += v
	case []any:
		for _, item := range v {
			op, _ := item.(map[string]any)
			if op == nil {
				continue
			}
			chunk, _ := op["v"].(string)
			if chunk == "" {
				continue
			}
			path, _ := op["p"].(string)
			if path != "" && !strings.Contains(path, "content/parts") {
				continue
			}
			if op["o"] == "replace" {
				*text = chunk
			} else {
				*text += chunk
			}
		}
	}
}

func assistantParts(msg map[string]any) string {
	content, _ := msg["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		if s, ok := part.(string); ok {
			b.WriteString(s)
		}
	}
	return b.String()
}

func scanImages(v any, turn *parsedTurn, seenURL, seenAsset map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if ct, _ := x["content_type"].(string); isImageContentType(ct) {
			turn.HasImage = true
		}
		if name, _ := x["name"].(string); isImageToolName(name) {
			turn.HasImage = true
		}
		if recipient, _ := x["recipient"].(string); isImageToolName(recipient) {
			turn.HasImage = true
		}
		if _, ok := x["image_gen"]; ok {
			turn.HasImage = true
		}
		if pointer, _ := x["asset_pointer"].(string); pointer != "" {
			turn.HasImage = true
			if id := fileIDFromPointer(pointer); id != "" && !seenAsset[id] {
				seenAsset[id] = true
				turn.AssetIDs = append(turn.AssetIDs, id)
			}
		}
		for _, key := range []string{"url", "download_url", "downloadUrl", "src", "image_url"} {
			if u := stringValue(x[key]); isGeneratedImageURL(u) && !seenURL[u] {
				seenURL[u] = true
				turn.ImageURLs = append(turn.ImageURLs, u)
				turn.HasImage = true
			}
		}
		for _, child := range x {
			scanImages(child, turn, seenURL, seenAsset)
		}
	case []any:
		for _, item := range x {
			scanImages(item, turn, seenURL, seenAsset)
		}
	case string:
		if isGeneratedImageURL(x) && !seenURL[x] {
			seenURL[x] = true
			turn.ImageURLs = append(turn.ImageURLs, x)
			turn.HasImage = true
		}
		if id := fileIDFromPointer(x); strings.Contains(x, "file-service://") && id != "" && !seenAsset[id] {
			seenAsset[id] = true
			turn.AssetIDs = append(turn.AssetIDs, id)
			turn.HasImage = true
		}
	}
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if u, _ := t["url"].(string); u != "" {
			return u
		}
	}
	return ""
}

func isImageContentType(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "image")
}

func isImageToolName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "dalle") || strings.Contains(n, "image_gen") || strings.Contains(n, "t2i") || strings.Contains(n, "text2im")
}
