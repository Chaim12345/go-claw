package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// sessionPicker is the opencode-style session browser. It lists every
// saved session on disk and lets the user load one with Enter. Esc
// closes without changes. We keep the picker simple — no fuzzy
// filtering for now because the list is bounded by what the user has
// actually run.
type sessionPicker struct {
	active  bool
	cursor  int
	entries []pickerSession
}

type pickerSession struct {
	id            string
	updated       string
	messageCount  int
	totalInTokens int
	totalOutTokens int
}

func newSessionPicker() *sessionPicker { return &sessionPicker{} }

// open rebuilds the entry list from the loop's session metadata. Called
// every time the user invokes the picker so newly-saved sessions show
// up immediately.
func (sp *sessionPicker) open(metas []runtimeSessionMeta) {
	sp.active = true
	sp.cursor = 0
	sp.entries = sp.entries[:0]
	for _, m := range metas {
		sp.entries = append(sp.entries, pickerSession{
			id:             m.id,
			updated:        m.updated,
			messageCount:   m.messageCount,
			totalInTokens:  m.totalInTokens,
			totalOutTokens: m.totalOutTokens,
		})
	}
}

func (sp *sessionPicker) close() {
	sp.active = false
	sp.cursor = 0
	sp.entries = nil
}

func (sp *sessionPicker) updateKey(msg tea.KeyMsg) (chosen string, close bool) {
	switch msg.Type {
	case tea.KeyEsc:
		return "", true
	case tea.KeyEnter:
		if len(sp.entries) == 0 {
			return "", true
		}
		return sp.entries[sp.cursor].id, true
	case tea.KeyUp:
		if sp.cursor > 0 {
			sp.cursor--
		}
	case tea.KeyDown:
		if sp.cursor < len(sp.entries)-1 {
			sp.cursor++
		}
	}
	return "", false
}

// runtimeSessionMeta is a minimal projection of runtime.SessionMeta so
// this file doesn't need to import runtime (which would create a
// circular dependency through the tui package).
type runtimeSessionMeta struct {
	id             string
	updated        string
	messageCount   int
	totalInTokens  int
	totalOutTokens int
}

func (sp *sessionPicker) view(width, height int) string {
	if !sp.active {
		return ""
	}

	var b strings.Builder
	b.WriteString(pickerHeaderStyle.Render("  ⌫ Load Session"))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render("  ↑↓ navigate  Enter load  Esc cancel"))
	b.WriteString("\n\n")

	if len(sp.entries) == 0 {
		b.WriteString(paletteHintStyle.Render("  no saved sessions"))
	} else {
		// Header row
		b.WriteString(paletteHintStyle.Render(fmt.Sprintf("    %-30s  %-19s  %6s  %10s  %10s",
			"ID", "Updated", "Msgs", "In tok", "Out tok")))
		b.WriteString("\n")
		b.WriteString(paletteHintStyle.Render("    " + strings.Repeat("─", 84)))
		b.WriteString("\n")
		for i, s := range sp.entries {
			marker := "  "
			style := unselectedModelStyle
			if i == sp.cursor {
				marker = "▶ "
				style = selectedModelStyle
			}
			id := s.id
			if len(id) > 28 {
				id = id[:25] + "..."
			}
			b.WriteString(marker)
			b.WriteString(style.Render(fmt.Sprintf("%-30s  %-19s  %6d  %10s  %10s",
				id, s.updated, s.messageCount, formatNum(s.totalInTokens), formatNum(s.totalOutTokens))))
			b.WriteString("\n")
		}
	}

	box := helpBoxStyle.Width(min(96, width-4)).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
