package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

// Banner returns a FIGlet-style SALAD wordmark (the big "ancient computer" letters).
// Falls back to a compact mark when the terminal is too narrow.
func Banner(width int) string {
	font := "slant"
	if width > 0 && width < 52 {
		font = "small"
	}
	raw := figure.NewFigure("SALAD", font, true).String()
	raw = strings.TrimRight(raw, "\n")
	styled := lipgloss.NewStyle().
		Bold(true).
		Foreground(Ink).
		Render(raw)
	rule := MutedText().Render(strings.Repeat("─", minWidth(width, 40)))
	tag := MutedText().Render("terminal")
	return lipgloss.JoinVertical(lipgloss.Left, styled, rule+"  "+tag)
}

// Mark is the compact in-session brand (room headers).
func Mark() string {
	return lipgloss.NewStyle().Bold(true).Foreground(Ink).Render("∬alad")
}

func minWidth(w, fallback int) int {
	if w <= 0 {
		return fallback
	}
	if w > 56 {
		return 56
	}
	if w < 20 {
		return 20
	}
	return w
}
