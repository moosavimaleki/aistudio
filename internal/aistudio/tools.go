package aistudio

import (
	"fmt"
	"sort"
	"strings"
)

func EncodeTools(tools []any) (any, error) {
	if tools == nil {
		return nil, nil
	}
	result := make([]any, 0, len(tools))
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Tool must be an object")
		}
		unknown := []string{}
		for name := range tool {
			if name != "codeExecution" && name != "functionDeclarations" {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, fmt.Errorf("Unsupported Tool fields: %s", strings.Join(unknown, ", "))
		}
		encoded := make([]any, 2)
		if tool["codeExecution"] != nil {
			encoded[0] = []any{}
		}
		if declarations := tool["functionDeclarations"]; declarations != nil {
			items, ok := declarations.([]any)
			if !ok {
				return nil, fmt.Errorf("functionDeclarations must be an array")
			}
			values := make([]any, 0, len(items))
			for _, item := range items {
				declaration, err := encodeFunctionDeclaration(item)
				if err != nil {
					return nil, err
				}
				values = append(values, declaration)
			}
			encoded[1] = values
		}
		if encoded[0] == nil && encoded[1] == nil {
			return nil, fmt.Errorf("Tool must set codeExecution or functionDeclarations")
		}
		result = append(result, trim(encoded))
	}
	return result, nil
}

func encodeFunctionDeclaration(value any) ([]any, error) {
	declaration, ok := value.(map[string]any)
	if !ok || declaration["name"] == nil || fmt.Sprint(declaration["name"]) == "" {
		return nil, fmt.Errorf("functionDeclarations[].name is required")
	}
	parameters := declaration["parameters"]
	if parameters == nil {
		parameters = declaration["parametersJsonSchema"]
	}
	response := declaration["response"]
	if response == nil {
		response = declaration["responseJsonSchema"]
	}
	var encodedParameters, encodedResponse any
	var err error
	if parameters != nil {
		encodedParameters, err = EncodeSchema(parameters)
		if err != nil {
			return nil, err
		}
	}
	if response != nil {
		encodedResponse, err = EncodeSchema(response)
		if err != nil {
			return nil, err
		}
	}
	return trim([]any{declaration["name"], declaration["description"], encodedParameters, encodedResponse}), nil
}
