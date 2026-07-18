package tui

import (
	"strings"

	"github.com/salad-ai/salad-terminal/internal/app"
)

// Run launches Terminal. Bare launch (empty chatID) starts a new chat —
// same as Claude Code with no flags.
func Run(chatID string) error {
	if strings.TrimSpace(chatID) != "" {
		return app.RunOptions(app.Options{ChatID: chatID})
	}
	return app.RunOptions(app.Options{ForceNew: true})
}

// RunContinue resumes the last chat for this folder (claude --continue).
func RunContinue() error {
	return app.RunOptions(app.Options{ForceContinue: true})
}

// RunResume shows the resume picker (claude --resume).
func RunResume() error {
	return app.RunOptions(app.Options{ForceResume: true})
}

// RunNew starts the new-chat AI picker (same as bare Run).
func RunNew() error {
	return app.RunOptions(app.Options{ForceNew: true})
}
