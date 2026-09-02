package browsers

import "fmt"

// Extract reads ChatGPT cookies from a discovered browser profile.
func Extract(src Source) ([]Cookie, error) {
	var (
		cookies []Cookie
		err     error
	)
	switch src.Kind {
	case KindFirefox:
		cookies, err = extractFirefox(src)
	case KindChromium:
		cookies, err = extractChromium(src)
	default:
		return nil, fmt.Errorf("unsupported browser: %s", src.Name)
	}
	if err != nil {
		return nil, err
	}

	cookies = FilterChatGPT(cookies)
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no ChatGPT cookies found in %s", src.Label())
	}
	if !HasChatGPTSession(cookies) {
		return nil, fmt.Errorf("found ChatGPT cookies in %s, but no login session — log in to ChatGPT in that browser first", src.Label())
	}
	return cookies, nil
}
