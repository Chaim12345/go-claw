package deepseek

import (
	"encoding/json"
	"strings"
)

// knownTools is the set of tool names the model can call. The conversation
// loop wires these up to the built-in claw-code-go tools (read, write, edit,
// bash, grep, glob, ls).
var knownTools = map[string]bool{
	"bash":      true,
	"read":      true,
	"read_file": true,
	"write":     true,
	"write_file": true,
	"edit":      true,
	"file_edit": true,
	"grep":      true,
	"glob":      true,
	"ls":        true,
}

// ToolCall is a parsed tool invocation from the model output.
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ExtractToolCalls scans model output and extracts tool calls in the
// canonical XML format:
//
//	<tool_calls>
//	  <tool name="bash">
//	    <arg name="command">ls -la</arg>
//	  </tool>
//	</tool_calls>
//
// Falls back to JSON if no XML tool calls are found (for backward compat).
func ExtractToolCalls(text string) []ToolCall {
	if len(text) == 0 {
		return nil
	}
	if calls := extractXmlToolCalls(text); len(calls) > 0 {
		return calls
	}
	// Fallback: JSON format (backward compat)
	if calls := extractJsonToolCalls(text); len(calls) > 0 {
		return calls
	}
	return nil
}

// ── XML parser ──────────────────────────────────────────────────

func extractXmlToolCalls(text string) []ToolCall {
	si := strings.Index(text, "<tool_calls>")
	if si == -1 {
		return nil
	}
	ei := strings.Index(text[si:], "</tool_calls>")
	if ei == -1 {
		return nil
	}
	ei += si
	inner := text[si+len("<tool_calls>") : ei]
	return parseToolCallXml(inner)
}

// parseToolCallXml parses the inner content of a <tool_calls> block,
// extracting <tool name="..."><arg name="...">value</arg></tool> elements.
func parseToolCallXml(inner string) []ToolCall {
	var calls []ToolCall
	pos := 0
	for {
		toolStart := strings.Index(inner[pos:], "<tool ")
		if toolStart == -1 {
			break
		}
		toolStart += pos

		// Find the closing </tool> for this tool block
		toolEnd := strings.Index(inner[toolStart:], "</tool>")
		if toolEnd == -1 {
			break
		}
		toolEnd += toolStart
		toolBlock := inner[toolStart:toolEnd]

		// Extract tool name from the opening tag: <tool name="bash">
		name := extractXmlAttr(toolBlock, "name")
		if name == "" || !knownTools[name] {
			pos = toolEnd + len("</tool>")
			continue
		}

		// Parse all <arg name="...">value</arg> children
		args := make(map[string]interface{})
		argPos := 0
		for {
			argStart := strings.Index(toolBlock[argPos:], "<arg ")
			if argStart == -1 {
				break
			}
			argStart += argPos

			argEnd := strings.Index(toolBlock[argStart:], "</arg>")
			if argEnd == -1 {
				break
			}
			argEnd += argStart
			argBlock := toolBlock[argStart:argEnd]

			argName := extractXmlAttr(argBlock, "name")
			if argName == "" {
				argPos = argEnd + len("</arg>")
				continue
			}

			// Extract text content between > and </arg>
			contentStart := strings.Index(argBlock, ">")
			if contentStart == -1 {
				argPos = argEnd + len("</arg>")
				continue
			}
			content := strings.TrimSpace(argBlock[contentStart+1:])
			// Auto-coerce: try JSON parse for numbers, booleans, objects
			var val interface{}
			if json.Unmarshal([]byte(content), &val) == nil {
				args[argName] = val
			} else {
				args[argName] = content
			}
			argPos = argEnd + len("</arg>")
		}

		calls = append(calls, ToolCall{Name: name, Arguments: args})
		pos = toolEnd + len("</tool>")
	}
	return calls
}

