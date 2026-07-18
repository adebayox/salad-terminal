package tui

import "github.com/salad-ai/salad-terminal/internal/app"

// Run launches the Salad Terminal surface (same chats as the web app).
func Run(chatID string) error {
	return app.RunOptions(app.Options{ChatID: chatID})
}

// RunResume always shows the resume picker (like Claude Code --resume).
func RunResume() error {
	return app.RunOptions(app.Options{ForceResume: true})
}

// RunNew creates a new Salad chat and opens it.
func RunNew() error {
	return app.RunOptions(app.Options{ForceNew: true})
}
