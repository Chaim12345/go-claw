package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mentionAutocomplete powers the `@file` mention syntax. When the user
// types `@` (or completes a partial path after `@`) the TUI offers a
// fuzzy-filtered list of files in the current working directory and
// its subdirectories. The selection is inserted as a literal token
// into the textarea (the model receives the verbatim path; there is no
// client-side substitution).
//
// The list is capped at depth 4 and skips heavy directories (.git,
// node_modules, dist) to keep startup snappy. We don't watch the
// filesystem for changes — pressing the @ trigger re-walks on demand.
type mentionAutocomplete struct {
	active     bool
	triggerCol int    // column where the @ was typed
	query      string // text after the @
	cursor     int
	matches    []string
}

func newMentionAutocomplete() *mentionAutocomplete { return &mentionAutocomplete{} }

// update scans the textarea value for an in-progress @mention. If one
// is found, the autocomplete becomes active. Returns true if the
// mention is currently active (caller should display the popup).
func (m *mentionAutocomplete) update(text string) bool {
	// Find the last `@` on the current line that isn't inside a token.
	at := strings.LastIndex(text, "@")
	if at == -1 {
		m.active = false
		return false
	}
	// Don't activate if the @ is in the middle of a word.
	if at > 0 {
		prev := text[at-1]
		if !isWhitespace(prev) {
			m.active = false
			return false
		}
	}
	// Capture the query (text from @ to end of last word).
	rest := text[at+1:]
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if isWhitespace(rest[i]) {
			end = i
			break
		}
	}
	q := rest[:end]
	if q != m.query {
		m.query = q
		m.matches = m.search(q)
		m.cursor = 0
	}
	m.active = true
	m.triggerCol = at
	return true
}

func isWhitespace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' }

// insert replaces the @trigger and partial query in text with the
// selected file path. Returns the new textarea value and the new
// cursor offset (just after the inserted path).
func (m *mentionAutocomplete) insert(text string) (string, int) {
	if !m.active || len(m.matches) == 0 {
		return text, len(text)
	}
	choice := m.matches[m.cursor]
	before := text[:m.triggerCol]
	after := text[m.triggerCol+1+len(m.query):]
	newVal := before + "@" + choice + " " + after
	cursor := m.triggerCol + 1 + len(choice) + 1
	m.active = false
	return newVal, cursor
}

func (m *mentionAutocomplete) moveCursor(delta int) {
	if len(m.matches) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
}

// search walks the current directory and returns up to 20 matches for
// query. The query is matched against the full path (so `@cmd/main`
// finds `cmd/main.go`).
func (m *mentionAutocomplete) search(query string) []string {
	if query == "" {
		// Empty query: show a small list of recent / common files.
		return m.recentFiles(20)
	}
	q := strings.ToLower(query)
	var out []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}
			if depthOf(path) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesQuery(strings.ToLower(path), q) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// recentFiles returns a handful of top-level files and directories.
// Used when the user types `@` with no follow-up characters.
func (m *mentionAutocomplete) recentFiles(limit int) []string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".claude" {
			continue
		}
		out = append(out, name)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func matchesQuery(path, q string) bool {
	if strings.Contains(path, q) {
		return true
	}
	// Allow fuzzy: every rune in q appears in path in order.
	pi := 0
	for i := 0; i < len(path) && pi < len(q); i++ {
		if path[i] == q[pi] {
			pi++
		}
	}
	return pi == len(q)
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "node_modules", "dist", "build", "target", ".next", "vendor", ".venv":
		return true
	}
	return false
}

func depthOf(path string) int {
	if path == "." || path == "" {
		return 0
	}
	return strings.Count(filepath.Clean(path), string(filepath.Separator))
}

// view renders the autocomplete popup. width/height bound the box.
func (m *mentionAutocomplete) view(width, height int) string {
	if !m.active || len(m.matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(paletteHeaderStyle.Render("  📎 Mention a file"))
	b.WriteString("\n")
	for i, match := range m.matches {
		marker := "  "
		style := paletteItemStyle
		if i == m.cursor {
			marker = "▶ "
			style = paletteItemSelectedStyle
		}
		b.WriteString(marker)
		b.WriteString(style.Render(truncate(match, width-8)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(paletteHintStyle.Render("  Tab/↑↓ select  Enter insert  Esc cancel"))
	box := paletteBoxStyle.Width(min(60, width-4)).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Bottom, box)
}
