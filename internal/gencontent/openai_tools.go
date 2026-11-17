package gencontent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	toolCallOpenTag    = "<tool>"
	toolCallCloseTag   = "</tool>"
	legacyToolOpenTag  = "<tool_call"
	legacyToolCloseTag = "</tool_call>"
)

type toolPlan struct {
	tools    []openAITool
	choice   string
	forced   string
	parallel bool
}

func parseToolPlan(tools []openAITool, choice json.RawMessage, parallel *bool) (toolPlan, error) {
	plan := toolPlan{parallel: true}
	if parallel != nil {
		plan.parallel = *parallel
	}
	for _, tool := range tools {
		if tool.Type != "function" || strings.TrimSpace(tool.Function.Name) == "" {
			return toolPlan{}, fmt.Errorf("only function tools with a name are supported")
		}
		if len(tool.Function.Parameters) > 0 && !json.Valid(tool.Function.Parameters) {
			return toolPlan{}, fmt.Errorf("tool %q parameters must be valid JSON", tool.Function.Name)
		}
		plan.tools = append(plan.tools, tool)
	}
	if len(choice) == 0 || strings.TrimSpace(string(choice)) == "null" {
		plan.choice = "auto"
		return plan, nil
	}
	if len(plan.tools) == 0 {
		return toolPlan{}, fmt.Errorf("tool_choice requires tools")
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	var value string
	if json.Unmarshal(choice, &value) == nil {
		switch value {
		case "auto", "none", "required", "any":
			plan.choice = value
			return plan, nil
		default:
			return toolPlan{}, fmt.Errorf("unsupported tool_choice: %s", value)
		}
	}
	if err := json.Unmarshal(choice, &named); err != nil || named.Type != "function" || strings.TrimSpace(named.Function.Name) == "" {
		return toolPlan{}, fmt.Errorf("tool_choice must be auto, none, required, any, or a function selector")
	}
	if !plan.allows(named.Function.Name) {
		return toolPlan{}, fmt.Errorf("tool_choice selects unknown function: %s", named.Function.Name)
	}
	plan.choice = "required"
	plan.forced = named.Function.Name
	return plan, nil
}

func (p toolPlan) enabled() bool {
	return len(p.tools) > 0 && p.choice != "none"
}

func (p toolPlan) requiresCall() bool {
	return p.enabled() && (p.choice == "required" || p.choice == "any")
}

func (p toolPlan) allows(name string) bool {
	if p.forced != "" && name != p.forced {
		return false
	}
	for _, tool := range p.tools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

func (p toolPlan) instruction() string {
	if !p.enabled() {
		return ""
	}
	available := make([]map[string]any, 0, len(p.tools))
	for _, tool := range p.tools {
		if !p.allows(tool.Function.Name) {
			continue
		}
		entry := map[string]any{"name": tool.Function.Name, "description": tool.Function.Description}
		if len(tool.Function.Parameters) > 0 {
			var parameters any
			if json.Unmarshal(tool.Function.Parameters, &parameters) == nil {
				entry["parameters"] = parameters
			}
		}
		available = append(available, entry)
	}
	schema, _ := json.Marshal(available)
	callRule := "Tools are available and the caller executes them. Use a tool whenever the answer depends on the local workspace, filesystem, terminal, installed software, containers, processes, network state, or any fact that can be verified with an available tool. Never claim that you lack terminal access, report a transient tool error, or merely suggest an unexecuted command when an applicable tool is available. Otherwise answer normally."
	if p.choice == "required" || p.choice == "any" {
		callRule = "You must invoke a tool and must not answer normally."
	}
	if p.forced != "" {
		callRule = "You must invoke the function " + p.forced + " and must not answer normally."
	}
	parallelRule := "You may return multiple tool calls when necessary."
	if !p.parallel {
		parallelRule = "Return at most one tool call."
	}
	return "You are operating through an API tool adapter. " + callRule + " " + parallelRule +
		" A tool invocation must be exactly " + toolCallOpenTag +
		"{\"name\":\"FUNCTION_NAME\",\"arguments\":{...}}" + toolCallCloseTag +
		". Arguments must be a JSON object matching the function schema. Do not invent functions. Available functions:\n" + string(schema)
}

func (p toolPlan) reminder() string {
	if !p.enabled() {
		return ""
	}
	names := make([]string, 0, len(p.tools))
	for _, tool := range p.tools {
		if p.allows(tool.Function.Name) {
			names = append(names, tool.Function.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "\n\n[Client tool protocol: the system tool contract is active. Tools are available via " +
		toolCallOpenTag + " JSON " + toolCallCloseTag + ": " + strings.Join(names, ", ") + ".]"
}

func (p toolPlan) repairPrompt(original string) string {
	names := make([]string, 0, len(p.tools))
	for _, tool := range p.tools {
		if p.allows(tool.Function.Name) {
			names = append(names, tool.Function.Name)
		}
	}
	return "Correct the previous reply by invoking one available API tool now. Do not claim that a tool is unavailable, do not answer with prose, and do not use a Markdown code block. Return exactly " +
		toolCallOpenTag + "{\"name\":\"FUNCTION_NAME\",\"arguments\":{...}}" + toolCallCloseTag +
		". Available functions: " + strings.Join(names, ", ") + ". Original user request:\n" + original
}

func claimsToolUnavailable(text string) bool {
	text = strings.ToLower(text)
	english := strings.Contains(text, "tool") && (strings.Contains(text, "unavailable") || strings.Contains(text, "cannot") ||
		strings.Contains(text, "can't") || strings.Contains(text, "failed") || strings.Contains(text, "error") || strings.Contains(text, "no access"))
	persian := strings.Contains(text, "ابزار") && (strings.Contains(text, "خطا") || strings.Contains(text, "دسترسی") ||
		strings.Contains(text, "نمی") || strings.Contains(text, "نمیتون") || strings.Contains(text, "موقت") || strings.Contains(text, "دسترس"))
	return english || persian
}

func (p toolPlan) extract(text string) ([]openAIToolCall, string) {
	if !p.enabled() {
		return nil, text
	}
	var calls []openAIToolCall
	var content strings.Builder
	remaining := text
	for {
		start, tag := nextToolTag(remaining)
		if start < 0 {
			content.WriteString(remaining)
			break
		}
		content.WriteString(remaining[:start])
		tail := remaining[start+len(tag.open):]
		payload, after, ok := readToolCallPayload(tail, tag.close)
		if !ok {
			content.WriteString(tag.open)
			remaining = tail
			continue
		}
		call, ok := p.decodeCall(payload, len(calls))
		if !ok {
			content.WriteString(tag.open)
			content.Write(payload)
			content.WriteString(tag.close)
		} else if !p.parallel && len(calls) > 0 {
			// The API contract requested one call. Keep the first valid call and
			// discard additional model markup instead of exposing it as user text.
		} else {
			calls = append(calls, call)
		}
		remaining = after
	}
	clean := strings.TrimSpace(content.String())
	// Some web-backed models correctly formulate a diagnostic shell command but
	// occasionally omit the adapter markup. Promote only an explicitly fenced,
	// read-only command to the bash tool. This preserves OpenCode's normal tool
	// loop without ever auto-running a mutating command.
	if len(calls) == 0 {
		if call, ok := p.readOnlyBashFallback(clean); ok {
			return []openAIToolCall{call}, ""
		}
	}
	return calls, clean
}

type toolTag struct {
	open  string
	close string
}

func nextToolTag(text string) (int, toolTag) {
	canonical := strings.Index(text, toolCallOpenTag)
	legacy, legacyTag := nextLegacyToolTag(text)
	if canonical >= 0 && (legacy < 0 || canonical < legacy) {
		return canonical, toolTag{open: toolCallOpenTag, close: toolCallCloseTag}
	}
	return legacy, legacyTag
}

func nextLegacyToolTag(text string) (int, toolTag) {
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], legacyToolOpenTag)
		if index < 0 {
			return -1, toolTag{}
		}
		index += offset
		rest := text[index+len(legacyToolOpenTag):]
		if strings.HasPrefix(rest, ">") {
			return index, toolTag{open: legacyToolOpenTag + ">", close: legacyToolCloseTag}
		}
		if len(rest) > 0 && strings.ContainsRune(" \t\r\n", rune(rest[0])) {
			if end := strings.IndexByte(rest, '>'); end >= 0 {
				return index, toolTag{open: legacyToolOpenTag + rest[:end+1], close: legacyToolCloseTag}
			}
		}
		offset = index + len(legacyToolOpenTag)
	}
	return -1, toolTag{}
}

func (p toolPlan) readOnlyBashFallback(text string) (openAIToolCall, bool) {
	if !p.allows("bash") {
		return openAIToolCall{}, false
	}
	start := strings.Index(text, "```")
	if start < 0 {
		return openAIToolCall{}, false
	}
	block := text[start+3:]
	if newline := strings.IndexByte(block, '\n'); newline >= 0 {
		block = block[newline+1:]
	} else {
		return openAIToolCall{}, false
	}
	if end := strings.Index(block, "```"); end >= 0 {
		block = block[:end]
	} else {
		return openAIToolCall{}, false
	}
	command := strings.TrimSpace(block)
	if !readOnlyShellCommand(command) {
		return openAIToolCall{}, false
	}
	arguments, _ := json.Marshal(map[string]string{"command": command})
	call := openAIToolCall{ID: fmt.Sprintf("call_lab_%d_0", time.Now().UnixNano()), Type: "function"}
	call.Function.Name = "bash"
	call.Function.Arguments = string(arguments)
	return call, true
}

func readOnlyShellCommand(command string) bool {
	if command == "" || strings.ContainsAny(command, "|&;<>\n\r`\\\"'$()[]*?!") {
		return false
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "pwd", "ls", "rg", "grep", "ps", "df", "free", "ss", "cat", "head", "stat", "du", "uname", "whoami":
		return true
	case "tail":
		return !hasShellFlag(fields, "-f", "--follow")
	case "sed":
		return !hasShellFlag(fields, "-i", "--in-place")
	case "find":
		return !hasShellFlag(fields, "-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fls")
	case "git":
		return hasReadOnlySubcommand(fields, "status", "diff", "log", "show", "branch", "rev-parse")
	case "docker":
		return hasReadOnlySubcommand(fields, "ps", "images", "inspect", "logs")
	case "systemctl":
		return hasReadOnlySubcommand(fields, "status", "is-active", "is-failed", "show", "list-units")
	default:
		return false
	}
}

func hasShellFlag(fields []string, forbidden ...string) bool {
	for _, field := range fields[1:] {
		for _, flag := range forbidden {
			if field == flag || strings.HasPrefix(field, flag+"=") {
				return true
			}
		}
	}
	return false
}

func hasReadOnlySubcommand(fields []string, allowed ...string) bool {
	if len(fields) < 2 {
		return false
	}
	for _, subcommand := range allowed {
		if fields[1] == subcommand {
			return true
		}
	}
	return false
}

func readToolCallPayload(value, closeTag string) ([]byte, string, bool) {
	decoder := json.NewDecoder(strings.NewReader(value))
	var payload json.RawMessage
	if decoder.Decode(&payload) != nil {
		return nil, value, false
	}
	after := strings.TrimLeft(value[decoder.InputOffset():], " \t\r\n")
	if !strings.HasPrefix(after, closeTag) {
		return nil, value, false
	}
	return payload, after[len(closeTag):], true
}

func (p toolPlan) decodeCall(payload []byte, index int) (openAIToolCall, bool) {
	var value struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(payload, &value) != nil || !p.allows(value.Name) || !jsonObject(value.Arguments) {
		return openAIToolCall{}, false
	}
	call := openAIToolCall{ID: fmt.Sprintf("call_lab_%d_%d", time.Now().UnixNano(), index), Type: "function"}
	call.Function.Name = value.Name
	call.Function.Arguments = string(value.Arguments)
	return call, true
}

func jsonObject(value json.RawMessage) bool {
	return len(value) > 0 && json.Valid(value) && bytes.HasPrefix(bytes.TrimSpace(value), []byte("{"))
}

func toolCallsPrompt(calls []openAIToolCall) string {
	blocks := make([]string, 0, len(calls))
	for _, call := range calls {
		arguments := json.RawMessage(call.Function.Arguments)
		if !jsonObject(arguments) {
			continue
		}
		value, _ := json.Marshal(map[string]any{"name": call.Function.Name, "arguments": json.RawMessage(arguments)})
		blocks = append(blocks, toolCallOpenTag+string(value)+toolCallCloseTag)
	}
	return strings.Join(blocks, "\n")
}

func streamToolCalls(calls []openAIToolCall) []any {
	values := make([]any, 0, len(calls))
	for index, call := range calls {
		values = append(values, map[string]any{
			"index":    index,
			"id":       call.ID,
			"type":     call.Type,
			"function": call.Function,
		})
	}
	return values
}

func toolResultName(messages []chatMessage, index int) string {
	id := messages[index].ToolCallID
	for previous := index - 1; previous >= 0; previous-- {
		for _, call := range messages[previous].ToolCalls {
			if call.ID == id {
				return call.Function.Name
			}
		}
	}
	return ""
}
