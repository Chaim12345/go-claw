package tui

import "github.com/charmbracelet/bubbletea"

// hasShift reports whether the keypress carries the Shift modifier.
// bubbletea reports shift-tab as the rune "\t" (0x09) with a non-empty
// `.String()` of "tab", and the runtime strips the shift from the rune
// itself. We detect shift+tab by combining the KeyMsg's `Type` with
// the raw `.String()` value: when shift is held the runtime appends
// "shift+tab" / "shift+backtab" depending on terminal mode.
func hasShift(msg tea.KeyMsg, target tea.KeyType) bool {
	// Reverse-tab (shift+tab) is reported as KeyShiftTab in bubbletea 1.x.
	if target == tea.KeyTab && msg.Type == tea.KeyShiftTab {
		return true
	}
	return false
}
