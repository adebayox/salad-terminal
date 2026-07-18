package realtime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is a salad.v1 realtime envelope (subset used by Terminal).
type Event struct {
	Type   string          `json:"type"`
	ChatID string          `json:"chat_id"`
	Data   json.RawMessage `json:"data"`
}

type Client struct {
	baseURL string
	token   string

	mu     sync.Mutex
	conn   *websocket.Conn
	closed bool
}

func New(baseURL, token string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token}
}

func (c *Client) wsURL() (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
		// ok
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/ws"
	q := u.Query()
	q.Set("token", c.token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func authProtocol(token string) string {
	enc := base64.RawURLEncoding.EncodeToString([]byte(token))
	return "salad.auth." + enc
}

// Connect dials salad.v1 and returns a channel of events. Caller should cancel by Close().
func (c *Client) Connect() (<-chan Event, error) {
	endpoint, err := c.wsURL()
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		Subprotocols:     []string{"salad.v1", authProtocol(c.token)},
		HandshakeTimeout: 15 * time.Second,
	}
	// Never send a browser Origin. Empty Origin is allowed by SaladBE CheckOrigin
	// and avoids Cloudflare/proxy rejecting CLI clients as foreign web origins.
	header := http.Header{}
	header.Del("Origin")
	conn, resp, err := dialer.Dial(endpoint, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("%w (http %d)", err, resp.StatusCode)
		}
		return nil, err
	}
	_ = resp
	c.mu.Lock()
	c.conn = conn
	c.closed = false
	c.mu.Unlock()

	out := make(chan Event, 32)
	go c.readLoop(out)
	go c.pingLoop()
	return out, nil
}

func (c *Client) readLoop(out chan<- Event) {
	defer close(out)
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var evt Event
		if err := json.Unmarshal(payload, &evt); err != nil {
			// Some frames use nested envelope.
			var wrap struct {
				Type   string          `json:"type"`
				ChatID string          `json:"chat_id"`
				Event  json.RawMessage `json:"event"`
				Data   json.RawMessage `json:"data"`
			}
			if json.Unmarshal(payload, &wrap) != nil {
				continue
			}
			evt.Type = wrap.Type
			evt.ChatID = wrap.ChatID
			evt.Data = wrap.Data
			if len(evt.Data) == 0 {
				evt.Data = wrap.Event
			}
		}
		if evt.Type == "" {
			continue
		}
		select {
		case out <- evt:
		default:
		}
	}
}

func (c *Client) pingLoop() {
	t := time.NewTicker(25 * time.Second)
	defer t.Stop()
	for range t.C {
		c.mu.Lock()
		conn := c.conn
		closed := c.closed
		c.mu.Unlock()
		if closed || conn == nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": "ping", "ts": time.Now().Unix()})
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func IsChatSignal(evt Event) bool {
	switch strings.ToLower(evt.Type) {
	case "message", "new_message", "message_updated", "stream_start", "stream_chunk", "stream_end", "status_update", "sync_required":
		return true
	default:
		return false
	}
}
