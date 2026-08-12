package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHumanizeErrorHidesHTTPStatusAndInternalAuthCode(t *testing.T) {
	err := &APIError{Status: http.StatusUnauthorized, Code: "AUTH_INVALID_CREDENTIALS", Message: "Invalid email or password"}
	got := HumanizeError(err)
	if got != "The email or password is not correct. Check it and try again." {
		t.Fatalf("HumanizeError() = %q", got)
	}
}

func TestHumanizeErrorHandlesUnavailableService(t *testing.T) {
	err := &APIError{Status: http.StatusBadGateway, Code: "UPSTREAM_FAILURE", Message: "upstream unavailable"}
	if got := HumanizeError(err); got != "Salad is temporarily unavailable. Try again in a moment." {
		t.Fatalf("HumanizeError() = %q", got)
	}
}

func TestClientRefreshesOnceAfterUnauthorizedResponse(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"TOKEN_INVALID","message":"expired"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-1","email":"qa@example.test"}`))
	}))
	defer server.Close()

	client := New(server.URL, "expired-token")
	var refreshes atomic.Int32
	client.RefreshFunc = func(context.Context) error {
		refreshes.Add(1)
		client.AccessToken = "fresh-token"
		return nil
	}

	user, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if user.Email != "qa@example.test" {
		t.Fatalf("user email = %q", user.Email)
	}
	if refreshes.Load() != 1 || requests.Load() != 2 {
		t.Fatalf("refreshes=%d requests=%d, want one refresh and two requests", refreshes.Load(), requests.Load())
	}
}

func TestClientDoesNotRetryUnauthorizedResponseTwice(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.URL, "expired-token")
	var refreshes atomic.Int32
	client.RefreshFunc = func(context.Context) error {
		refreshes.Add(1)
		return nil
	}

	if _, err := client.Me(context.Background()); err == nil {
		t.Fatal("Me() unexpectedly succeeded")
	}
	if refreshes.Load() != 1 || requests.Load() != 2 {
		t.Fatalf("refreshes=%d requests=%d, want one refresh and two requests", refreshes.Load(), requests.Load())
	}
}

func TestClientRejectsPathInjectionInChatIDs(t *testing.T) {
	client := New("http://127.0.0.1:1", "token")
	if _, err := client.ChatBootstrap(context.Background(), "chat/../../me"); err == nil {
		t.Fatal("ChatBootstrap accepted a path-injection chat ID")
	}
	if _, err := client.ListMessages(context.Background(), "chat?token=leak", ""); err == nil {
		t.Fatal("ListMessages accepted a query-injection chat ID")
	}
	if _, err := client.SendMessageRequest(context.Background(), "", SendMessageRequest{Content: "hello"}); err == nil {
		t.Fatal("SendMessageRequest accepted an empty chat ID")
	}
}

func TestClientEscapesMessageCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chats/chat-1/messages" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("before"); got != "message&next=1" {
			t.Fatalf("before cursor = %q", got)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := New(server.URL, "token")
	if _, err := client.ListMessages(context.Background(), "chat-1", "message&next=1"); err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
}
