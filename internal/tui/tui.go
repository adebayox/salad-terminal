package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/salad-ai/salad-terminal/internal/chat"
	"github.com/salad-ai/salad-terminal/internal/config"
)

// Run starts a minimal send loop for the active (or provided) chat.
func Run(chatID string) error {
	if chatID == "" {
		if _, err := config.LoadActiveChat(); err != nil {
			fmt.Println("No active chat. Pick one to resume:")
			picked, pickErr := chat.PickInteractive()
			if pickErr != nil {
				return pickErr
			}
			if err := chat.Resume(picked); err != nil {
				return err
			}
			chatID = picked
		} else {
			active, _ := config.LoadActiveChat()
			chatID = active.ChatID
			if active.Title != "" {
				fmt.Printf("Salad · %s\n", active.Title)
			}
		}
	}
	fmt.Println("Type a message and press Enter. Commands: /participants  /quit")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch {
		case line == "/quit" || line == "/exit":
			return nil
		case line == "/participants":
			if err := chat.ShowParticipants(chatID); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		default:
			if err := chat.Send(chatID, line); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		}
	}
	return scanner.Err()
}
