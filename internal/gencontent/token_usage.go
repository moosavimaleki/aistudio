package gencontent

import (
	"strings"
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var (
	usageCodec     tokenizer.Codec
	usageCodecOnce sync.Once
)

func estimateChatUsage(messages []chatMessage, completion string) openAIUsage {
	codec := tokenCodec()
	prompt := 3
	for _, message := range messages {
		text, _ := chatContentText(message.Content)
		prompt += 3 + countTokens(codec, strings.TrimSpace(message.Role)) + countTokens(codec, text)
	}
	completed := countTokens(codec, completion)
	return openAIUsage{PromptTokens: prompt, CompletionTokens: completed, TotalTokens: prompt + completed}
}

func tokenCodec() tokenizer.Codec {
	usageCodecOnce.Do(func() {
		usageCodec, _ = tokenizer.Get(tokenizer.O200kBase)
	})
	return usageCodec
}

func countTokens(codec tokenizer.Codec, text string) int {
	if codec == nil || text == "" {
		return 0
	}
	count, err := codec.Count(text)
	if err != nil {
		return 0
	}
	return count
}
