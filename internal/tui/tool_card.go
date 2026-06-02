package tui

import (
	"fmt"
	"strings"
)

// toolCard represents a single tool invocation that has been streamed
// into the conversation viewport. It tracks a stable ID so the TUI can
// toggle expansion per-card with the same key the user pressed in
// opencode (`Ctrl+T` to toggle the most recent, or `Ctrl+O` to expand
// all). For now the model keeps a slice of these and lets the user
// toggle the last one with a single key.
type toolCard struct {
	id         string
	name       string
	input      string
	result     string
	expanded   bool
	hasDiff    bool
	diffInline string
}

// formatToolCard renders a tool card as a multi-line block. Collapsed
// view is one line: "  ◆ <name>  <input summary>". Expanded view shows
// the full input and result, with optional inline diff for file edits.
func formatToolCard(c toolCard) string {
	var b strings.Builder
	if c.expanded {
		b.WriteString(toolExpandedHeaderStyle.Render(fmt.Sprintf("  ◆ %s", c.name)))
		b.WriteString("\n")
		if c.hasDiff && c.diffInline != "" {
			b.WriteString(c.diffInline)
		} else {
			b.WriteString(toolInputStyle.Render("    input:\n"))
			b.WriteString(indentBlock(c.input, "    "))
		}
		if c.result != "" {
			b.WriteString(toolInputStyle.Render("\n    result:\n"))
			// Cap the result preview at 4KB so a huge dump doesn't blow up
			// the viewport.
			preview := c.result
			if len(preview) > 4096 {
				preview = preview[:4096] + "\n    …(truncated)"
			}
			b.WriteString(indentBlock(preview, "    "))
		}
	} else {
		suffix := ""
		if c.result != "" {
			suffix = " → " + truncate(c.result, 40)
		}
		b.WriteString(toolDoneStyle.Render(fmt.Sprintf("  ◆ %s: %s%s", c.name, truncate(c.input, 60), suffix)))
	}
	return b.String()
}

// indentBlock prefixes every line of s with prefix.
func indentBlock(s, prefix string) string {
	if s == "" {
		return prefix + "(empty)\n"
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}
