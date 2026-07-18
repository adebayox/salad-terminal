package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/config"
)

func List() error {
	client, _, err := auth.AuthedClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	boot, err := client.Bootstrap(ctx)
	if err != nil {
		return err
	}
	if len(boot.Chats) == 0 {
		fmt.Println("No chats yet.")
		return nil
	}
	for i, chat := range boot.Chats {
		title := firstNonEmpty(chat.Title, chat.ID)
		unread := ""
		if chat.UnreadCount > 0 {
			unread = fmt.Sprintf(" (%d unread)", chat.UnreadCount)
		}
		members := ""
		if len(chat.MemberNames) > 0 {
			members = " · " + strings.Join(chat.MemberNames, ", ")
		}
		fmt.Printf("%2d. %s%s%s\n    id=%s\n", i+1, title, unread, members, chat.ID)
	}
	return nil
}

func Resume(chatID string) error {
	client, _, err := auth.AuthedClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	title := chatID
	boot, err := client.ChatBootstrap(ctx, chatID)
	if err == nil && boot.Chat.Name != "" {
		title = boot.Chat.Name
	} else if list, listErr := client.Bootstrap(ctx); listErr == nil {
		for _, c := range list.Chats {
			if c.ID == chatID {
				title = firstNonEmpty(c.Title, chatID)
				break
			}
		}
	}

	if err := config.SaveActiveChat(&config.ActiveChat{ChatID: chatID, Title: title}); err != nil {
		return err
	}
	fmt.Printf("Resumed chat: %s\n", title)
	fmt.Printf("id=%s\n", chatID)

	if boot != nil {
		if len(boot.Chat.MemberNames) > 0 {
			fmt.Printf("participants: %s\n", strings.Join(boot.Chat.MemberNames, ", "))
		}
		printRecent(boot.Messages, 12)
		return nil
	}

	messages, msgErr := client.ListMessages(ctx, chatID)
	if msgErr == nil {
		printRecent(messages, 12)
	}
	return nil
}

func ShowParticipants(chatID string) error {
	if chatID == "" {
		active, err := config.LoadActiveChat()
		if err != nil {
			return fmt.Errorf("no active chat (run: salad resume <chat-id>)")
		}
		chatID = active.ChatID
	}
	client, _, err := auth.AuthedClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	members, err := client.ListMembers(ctx, chatID)
	if err != nil {
		boot, bootErr := client.ChatBootstrap(ctx, chatID)
		if bootErr != nil {
			return err
		}
		for _, name := range boot.Chat.MemberNames {
			fmt.Println("-", name)
		}
		return nil
	}
	for _, member := range members {
		name := firstNonEmpty(
			stringField(member, "display_name"),
			stringField(member, "name"),
			stringField(member, "email"),
			stringField(member, "id"),
		)
		fmt.Println("-", name)
	}
	return nil
}

func ActiveChatID() (string, error) {
	active, err := config.LoadActiveChat()
	if err != nil {
		return "", fmt.Errorf("no active chat (run: salad resume <chat-id> or salad chat)")
	}
	return active.ChatID, nil
}

func Send(chatID, content string) error {
	client, _, err := auth.AuthedClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	msg, err := client.SendMessage(ctx, chatID, content)
	if err != nil {
		return err
	}
	author := firstNonEmpty(msg.AuthorName, "you")
	body := firstNonEmpty(msg.Body, content)
	fmt.Printf("[%s] %s\n", author, body)
	return nil
}

func printRecent(messages []api.ChatMessage, limit int) {
	if len(messages) == 0 {
		fmt.Println("(no recent messages)")
		return
	}
	start := 0
	if len(messages) > limit {
		start = len(messages) - limit
	}
	for _, msg := range messages[start:] {
		author := firstNonEmpty(msg.AuthorName, msg.Role, "unknown")
		fmt.Printf("[%s] %s\n", author, strings.TrimSpace(msg.Body))
	}
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprint(v)
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
