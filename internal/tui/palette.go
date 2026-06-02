package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// paletteCommand describes a single command exposed in the Ctrl+P
// command palette. The Label is the primary visible string, Description
// is the muted hint, and Action is the slash command we synthesize
// (without the leading `/`) so the existing TUI slash-command plumbing
// runs end-to-end. Empty Action means the command is "system" — handled
// directly by the TUI (toggle theme, clear session, etc.).
type paletteCommand struct {
	label       string
	description string
	action      string // slash command without leading "/", e.g. "model"
	system      string // "toggle-mode" | "clear" | "theme-light" | ...
}

// palette is the opencode-style command palette: a fuzzy-filtered
// overlay that lets the user launch any slash command or system
// action without memorising key bindings.
type palette struct {
	active   bool
	query    string
	cursor   int
	commands []paletteCommand
	filtered []int // indices into commands
}

func newPalette() *palette { return &palette{} }

// allPaletteCommands returns the canonical list of palette commands.
// We rebuild it on demand so the list reflects the current provider /
// model state (the model picker shows different entries for Anthropic
// vs OpenAI). Most commands are system-level so we can route them
// without going through the slash-command pipeline.
func allPaletteCommands(provider string) []paletteCommand {
	cmds := []paletteCommand{
		{label: "Switch model", description: "Open the model picker", action: "model"},
		{label: "Change theme", description: "Cycle dark / light", system: "theme-toggle"},
		{label: "Clear session", description: "Wipe conversation history", system: "clear"},
		{label: "Save session", description: "Save current session to disk", action: "session save"},
		{label: "Load session", description: "Browse and load a saved session", system: "open-session-picker"},
		{label: "List sessions", description: "Browse saved sessions", action: "session list"},
		{label: "Show status", description: "Provider, model, mode", action: "status"},
		{label: "Show cost", description: "Token usage this session", action: "cost"},
		{label: "Show config", description: "Print all config values", action: "config"},
		{label: "Init project", description: "Create .claude/settings.json", action: "init"},
		{label: "Toggle permission mode", description: "Cycle default / accept-edits / bypass / plan", system: "toggle-mode"},
		{label: "Toggle todo panel", description: "Show/hide the todo list sidebar", system: "toggle-todo"},
		{label: "Show help", description: "Open the help panel", action: "help"},
		{label: "Exit", description: "Quit the TUI", action: "exit"},
	}
	// Provider-specific extras
	if provider == "openai" {
		cmds = append(cmds, paletteCommand{
			label: "Switch to gpt-4o", description: "Most capable OpenAI model", system: "set-model gpt-4o",
		}, paletteCommand{
			label: "Switch to gpt-4o-mini", description: "Fast and cheap", system: "set-model gpt-4o-mini",
		})
	} else {
		cmds = append(cmds, paletteCommand{
			label: "Switch to claude-opus-4-6", description: "Most capable — complex reasoning", system: "set-model claude-opus-4-6",
		}, paletteCommand{
			label: "Switch to claude-sonnet-4-6", description: "Balanced — speed and quality", system: "set-model claude-sonnet-4-6",
		})
	}
	return cmds
}

// open resets the palette to its initial state.
func (p *palette) open(provider string) {
	p.active = true
	p.query = ""
	p.cursor = 0
	p.commands = allPaletteCommands(provider)
	p.filtered = nil
	for i := range p.commands {
		p.filtered = append(p.filtered, i)
	}
}

// close hides the palette and clears its state.
func (p *palette) close() {
	p.active = false
	p.query = ""
	p.cursor = 0
	p.filtered = nil
}

// updateKey consumes a key press while the palette is active. Returns
// the chosen command (or nil) and a flag indicating whether the palette
// should close.
func (p *palette) updateKey(msg tea.KeyMsg) (chosen *paletteCommand, close bool) {
	switch msg.Type {
	case tea.KeyEsc:
		return nil, true
	case tea.KeyEnter:
		if len(p.filtered) == 0 {
			return nil, true
		}
		cmd := p.commands[p.filtered[p.cursor]]
		return &cmd, true
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false
	case tea.KeyDown:
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return nil, false
	case tea.KeyBackspace:
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			p.refilter()
			if p.cursor >= len(p.filtered) {
				p.cursor = max(0, len(p.filtered)-1)
			}
		}
		return nil, false
	}

	// Append printable characters to the query.
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		p.query += msg.String()
		p.refilter()
		p.cursor = 0
	}
	return nil, false
}

// refilter rebuilds the filtered index list against the current query
// using case-insensitive substring matching. Multi-word queries are
// split on whitespace and every token must hit (in any order) so users
// can type "session save" to find "Save session".
func (p *palette) refilter() {
	p.filtered = p.filtered[:0]
	q := strings.ToLower(strings.TrimSpace(p.query))
	if q == "" {
		for i := range p.commands {
			p.filtered = append(p.filtered, i)
		}
		return
	}
	tokens := strings.Fields(q)
	type scored struct {
		idx   int
		score int
	}
	var hits []scored
	for i, c := range p.commands {
		hay := strings.ToLower(c.label + " " + c.description)
		score, ok := scoreMatch(hay, tokens)
		if ok {
			hits = append(hits, scored{i, score})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	for _, h := range hits {
		p.filtered = append(p.filtered, h.idx)
	}
}

// scoreMatch returns a simple score and match-ok flag. The score is
// higher when the query tokens appear as prefixes of words in the hay
// stack (so "sa ses" scores higher than "ses sa" for "Save session").
func scoreMatch(hay string, tokens []string) (int, bool) {
	score := 0
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return 0, false
		}
		// Bonus for prefix match.
		if strings.HasPrefix(hay, t) {
			score += 5
		} else {
			score += 1
		}
	}
	return score, true
}

// view renders the palette overlay.
func (p *palette) view(width, height int) string {
	if !p.active {
		return ""
	}

	var b strings.Builder
	b.WriteString(paletteHeaderStyle.Render("  ⌘ Command Palette"))
	b.WriteString("\n")
	b.WriteString(paletteQueryStyle.Render("  > " + p.query + "│"))
	b.WriteString("\n\n")

	if len(p.filtered) == 0 {
		b.WriteString(paletteHintStyle.Render("  no matches"))
	} else {
		max := 10
		if max > len(p.filtered) {
			max = len(p.filtered)
		}
		for i := 0; i < max; i++ {
			cmd := p.commands[p.filtered[i]]
			marker := "  "
			style := paletteItemStyle
			if i == p.cursor {
				marker = "▶ "
				style = paletteItemSelectedStyle
			}
			b.WriteString(marker)
			b.WriteString(style.Render(cmd.label))
			if cmd.description != "" {
				b.WriteString("  ")
				b.WriteString(paletteHintStyle.Render(cmd.description))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(paletteHintStyle.Render("  ↑↓ navigate  Enter run  Esc close"))

	box := paletteBoxStyle.Width(min(72, width-4)).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
