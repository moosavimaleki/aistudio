package aistudio

import "fmt"

func EncodeStruct(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		object = map[string]any{}
	}
	entries := make([]any, 0, len(object))
	for key, item := range object {
		entries = append(entries, []any{key, EncodeValue(item)})
	}
	return []any{entries}
}
func EncodeValue(value any) any {
	switch item := value.(type) {
	case nil:
		return []any{0}
	case bool:
		return []any{nil, nil, nil, item}
	case float64:
		return []any{nil, item}
	case int:
		return []any{nil, item}
	case string:
		return []any{nil, nil, item}
	case map[string]any:
		return []any{nil, nil, nil, nil, EncodeStruct(item)}
	case []any:
		values := make([]any, 0, len(item))
		for _, nested := range item {
			values = append(values, EncodeValue(nested))
		}
		return []any{nil, nil, nil, nil, nil, []any{values}}
	default:
		panic(fmt.Sprintf("Unsupported protobuf Struct value: %T", value))
	}
}
