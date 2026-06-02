package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// inlineDiff produces a compact, line-level unified diff for a write/edit
// tool card. We don't shell out to the real `diff` binary because we
// often have to render before the file is on disk (the edit is a
// forward-looking description, not a pre/post snapshot). Instead we
// show whatever the model emitted as `content` (or `new_string`) so the
// user can review the proposed change inline.
//
// `path` is the target file (used for the header). `oldStr` and
// `newStr` are the pre/post snapshots; either may be empty. The result
// is plain text with the + / - prefixes colour-coded via the active
// theme.
func inlineDiff(path, oldStr, newStr string) string {
	var b strings.Builder
	header := diffHeaderStyle.Render(fmt.Sprintf("    %s", path))
	b.WriteString(header)
	b.WriteString("\n")

	oldLines := splitLines(oldStr)
	newLines := splitLines(newStr)

	// For very small inputs (write tool with no prior content) we show
	// the new content as all +. Otherwise we run a simple LCS-based
	// line diff. This is approximate — the goal is to give the user a
	// quick visual scan of "what's changing", not a byte-accurate diff.
	if oldStr == "" {
		for _, line := range newLines {
			b.WriteString(diffAddStyle.Render("    + " + line))
			b.WriteString("\n")
		}
		return b.String()
	}

	diffs := lcsDiff(oldLines, newLines)
	for _, d := range diffs {
		switch d.kind {
		case ' ':
			b.WriteString(diffCtxStyle.Render("    " + d.text))
		case '-':
			b.WriteString(diffDelStyle.Render("    - " + d.text))
		case '+':
			b.WriteString(diffAddStyle.Render("    + " + d.text))
		}
		b.WriteString("\n")
	}
	return b.String()
}

type diffLine struct {
	kind byte // ' ' (context), '-' (removed), '+' (added)
	text string
}

// splitLines normalises line endings and drops a single trailing empty
// line so an `s == ""` file doesn't render as a phantom blank line.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// lcsDiff is a simple line-level diff using classic LCS dynamic
// programming. Inputs are small (a few hundred lines tops for hand-
// edited files) so the O(n*m) matrix is fine. Output is an edit script
// of context / add / delete operations in document order.
func lcsDiff(a, b []string) []diffLine {
	n, m := len(a), len(b)
	// dp[i][j] = LCS length of a[i:] and b[j:].
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var out []diffLine
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, diffLine{' ', a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			out = append(out, diffLine{'-', a[i]})
			i++
		default:
			out = append(out, diffLine{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffLine{'-', a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffLine{'+', b[j]})
	}
	return out
}

// diffStat counts the +/-/context lines in a unified diff for the
// summary line shown in collapsed view ("+12 -3").
func diffStat(d []diffLine) (adds, dels int) {
	for _, x := range d {
		switch x.kind {
		case '+':
			adds++
		case '-':
			dels++
		}
	}
	return
}

// diffSummary is a one-liner summary for a collapsed tool card.
func diffSummary(adds, dels int) string {
	if adds == 0 && dels == 0 {
		return ""
	}
	return fmt.Sprintf(" (+%d -%d)", adds, dels)
}

// ensure the lipgloss import is used even if all diffs go through the
// colour-coded styles declared in styles.go.
var _ = lipgloss.NewStyle
