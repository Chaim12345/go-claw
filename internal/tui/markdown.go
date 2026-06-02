package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// markdownRenderer is a pooled glamour renderer used to format assistant
// text and tool results. Built lazily on first use with a style that
// matches the active TUI theme — we default to "dark" and let users
// override via the /theme command (we also rebuild the renderer on
// theme change).
var (
	mdRenderer      *glamour.TermRenderer
	mdRendererStyle = "dark"
)

// renderMarkdown formats s as terminal markdown. If the glamour renderer
// isn't initialised (or is built for a different style), it rebuilds
// itself to match. Empty input returns an empty string.
func renderMarkdown(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if mdRenderer == nil || mdRendererStyle != currentTheme.Name {
		if err := rebuildMarkdownRenderer(); err != nil {
			// Fall back to plain text on any renderer error — the assistant
			// output still flows through to the user.
			return s
		}
	}
	out, err := mdRenderer.Render(s)
	if err != nil {
		return s
	}
	return strings.TrimRight(out, "\n")
}

func rebuildMarkdownRenderer() error {
	style := "dark"
	if currentTheme.Name == "light" {
		style = "light"
	}
	mdRendererStyle = style
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(max(40, 100)),
		glamour.WithEmoji(),
	)
	if err != nil {
		return err
	}
	mdRenderer = r
	return nil
}

// renderBufferSegments splits a streamed buffer into markdown text and
// tool-card lines, renders the markdown portions through glamour, and
// returns the recombined string. Lines starting with "  ◆" or "  ✓"
// (tool cards) and "  [Tool:" markers are passed through untouched so
// styling is preserved.
func renderBufferSegments(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var out strings.Builder
	var mdBuf strings.Builder
	flushMD := func() {
		if mdBuf.Len() == 0 {
			return
		}
		out.WriteString(renderMarkdown(mdBuf.String()))
		out.WriteString("\n")
		mdBuf.Reset()
	}
	for _, line := range lines {
		if isToolLine(line) {
			flushMD()
			out.WriteString(line)
			out.WriteString("\n")
		} else {
			mdBuf.WriteString(line)
			mdBuf.WriteString("\n")
		}
	}
	flushMD()
	return strings.TrimRight(out.String(), "\n")
}

func isToolLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "◆") || strings.HasPrefix(t, "✓") ||
		strings.HasPrefix(t, "[Tool:") || strings.HasPrefix(t, "[Assistant tool call:") ||
		strings.HasPrefix(s, "  ◆") || strings.HasPrefix(s, "  ✓")
}
