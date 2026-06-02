package tui

import (
	"sort"
	"strings"
)

// todoPanel is a sidebar-style view of the current todo list. It's
// updated automatically whenever the agent runs the todo_write tool —
// the TUI reads the persisted `.claude/todos.json` so it doesn't need
// a separate event channel.
//
// The panel is rendered to the right of the conversation viewport on
// wide terminals and hidden on narrow ones (width < 100). Users can
// also pin it with `/todo` or dismiss it with the same command.
type todoPanel struct {
	visible bool
	items   []todoItem
	width   int
}

type todoItem struct {
	id       string
	content  string
	status   string // "pending" | "in_progress" | "done"
	priority string // "high" | "medium" | "low"
}

func newTodoPanel() *todoPanel {
	return &todoPanel{visible: false, width: 36}
}

// setItems replaces the current list. Sorted so in_progress items come
// first, then pending, then done.
func (t *todoPanel) setItems(items []todoItem) {
	t.items = items
	sort.SliceStable(t.items, func(i, j int) bool {
		return todoOrder(t.items[i].status) < todoOrder(t.items[j].status)
	})
}

func todoOrder(status string) int {
	switch status {
	case "in_progress":
		return 0
	case "pending":
		return 1
	case "done":
		return 2
	default:
		return 3
	}
}

func (t *todoPanel) toggle() { t.visible = !t.visible }

// isVisible reports whether the panel is shown. The model also gates
// rendering on terminal width to keep small terminals readable.
func (t *todoPanel) isVisible() bool { return t.visible && len(t.items) > 0 }

// view renders the todo sidebar. height is the maximum number of lines
// to render — the caller passes the viewport height so we don't draw
// past the conversation scrollback.
func (t *todoPanel) view(height int) string {
	if !t.isVisible() {
		return ""
	}

	var b strings.Builder
	b.WriteString(todoHeaderStyle.Render("  ☑ Tasks"))
	b.WriteString("\n\n")

	// Group items by status for readability.
	var inProgress, pending, done []todoItem
	for _, it := range t.items {
		switch it.status {
		case "in_progress":
			inProgress = append(inProgress, it)
		case "pending":
			pending = append(pending, it)
		case "done":
			done = append(done, it)
		}
	}

	renderGroup := func(label string, items []todoItem, box, mark string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(todoSectionStyle.Render("  " + label))
		b.WriteString("\n")
		for _, it := range items {
			priority := ""
			if it.priority == "high" {
				priority = todoHighStyle.Render(" (!)")
			}
			row := "  " + box + " " + truncate(it.content, t.width-10) + priority
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	renderGroup("In Progress", inProgress, "◉", "◉")
	renderGroup("Pending", pending, "○", "○")
	renderGroup("Done", done, "✓", "✓")

	return b.String()
}
