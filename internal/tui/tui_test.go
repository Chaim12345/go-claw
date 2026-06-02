package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPaletteOpenAndClose(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	if !p.active {
		t.Fatal("palette should be active after open()")
	}
	if len(p.filtered) == 0 {
		t.Fatal("palette should have commands after open()")
	}
	p.close()
	if p.active {
		t.Fatal("palette should be inactive after close()")
	}
}

func TestPaletteRefilter(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	p.query = "model"
	p.refilter()
	if len(p.filtered) == 0 {
		t.Fatal("expected at least one match for 'model'")
	}
	// 'model' should match at least "Switch model" and the provider-
	// specific "Switch to claude-...-4-6" entries. All should have
	// 'model' in their label or description.
	for _, idx := range p.filtered {
		hay := strings.ToLower(p.commands[idx].label + " " + p.commands[idx].description)
		if !strings.Contains(hay, "model") {
			t.Errorf("filter matched unexpected label: %q", p.commands[idx].label)
		}
	}
}

func TestPaletteNoMatch(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	p.query = "xyzzy-nope-nothing-matches"
	p.refilter()
	if len(p.filtered) != 0 {
		t.Errorf("expected no matches, got %d", len(p.filtered))
	}
}

func TestPaletteKeyEnterSelects(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	chosen, close := p.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !close {
		t.Fatal("Enter should close the palette")
	}
	if chosen == nil {
		t.Fatal("Enter should select a command")
	}
}

func TestPaletteKeyEscCloses(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	_, close := p.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !close {
		t.Fatal("Esc should close the palette")
	}
	// The caller is responsible for calling p.close() once the close
	// signal is observed. We simulate that here.
	if close {
		p.close()
	}
	if p.active {
		t.Fatal("palette should be inactive after Esc + close")
	}
}

func TestPaletteBackspace(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	p.query = "abc"
	_, _ = p.updateKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.query != "ab" {
		t.Errorf("expected query 'ab' after backspace, got %q", p.query)
	}
}

func TestPaletteMultiToken(t *testing.T) {
	p := newPalette()
	p.open("anthropic")
	p.query = "show config"
	p.refilter()
	if len(p.filtered) == 0 {
		t.Fatal("'show config' should match something")
	}
	// Top hit should be "Show config" (highest score — both tokens
	// match the label).
	top := p.commands[p.filtered[0]].label
	if !strings.Contains(strings.ToLower(top), "config") {
		t.Errorf("top hit should contain 'config', got %q", top)
	}
}

func TestTodoPanelSetItemsSorted(t *testing.T) {
	tp := newTodoPanel()
	tp.setItems([]todoItem{
		{id: "a", content: "do A", status: "done", priority: "low"},
		{id: "b", content: "do B", status: "in_progress", priority: "high"},
		{id: "c", content: "do C", status: "pending", priority: "medium"},
	})
	if tp.items[0].id != "b" {
		t.Errorf("in_progress should be first, got %q", tp.items[0].id)
	}
	if tp.items[1].id != "c" {
		t.Errorf("pending should be second, got %q", tp.items[1].id)
	}
	if tp.items[2].id != "a" {
		t.Errorf("done should be last, got %q", tp.items[2].id)
	}
}

func TestTodoPanelToggleVisibility(t *testing.T) {
	tp := newTodoPanel()
	if tp.visible {
		t.Fatal("new panel should be hidden")
	}
	tp.toggle()
	if !tp.visible {
		t.Fatal("panel should be visible after toggle")
	}
	tp.toggle()
	if tp.visible {
		t.Fatal("panel should be hidden after second toggle")
	}
}

func TestTodoPanelIsVisible(t *testing.T) {
	tp := newTodoPanel()
	tp.toggle()
	if tp.isVisible() {
		t.Fatal("empty panel shouldn't be visible")
	}
	tp.setItems([]todoItem{{id: "a", content: "x", status: "pending"}})
	if !tp.isVisible() {
		t.Fatal("panel with items + visible flag should be visible")
	}
}

func TestMentionUpdate(t *testing.T) {
	m := newMentionAutocomplete()
	if m.update("hello world") {
		t.Fatal("no @ should not activate")
	}
	if !m.update("hello @") {
		t.Fatal("trailing @ should activate")
	}
	if !m.update("check @read") {
		t.Fatal("@partial should activate")
	}
	if m.update("foo@bar") {
		t.Fatal("@ in middle of word should not activate")
	}
}

