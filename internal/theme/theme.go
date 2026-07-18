package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Salad app chrome: cream canvas, charcoal text, calm collaboration.
var (
	Cream     = lipgloss.Color("#fbfbfa")
	CreamSoft = lipgloss.Color("#f4f4f2")
	Ink       = lipgloss.Color("#202123")
	Muted     = lipgloss.Color("#6b6f76")
	Unread    = lipgloss.Color("#4fa3ff")
	Claude    = lipgloss.Color("#8B5CF6")
	GPT       = lipgloss.Color("#10B981")
	Gemini    = lipgloss.Color("#3B82F6")
	Grok      = lipgloss.Color("#F59E0B")
	Mistral   = lipgloss.Color("#F97316")
	Danger    = lipgloss.Color("#DC2626")
)

func Brand() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(Ink)
}

func MutedText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Muted)
}

func Header() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(Ink).
		Background(CreamSoft).
		Padding(0, 1)
}

func Footer() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Muted).Padding(0, 1)
}

func Selected() lipgloss.Style {
	// High-contrast so the active row is obvious even without truecolor.
	return lipgloss.NewStyle().
		Foreground(Cream).
		Background(Ink).
		Bold(true).
		Padding(0, 1)
}

func ListItem() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Ink).Padding(0, 1)
}

func UserBubble() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(Ink).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Muted).
		Padding(0, 1).
		MarginLeft(4)
}

func AIHeader(name string) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(AgentColor(name))
}

func AIBody() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Ink).MarginLeft(2)
}

func Composer() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Ink).
		Padding(0, 1)
}

func Error() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Danger)
}

func UnreadDot() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Unread).Bold(true)
}

func AgentColor(name string) lipgloss.Color {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "claude"):
		return Claude
	case strings.Contains(n, "gpt"), strings.Contains(n, "chatgpt"):
		return GPT
	case strings.Contains(n, "gemini"):
		return Gemini
	case strings.Contains(n, "grok"):
		return Grok
	case strings.Contains(n, "mistral"), strings.Contains(n, "llama"), strings.Contains(n, "groq"):
		return Mistral
	default:
		return Unread
	}
}
