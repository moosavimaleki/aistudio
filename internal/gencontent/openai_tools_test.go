package gencontent

import (
	"encoding/json"
	"strings"
	"testing"
)

func weatherTool() openAITool {
	tool := openAITool{Type: "function"}
	tool.Function.Name = "weather"
	tool.Function.Description = "Read the weather for a city."
	tool.Function.Parameters = json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)
	return tool
}

func TestToolPlanExtractsNestedJSONWithoutRegex(t *testing.T) {
	plan, err := parseToolPlan([]openAITool{weatherTool()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := `Before <tool>{"name":"weather","arguments":{"city":"Tehran","filters":{"literal":"{} </tool> is text"}}}</tool> After`
	calls, content := plan.extract(text)
	if len(calls) != 1 || calls[0].Function.Name != "weather" ||
		calls[0].Function.Arguments != `{"city":"Tehran","filters":{"literal":"{} </tool> is text"}}` ||
		content != "Before  After" {
		t.Fatalf("calls=%#v content=%q", calls, content)
	}
}

func TestToolPlanAcceptsLegacyToolCallTag(t *testing.T) {
	plan, err := parseToolPlan([]openAITool{weatherTool()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls, content := plan.extract(`<tool_call name="weather">{"name":"weather","arguments":{"city":"Tehran"}}</tool_call>`)
	if len(calls) != 1 || calls[0].Function.Name != "weather" || content != "" {
		t.Fatalf("calls=%#v content=%q", calls, content)
	}
}

func TestToolPlanChoicePolicies(t *testing.T) {
	tools := []openAITool{weatherTool()}
	falseValue := false
	tests := []struct {
		name         string
		choice       string
		parallel     *bool
		wantEnabled  bool
		wantRequired bool
		wantForced   string
	}{
		{name: "default", wantEnabled: true},
		{name: "none", choice: `"none"`, wantEnabled: false},
		{name: "required", choice: `"required"`, wantEnabled: true, wantRequired: true},
		{name: "function", choice: `{"type":"function","function":{"name":"weather"}}`, wantEnabled: true, wantRequired: true, wantForced: "weather"},
		{name: "single call", parallel: &falseValue, wantEnabled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := parseToolPlan(tools, json.RawMessage(test.choice), test.parallel)
			if err != nil || plan.enabled() != test.wantEnabled || plan.requiresCall() != test.wantRequired || plan.forced != test.wantForced {
				t.Fatalf("plan=%#v err=%v", plan, err)
			}
		})
	}
}

func TestDirectMessagesPlaceToolInstructionAfterCallerSystem(t *testing.T) {
	plan, err := parseToolPlan([]openAITool{weatherTool()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := directMessages([]chatMessage{
		{Role: "system", Content: json.RawMessage(`"Caller policy"`)},
		{Role: "user", Content: json.RawMessage(`"Weather in Tehran?"`)},
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[0].Content != "Caller policy" ||
		!strings.Contains(messages[1].Content, "API tool adapter") || messages[2].Content != "Weather in Tehran?" {
		t.Fatalf("unexpected tool instruction order: %#v", messages)
	}
}

func TestToolPlanRejectsUnknownForcedFunction(t *testing.T) {
	_, err := parseToolPlan([]openAITool{weatherTool()}, json.RawMessage(`{"type":"function","function":{"name":"unknown"}}`), nil)
	if err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Fatalf("err=%v", err)
	}
}

func TestReadOnlyShellCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{command: "docker ps -a", want: true},
		{command: "git status", want: true},
		{command: "systemctl status docker", want: true},
		{command: "docker ps && rm -rf /tmp/example", want: false},
		{command: "docker ps | sh", want: false},
		{command: "git config --global user.name test", want: false},
		{command: "systemctl restart docker", want: false},
		{command: "find . -delete", want: false},
		{command: "sed -i s/a/b/ file", want: false},
		{command: "tail -f app.log", want: false},
	}
	for _, test := range tests {
		if got := readOnlyShellCommand(test.command); got != test.want {
			t.Errorf("readOnlyShellCommand(%q) = %t, want %t", test.command, got, test.want)
		}
	}
}
