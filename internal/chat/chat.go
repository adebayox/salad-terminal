package chat

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/bridge"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

const listLimit = 15

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
	printChatList(boot.Chats)
	return nil
}

// PickInteractive lists recent chats and lets the developer choose one by number or id.
func PickInteractive() (string, error) {
	client, _, err := auth.AuthedClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	boot, err := client.Bootstrap(ctx)
	if err != nil {
		return "", err
	}
	if len(boot.Chats) == 0 {
		return "", fmt.Errorf("no chats yet")
	}
	printChatList(boot.Chats)
	fmt.Print("Resume chat # (or paste chat id): ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return "", fmt.Errorf("no chat selected")
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(boot.Chats) || n > listLimit {
			return "", fmt.Errorf("chat # out of range")
		}
		return boot.Chats[n-1].ID, nil
	}
	return choice, nil
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
	if err == nil {
		title = firstNonEmpty(boot.Chat.Title, boot.Chat.Name, chatID)
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
		members := boot.Chat.MemberNames
		if len(members) == 0 {
			if listed, listErr := client.ListMembers(ctx, chatID); listErr == nil {
				for _, member := range listed {
					name := firstNonEmpty(
						stringField(member, "display_name"),
						stringField(member, "name"),
						stringField(member, "email"),
					)
					if name != "" {
						members = append(members, name)
					}
				}
			}
		}
		if len(members) > 0 {
			fmt.Printf("participants: %s\n", strings.Join(members, ", "))
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

	req := api.SendMessageRequest{
		Content: content,
		Metadata: map[string]any{
			"client_surface": "salad_terminal",
		},
	}
	root, _ := workspace.ResolveRoot("")
	if workspace.IsTrusted(root) {
		if codeCtx, _, bridgeErr := bridge.BuildCodeContext(root, nil); bridgeErr == nil {
			req.CodeContext = codeCtx
		}
	}
	msg, err := client.SendMessageRequest(ctx, chatID, req)
	if err != nil {
		return err
	}
	author := firstNonEmpty(msg.AuthorName, "you")
	body := firstNonEmpty(msg.Body, content)
	fmt.Printf("[%s] %s\n", author, body)

	if reply := waitForAssistantReply(ctx, client, chatID, msg.ID, 20*time.Second); reply != nil {
		fmt.Printf("[%s] %s\n", firstNonEmpty(reply.AuthorName, "assistant"), strings.TrimSpace(reply.Body))
	} else {
		fmt.Println("(still waiting on assistant — open: salad resume <chat-id>)")
	}
	return nil
}

func waitForAssistantReply(ctx context.Context, client *api.Client, chatID, afterID string, timeout time.Duration) *api.ChatMessage {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(700 * time.Millisecond):
		}
		boot, err := client.ChatBootstrap(ctx, chatID)
		if err != nil {
			continue
		}
		seenAfter := afterID == ""
		for i := range boot.Messages {
			msg := boot.Messages[i]
			if !seenAfter {
				if msg.ID == afterID {
					seenAfter = true
				}
				continue
			}
			role := strings.ToLower(msg.Role)
			if role == "assistant" || role == "ai" || looksLikeAssistant(msg.AuthorName) {
				if strings.TrimSpace(msg.Body) != "" {
					return &msg
				}
			}
		}
	}
	return nil
}

func looksLikeAssistant(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "you" {
		return false
	}
	for _, needle := range []string{"gpt", "claude", "gemini", "grok", "mistral", "llama", "groq", "chatgpt"} {
		if strings.Contains(name, needle) {
			return true
		}
	}
	return false
}

func printChatList(chats []api.ChatPreview) {
	limit := listLimit
	if len(chats) < limit {
		limit = len(chats)
	}
	for i := 0; i < limit; i++ {
		chat := chats[i]
		title := firstNonEmpty(chat.Title, chat.ID)
		unread := ""
		if chat.UnreadCount > 0 {
			unread = fmt.Sprintf(" (%d unread)", chat.UnreadCount)
		}
		members := ""
		if len(chat.MemberNames) > 0 {
			members = " · " + strings.Join(chat.MemberNames, ", ")
		}
		fmt.Printf("%2d. %s%s%s\n", i+1, title, unread, members)
	}
	if len(chats) > listLimit {
		fmt.Printf("… %d more not shown. Use: salad resume <chat-id>\n", len(chats)-listLimit)
	}
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
