package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

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