// extractXmlAttr extracts a quoted attribute value from an XML tag string.
// Returns "" if not found.
func extractXmlAttr(tag, attr string) string {
	// Look for attr="value" or attr='value'
	search := attr + "=\""
	idx := strings.Index(tag, search)
	if idx == -1 {
		search = attr + "='"
		idx = strings.Index(tag, search)
		if idx == -1 {
			return ""
		}
	}
	idx += len(search)
	end := strings.Index(tag[idx:], "\"")
	if end == -1 {
		end = strings.Index(tag[idx:], "'")
		if end == -1 {
			return tag[idx:]
		}
	}
	return tag[idx : idx+end]
}

// ── JSON fallback (backward compat) ─────────────────────────────

func extractJsonToolCalls(text string) []ToolCall {
	startIdx := -1
	for _, p := range []string{`{"tool_calls"`, `{"_calls"`} {
		if idx := strings.Index(text, p); idx != -1 {
			startIdx = idx
			break
		}
	}
	if startIdx == -1 {
		return nil
	}
	jsonStr := extractJsonBlock(text, startIdx)
	if jsonStr == "" {
		return nil
	}
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(jsonStr), &parsed) != nil {
		return nil
	}
	var arr []interface{}
	if tc, ok := parsed["tool_calls"].([]interface{}); ok {
		arr = tc
	}
	if arr == nil {
		if tc, ok := parsed["_calls"].([]interface{}); ok {
			arr = tc
		}
	}
	if len(arr) == 0 {
		return nil
	}
	var calls []ToolCall
	for _, c := range arr {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		name := ""
		if n, ok := m["name"].(string); ok {
			name = n
		}
		if name == "" {
			if fn, ok := m["function"].(map[string]interface{}); ok {
				if n, ok := fn["name"].(string); ok {
					name = n
				}
			}
		}
		if !knownTools[name] {
			continue
		}
		calls = append(calls, ToolCall{Name: name, Arguments: normalizeToolArgs(m)})
	}
	return calls
}

func extractJsonBlock(text string, startIdx int) string {
	depth, inStr := 0, false
	for i := startIdx; i < len(text); i++ {
		c := text[i]
		if inStr {
			if c == 92 { // backslash
				i++
				continue
			}
			if c == 34 { // "
				inStr = false
			}
			continue
		}
		if c == 34 {
			inStr = true
			continue
		}
		if c == 123 { // {
			depth++
		}
		if c == 125 { // }
			depth--
			if depth == 0 {
				return text[startIdx : i+1]
			}
		}
	}
	return ""
}

func normalizeToolArgs(m map[string]interface{}) map[string]interface{} {
	if args, ok := m["arguments"]; ok {
		switch a := args.(type) {
		case map[string]interface{}:
			return a
		case string:
			var p map[string]interface{}
			if json.Unmarshal([]byte(a), &p) == nil {
				return p
			}
		}
	}
	if fn, ok := m["function"].(map[string]interface{}); ok {
		if args, ok := fn["arguments"]; ok {
			switch a := args.(type) {
			case map[string]interface{}:
				return a
			case string:
				var p map[string]interface{}
				if json.Unmarshal([]byte(a), &p) == nil {
					return p
				}
			}
		}
	}
	return map[string]interface{}{}
}

// StripToolCalls removes tool-call XML blocks from the assistant text
// so the cleaned message can be displayed to the user.
func StripToolCalls(text string) string {
	result := text
	// Strip <tool_calls>...</tool_calls> blocks
	for {
		si := strings.Index(result, "<tool_calls>")
		if si == -1 {
			break
		}
		ei := strings.Index(result[si:], "</tool_calls>")
		if ei == -1 {
			result = result[:si]
			break
		}
		ei += si
		result = result[:si] + result[ei+len("</tool_calls>"):]
	}
	// Strip JSON fallback tool calls
	for _, key := range []string{`{"tool_calls"`, `{"_calls"`} {
		for {
			idx := strings.Index(result, key)
			if idx == -1 {
				break
			}
			jsonStr := extractJsonBlock(result, idx)
			if jsonStr == "" {
				break
			}
			result = result[:idx] + result[idx+len(jsonStr):]
		}
	}
	return strings.TrimSpace(result)
}
