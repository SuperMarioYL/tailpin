package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SuperMarioYL/tailpin/internal/pin"
)

// Colors stay in the 256-color palette so the panes remain readable on any
// terminal that supports color; lipgloss degrades to plain text when it does
// not.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	recordStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	anchorStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
)

var kindStyles = map[pin.Kind]lipgloss.Style{
	pin.KindFinding: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	pin.KindPlan:    lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
	pin.KindFix:     lipgloss.NewStyle().Foreground(lipgloss.Color("84")),
	pin.KindCaveat:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
}

// kindTag renders the colored claim-kind tag. The fixed padding keeps claim
// texts aligned across the kinds.
func kindTag(k pin.Kind) string {
	return kindStyles[k].Render(fmt.Sprintf("%-8s", "["+string(k)+"]"))
}

// paneHeader renders a labeled horizontal rule such as "── span ──────",
// truncated to width when the terminal is too narrow.
func paneHeader(label string, width int) string {
	label = "── " + label + " "
	if w := lipgloss.Width(label); w >= width {
		return dimStyle.Render(truncateRunes(label, width))
	}
	return dimStyle.Render(label + strings.Repeat("─", width-lipgloss.Width(label)))
}

// truncateRunes cuts s to at most width cells.
func truncateRunes(s string, width int) string {
	if width < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width])
}
