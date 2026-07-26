package aistudio

import "fmt"

func EncodeGenerationConfig(config map[string]any) ([]any, error) {
	if config == nil {
		config = map[string]any{}
	}
	encoded := make([]any, 19)
	encoded[0] = configValue(config, "candidateCount", "candidate_count")
	encoded[1] = configValue(config, "stopSequences", "stop_sequences")
	encoded[3] = configDefault(config, 65536, "maxOutputTokens", "max_output_tokens")
	encoded[4] = configDefault(config, 1, "temperature")
	encoded[5] = configDefault(config, .95, "topP", "top_p")
	encoded[6] = configDefault(config, 64, "topK", "top_k")
	encoded[7] = configValue(config, "responseMimeType", "response_mime_type")
	if schema := configValue(config, "responseSchema", "response_schema"); schema != nil {
		if encoded[7] == nil {
			encoded[7] = "application/json"
		}
		value, err := EncodeSchema(schema)
		if err != nil {
			return nil, err
		}
		encoded[8] = value
	}
	encoded[14] = configValue(config, "responseModalities", "response_modalities")
	encoded[15] = configValue(config, "speechConfig", "speech_config")
	thinking, err := encodeThinkingConfig(configValue(config, "thinkingConfig", "thinking_config"))
	if err != nil {
		return nil, err
	}
	encoded[16] = thinking
	encoded[17] = configValue(config, "mediaResolution", "media_resolution")
	encoded[18] = configValue(config, "seed")
	return trim(encoded), nil
}

func configValue(source map[string]any, names ...string) any {
	for _, name := range names {
		if value, ok := source[name]; ok {
			return value
		}
	}
	return nil
}
func configDefault(source map[string]any, fallback any, names ...string) any {
	if value := configValue(source, names...); value != nil {
		return value
	}
	return fallback
}
func encodeThinkingConfig(value any) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("thinkingConfig must set thinkingBudget or levelEnum")
	}
	config, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("thinkingConfig must be an object")
	}
	budget := configValue(config, "thinkingBudget", "thinking_budget")
	level := configValue(config, "levelEnum", "level_enum", "thinkingLevel", "thinking_level")
	if budget != nil && level != nil {
		return nil, fmt.Errorf("thinkingConfig cannot set budget and level together")
	}
	if budget == nil && level == nil {
		return nil, fmt.Errorf("thinkingConfig must set thinkingBudget or levelEnum")
	}
	if text, ok := level.(string); ok {
		number, found := map[string]int{"LOW": 1, "MEDIUM": 2, "HIGH": 3, "MINIMAL": 4}[text]
		if !found {
			return nil, fmt.Errorf("Unsupported thinkingLevel: %s", text)
		}
		level = number
	}
	switch level.(type) {
	case nil, int, int64, float64:
	default:
		return nil, fmt.Errorf("thinkingLevel must be a known string or numeric levelEnum")
	}
	include := false
	if raw, ok := config["includeThoughts"]; ok {
		include, _ = raw.(bool)
	}
	if raw, ok := config["include_thoughts"]; ok {
		include, _ = raw.(bool)
	}
	return []any{include, budget, nil, level}, nil
}
