package deepseek

import (
	"encoding/json"
	"testing"
)

func TestExtractXmlToolCalls(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "single tool",
			text: `<tool_calls><tool name="bash"><arg name="command">ls -la</arg></tool></tool_calls>`,
			want: 1,
		},
		{
			name: "multi tool",
			text: `<tool_calls><tool name="bash"><arg name="command">ls</arg></tool><tool name="read"><arg name="path">x.txt</arg></tool></tool_calls>`,
			want: 2,
		},
		{
			name: "multi-line whitespace",
			text: "<tool_calls>\n  <tool name=\"write\">\n    <arg name=\"path\">out.txt</arg>\n    <arg name=\"content\">hello</arg>\n  </tool>\n</tool_calls>",
			want: 1,
		},
		{
			name: "no tool calls returns nil",
			text: "Hello world",
			want: 0,
		},
		{
			name: "unknown tool skipped",
			text: `<tool_calls><tool name="unknown"><arg name="foo">bar</arg></tool></tool_calls>`,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractToolCalls(tt.text)
			if len(got) != tt.want {
				t.Errorf("ExtractToolCalls() got %d calls, want %d", len(got), tt.want)
			}
		})
	}
}

func TestExtractXmlToolArgs(t *testing.T) {
	text := `<tool_calls><tool name="bash"><arg name="command">pwd</arg></tool></tool_calls>`
	calls := ExtractToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "bash" {
		t.Errorf("expected name bash, got %s", calls[0].Name)
	}
	if calls[0].Arguments["command"] != "pwd" {
		t.Errorf("expected command=pwd, got %v", calls[0].Arguments["command"])
	}
}

func TestExtractXmlCoercesTypes(t *testing.T) {
	text := `<tool_calls><tool name="bash"><arg name="timeout">30</arg></tool></tool_calls>`
	calls := ExtractToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	// "30" should be coerced to float64(30) via JSON.Unmarshal
	v, ok := calls[0].Arguments["timeout"].(float64)
	if !ok {
		t.Fatalf("expected timeout to be float64, got %T", calls[0].Arguments["timeout"])
	}
	if v != 30 {
		t.Errorf("expected 30, got %v", v)
	}
}

func TestStripToolCalls(t *testing.T) {
	text := "some text <tool_calls><tool name=\"bash\"><arg name=\"command\">ls</arg></tool></tool_calls> end"
	stripped := StripToolCalls(text)
	if stripped != "some text  end" {
		t.Errorf("unexpected stripped text: %q", stripped)
	}
}

func TestJSONFallback(t *testing.T) {
	text := `{"tool_calls":[{"name":"bash","arguments":{"command":"ls -la"}}]}`
	calls := ExtractToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "bash" {
		t.Errorf("expected name bash, got %s", calls[0].Name)
	}
}

func TestStripJSON(t *testing.T) {
	text := `here {"tool_calls":[{"name":"bash","arguments":{"command":"ls"}}]} there`
	stripped := StripToolCalls(text)
	if stripped != "here  there" {
		t.Errorf("unexpected stripped: %q", stripped)
	}
}

func TestExtractXmlAttr(t *testing.T) {
	tests := []struct {
		tag  string
		attr string
		want string
	}{
		{`<tool name="bash">`, "name", "bash"},
		{`<arg name="command">`, "name", "command"},
		{`<tool name="read_file">`, "name", "read_file"},
		{`<tool name='single'>`, "name", "single"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := extractXmlAttr(tt.tag, tt.attr)
			if got != tt.want {
				t.Errorf("extractXmlAttr(%q, %q) = %q, want %q", tt.tag, tt.attr, got, tt.want)
			}
		})
	}
}

func TestIntegrationRoundTrip(t *testing.T) {
	// Verify the XML parser produces JSON-serializable results that match
	// the JSON fallback format.
	xml := `<tool_calls><tool name="bash"><arg name="command">find . -name "*.go"</arg></tool></tool_calls>`
	calls := ExtractToolCalls(xml)
	data, err := json.Marshal(calls)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var parsed []ToolCall
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Name != "bash" {
		t.Errorf("round-trip failed: %+v", parsed)
	}
}
