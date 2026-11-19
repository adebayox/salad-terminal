package realtime

import (
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
	EventID int64           `json:"event_id"`
	Type    string          `json:"type"`
	ChatID  string          `json:"chat_id"`
	Data    json.RawMessage `json:"data"`
}

// Client keeps its stream position across reconnects: Connect resumes after
// the last event it delivered, so a dropped connection never loses a tool
// request.
type Client struct {
	baseURL string
	token   string

	mu          sync.Mutex
	conn        *websocket.Conn
	closed      bool
	done        chan struct{}
	lastEventID int64
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
	return u.String(), nil
}

// Connect dials salad.v1 and returns a channel of events. Caller should cancel by Close().
func (c *Client) Connect() (<-chan Event, error) {
	endpoint, err := c.wsURL()
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		Subprotocols:     []string{"salad.v1"},
		HandshakeTimeout: 15 * time.Second,
	}
	// Never send a browser Origin. Empty Origin is allowed by SaladBE CheckOrigin
	// and avoids Cloudflare/proxy rejecting CLI clients as foreign web origins.
	header := http.Header{}
	header.Del("Origin")
	header.Set("Authorization", "Bearer "+c.token)
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
	c.done = make(chan struct{})
	done, lastEventID := c.done, c.lastEventID
	c.mu.Unlock()

	if lastEventID > 0 {
		// The server replays every stored event after this position.
		if err := conn.WriteJSON(map[string]any{"type": "resume", "last_event_id": lastEventID}); err != nil {
			c.Close()
			return nil, err
		}
	}

	out := make(chan Event, 32)
	go c.readLoop(conn, done, out)
	go c.pingLoop()
	return out, nil
}

func (c *Client) readLoop(conn *websocket.Conn, done <-chan struct{}, out chan<- Event) {
	defer close(out)
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var evt Event
		if err := json.Unmarshal(payload, &evt); err != nil {
			// Some frames use nested envelope.
			var wrap struct {
				EventID int64           `json:"event_id"`
				Type    string          `json:"type"`
				ChatID  string          `json:"chat_id"`
				Event   json.RawMessage `json:"event"`
				Data    json.RawMessage `json:"data"`
			}
			if json.Unmarshal(payload, &wrap) != nil {
				continue
			}
			evt.EventID = wrap.EventID
			evt.Type = wrap.Type
			evt.ChatID = wrap.ChatID
			evt.Data = wrap.Data
			if len(evt.Data) == 0 {
				evt.Data = wrap.Event
			}
		}
		if evt.Type == "" || !c.advance(evt.EventID) {
			continue
		}
		// Every event is delivered: a dropped tool request would stall the turn.
		select {
		case out <- evt:
		case <-done:
			return
		}
	}
}

// advance records a stored event's position and reports whether the event is
// new (a replay can overlap events already delivered).
func (c *Client) advance(eventID int64) bool {
	if eventID <= 0 {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if eventID <= c.lastEventID {
		return false
	}
	c.lastEventID = eventID
	return true
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
		if err := conn.WriteJSON(map[string]any{"type": "ping", "ts": time.Now().Unix()}); err != nil {
			c.Close()
			return
		}
	}
}

// ResumeFrom sets the stream position a new client continues from: the server
// replays every stored event after it, so a reconnect loses no tool request.
func (c *Client) ResumeFrom(eventID int64) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventID = eventID
	return c
}

// LastEventID is the position of the last stored event delivered.
func (c *Client) LastEventID() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastEventID
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.done != nil {
		close(c.done)
		c.done = nil
	}
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

func IsToolRequest(evt Event) bool {
	return strings.EqualFold(strings.TrimSpace(evt.Type), "tool_request")
}
