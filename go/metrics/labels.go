package metrics

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

func Field(name string, labels map[string]any) string {
	cleanName := clean(name, 80)
	if len(labels) == 0 {
		return cleanName
	}
	keys := make([]string, 0, len(labels))
	for key, value := range labels {
		if value != nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, url.QueryEscape(clean(key, 32))+"="+url.QueryEscape(dimension(labels[key])))
	}
	if len(pairs) == 0 {
		return cleanName
	}
	return cleanName + "|" + strings.Join(pairs, "&")
}

func ParseField(value string) (string, map[string]string) {
	name, encoded, found := strings.Cut(value, "|")
	labels := map[string]string{}
	if !found {
		return name, labels
	}
	for _, pair := range strings.Split(encoded, "&") {
		key, item, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		decodedKey, keyErr := url.QueryUnescape(key)
		decodedItem, itemErr := url.QueryUnescape(item)
		if keyErr == nil && itemErr == nil {
			labels[decodedKey] = decodedItem
		}
	}
	return name, labels
}

func dimension(value any) string { return clean(toString(value), 80) }
func toString(value any) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(value)
}
func clean(value string, limit int) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("._-:/", character) {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	result := builder.String()
	if result == "" {
		result = "unknown"
	}
	runes := []rune(result)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return result
}
