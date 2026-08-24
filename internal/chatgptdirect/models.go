package chatgptdirect

import "fmt"

type Model struct {
	Name           string
	Slug           string
	ThinkingEffort string
}

var supportedModels = []Model{
	{Name: "chatgpt/gpt-5.6", Slug: "gpt-5-6"},
	{Name: "chatgpt/gpt-5.6-thinking", Slug: "gpt-5-6-thinking", ThinkingEffort: "extended"},
	{Name: "chatgpt/gpt-5.6-pro", Slug: "gpt-5-6-thinking", ThinkingEffort: "extended"},
}

func ResolveModel(name string) (Model, error) {
	for _, model := range supportedModels {
		if model.Name == name {
			return model, nil
		}
	}
	return Model{}, fmt.Errorf("unsupported direct ChatGPT model: %s", name)
}

func ModelNames() []string {
	values := make([]string, 0, len(supportedModels))
	for _, model := range supportedModels {
		values = append(values, model.Name)
	}
	return values
}
