package aistudio

import (
	"fmt"
	"sort"
	"strings"
)

var schemaTypes = map[string]int{"string": 1, "number": 2, "integer": 3, "boolean": 4, "array": 5, "object": 6, "null": 7}
var supportedSchemaKeys = map[string]bool{"type": true, "format": true, "description": true, "nullable": true, "enum": true, "items": true, "properties": true, "required": true}

func EncodeSchema(value any) (any, error) {
	schema, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("responseSchema must be an object")
	}
	unknown := []string{}
	for key := range schema {
		if !supportedSchemaKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("Unsupported responseSchema fields: %s", strings.Join(unknown, ", "))
	}
	typeName := strings.ToLower(fmt.Sprint(schema["type"]))
	typeEnum, ok := schemaTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("responseSchema.type is required and must be supported")
	}
	encoded := make([]any, 8)
	encoded[0], encoded[1], encoded[2], encoded[3], encoded[4] = typeEnum, schema["format"], schema["description"], schema["nullable"], schema["enum"]
	if raw := schema["items"]; raw != nil {
		item, err := EncodeSchema(raw)
		if err != nil {
			return nil, err
		}
		encoded[5] = item
	}
	if raw := schema["properties"]; raw != nil {
		properties, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("responseSchema.properties must be an object")
		}
		entries := make([]any, 0, len(properties))
		for name, property := range properties {
			item, err := EncodeSchema(property)
			if err != nil {
				return nil, err
			}
			entries = append(entries, []any{name, item})
		}
		encoded[6] = entries
	}
	encoded[7] = schema["required"]
	return trim(encoded), nil
}