func TestMentionInsert(t *testing.T) {
	m := newMentionAutocomplete()
	_ = m.update("see @mai")
	m.matches = []string{"main.go", "main_test.go"}
	m.cursor = 0
	out, cursor := m.insert("see @mai")
	if !strings.Contains(out, "@main.go") {
		t.Errorf("expected inserted path in output, got %q", out)
	}
	if cursor <= 0 {
		t.Errorf("expected positive cursor, got %d", cursor)
	}
}

func TestExtractJSONString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", `"path": "foo.txt"`, "foo.txt"},
		{"escaped", `"path": "a\"b"`, `a"b`},
		{"no colon", `"path"`, ""},
		{"not string", `"path": 42`, ""},
		{"empty", `"path": ""`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONString(tt.in)
			if got != tt.want {
				t.Errorf("extractJSONString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLcsDiff(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"a", "x", "c", "y"}
	d := lcsDiff(a, b)
	if len(d) == 0 {
		t.Fatal("expected non-empty diff")
	}
	// We expect: 'a' (context), 'b' (del), 'x' (add), 'c' (context), 'd' (del), 'y' (add)
	adds, dels := 0, 0
	for _, x := range d {
		switch x.kind {
		case '+':
			adds++
		case '-':
			dels++
		}
	}
	if adds != 2 {
		t.Errorf("expected 2 adds, got %d", adds)
	}
	if dels != 2 {
		t.Errorf("expected 2 dels, got %d", dels)
	}
}

func TestInlineDiffWriteAll(t *testing.T) {
	// A write with no prior content should render all lines as adds.
	out := inlineDiff("foo.txt", "", "line1\nline2")
	if !strings.Contains(out, "foo.txt") {
		t.Errorf("expected filename in output, got %q", out)
	}
	if !strings.Contains(out, "+") {
		t.Errorf("expected + markers in write output, got %q", out)
	}
}

func TestFormatToolCardCollapsed(t *testing.T) {
	c := toolCard{id: "1", name: "bash", input: "ls", result: "file1\nfile2"}
	out := formatToolCard(c)
	if !strings.Contains(out, "bash") {
		t.Errorf("expected tool name in collapsed view")
	}
	if !strings.Contains(out, "ls") {
		t.Errorf("expected input in collapsed view")
	}
}

func TestFormatToolCardExpanded(t *testing.T) {
	c := toolCard{id: "1", name: "bash", input: "ls -la", result: "a\nb", expanded: true}
	out := formatToolCard(c)
	if !strings.Contains(out, "input:") {
		t.Errorf("expected 'input:' label in expanded view")
	}
	if !strings.Contains(out, "result:") {
		t.Errorf("expected 'result:' label in expanded view")
	}
}

func TestIndentBlock(t *testing.T) {
	out := indentBlock("a\nb", "  ")
	if !strings.HasPrefix(out, "  a") {
		t.Errorf("expected lines to be indented, got %q", out)
	}
}

func TestToolLineDetection(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"  ◆ bash: ls", true},
		{"  ✓ bash", true},
		{"  [Tool:bash result]", true},
		{"  [Assistant tool call: bash]", true},
		{"Hello world", false},
		{"Some markdown text", false},
	}
	for _, c := range cases {
		got := isToolLine(c.line)
		if got != c.want {
			t.Errorf("isToolLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestScoreMatch(t *testing.T) {
	score, ok := scoreMatch("save session", []string{"sa", "ses"})
	if !ok {
		t.Fatal("expected match")
	}
	if score == 0 {
		t.Fatal("expected non-zero score")
	}
	// 'sa' is a prefix => score 5, 'ses' is not => 1, total 6
	if score != 6 {
		t.Errorf("expected score 6, got %d", score)
	}
}

func TestScoreMatchMiss(t *testing.T) {
	_, ok := scoreMatch("foo bar", []string{"nope"})
	if ok {
		t.Fatal("expected no match")
	}
}

func TestDepthOf(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{".", 0},
		{"foo", 0},
		{"foo/bar", 1},
		{"foo/bar/baz", 2},
	}
	for _, tt := range tests {
		if got := depthOf(tt.path); got != tt.want {
			t.Errorf("depthOf(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".git", true},
		{"node_modules", true},
		{"src", false},
		{"foo", false},
	}
	for _, tt := range tests {
		if got := shouldSkipDir(tt.path); got != tt.want {
			t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
