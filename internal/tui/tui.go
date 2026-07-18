package tui

import "github.com/salad-ai/salad-terminal/internal/app"

// Run launches the Salad Terminal surface (same chats as the web app).
func Run(chatID string) error {
	return app.Run(chatID)
}
