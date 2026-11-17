package browserinterface

func (f *Fleet) startsAtBoot(spec BrowserSpec) bool {
	if f.config.ChatGPTRotateEvery > 0 && spec.Provider == ProviderChatGPT {
		return spec.ID == f.config.ChatGPTDefaultID
	}
	return !f.config.LazyStart || spec.ID == f.config.DefaultID || spec.ID == f.config.ChatGPTDefaultID
}

// All profiles stay addressable. Automatic selection can wake an unused
// account when no already-running ChatGPT browser is available.
func (f *Fleet) dormantChatGPT() (string, bool) {
	if !f.config.LazyStart {
		return "", false
	}
	for _, spec := range f.config.Browsers {
		session := f.sessions[spec.ID]
		if spec.Provider == ProviderChatGPT && session != nil && !session.started.Load() && f.warmErrors[spec.ID] == "" {
			return spec.ID, true
		}
	}
	return "", false
}
