package realtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
