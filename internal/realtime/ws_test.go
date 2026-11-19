package realtime

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestConnectSendsBearerHeaderWithoutPuttingTokenInURL(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "" {
			t.Errorf("token leaked into query string: %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Origin"); got != "" {
			t.Errorf("Origin = %q, want empty", got)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	client := New(server.URL, "secret-token")
	_, err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
}

// A dropped connection must not lose a tool request: the next client resumes
// after the last stored event and drops replayed duplicates.
func TestReconnectResumesAfterLastEventWithoutDuplicates(t *testing.T) {
	upgrader := websocket.Upgrader{Subprotocols: []string{"salad.v1"}}
	var mu sync.Mutex
	connections := 0
	resumedFrom := int64(-1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		mu.Lock()
		connections++
		n := connections
		mu.Unlock()
		if n == 1 {
			_ = conn.WriteJSON(map[string]any{"event_id": 1, "type": "tool_request"})
			_ = conn.WriteJSON(map[string]any{"event_id": 2, "type": "tool_request"})
			return // the connection drops
		}
		var resume struct {
			Type        string `json:"type"`
			LastEventID int64  `json:"last_event_id"`
		}
		if err := conn.ReadJSON(&resume); err == nil && resume.Type == "resume" {
			mu.Lock()
			resumedFrom = resume.LastEventID
			mu.Unlock()
		}
		_ = conn.WriteJSON(map[string]any{"event_id": 2, "type": "tool_request"}) // replay overlap
		_ = conn.WriteJSON(map[string]any{"event_id": 3, "type": "tool_request"})
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	read := func(client *Client) []int64 {
		defer client.Close()
		ch, err := client.Connect()
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for evt := range ch {
			ids = append(ids, evt.EventID)
		}
		return ids
	}
	first := New(server.URL, "token")
	if got := read(first); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("first connection delivered %v, want [1 2]", got)
	}
	got := read(New(server.URL, "token").ResumeFrom(first.LastEventID()))
	mu.Lock()
	defer mu.Unlock()
	if resumedFrom != 2 {
		t.Fatalf("reconnect resumed from %d, want 2", resumedFrom)
	}
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("reconnect delivered %v, want only [3]", got)
	}
}
