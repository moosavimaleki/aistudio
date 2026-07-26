package gencontent

import "fmt"

type inlinePart struct {
	data, mimeType, displayName string
}

func readInlinePart(part map[string]any) (inlinePart, bool, error) {
	raw, found := firstValue(part, "inlineData", "inline_data")
	if !found {
		return inlinePart{}, false, nil
	}
	inline, ok := raw.(map[string]any)
	if !ok {
		return inlinePart{}, true, fmt.Errorf("inlineData must be an object")
	}
	data, dataOK := firstString(inline, "data")
	mimeType, mimeOK := firstString(inline, "mimeType", "mime_type")
	if !dataOK || !mimeOK || mimeType == "" {
		return inlinePart{}, true, fmt.Errorf("inlineData requires data and mimeType")
	}
	displayName, _ := firstString(inline, "displayName", "display_name")
	return inlinePart{data: data, mimeType: mimeType, displayName: displayName}, true, nil
}

func firstValue(values map[string]any, names ...string) (any, bool) {
	for _, name := range names {
		value, ok := values[name]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func firstString(values map[string]any, names ...string) (string, bool) {
	value, ok := firstValue(values, names...)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}
