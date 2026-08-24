package browserinterface

import "fmt"

func (f *Fleet) resolveAutomaticChatGPT() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if id, ok := f.nextReadyChatGPT(true); ok {
		return id, nil
	}
	if id, ok := f.nextReadyChatGPT(false); ok {
		return id, nil
	}
	if f.config.ChatGPTDefaultID == "" {
		return "", fmt.Errorf("No ChatGPT browser profile is configured")
	}
	return f.config.ChatGPTDefaultID, nil
}

func (f *Fleet) nextReadyChatGPT(requireCookies bool) (string, bool) {
	count := len(f.config.Browsers)
	for offset := 0; offset < count; offset++ {
		index := (f.chatGPTNext + offset) % count
		spec := f.config.Browsers[index]
		session := f.sessions[spec.ID]
		if spec.Provider != ProviderChatGPT || session == nil {
			continue
		}
		if !session.Ready() || !f.broker.connected(spec.ID) {
			continue
		}
		if requireCookies && session.CookieDiagnostics().Count == 0 {
			continue
		}
		f.chatGPTNext = (index + 1) % count
		return spec.ID, true
	}
	return "", false
}
