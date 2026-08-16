package harness

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
)

const maxProviderRequestBytes = 8 * 1024 * 1024

// ProviderProxy is a loopback-only OpenAI-compatible endpoint for the DSH
// provider adapter. The Salad access token stays in this process and is never
// included in the child environment.
type ProviderProxy struct {
	server *http.Server
	cancel context.CancelFunc
	token  string
	url    string
}

// StartProviderProxy starts an ephemeral loopback proxy backed by Salad's
// authenticated harness provider endpoint.
func StartProviderProxy(ctx context.Context, client *api.Client, provider string) (*ProviderProxy, error) {
	if client == nil || strings.TrimSpace(client.BaseURL) == "" || strings.TrimSpace(client.AccessToken) == "" {
		return nil, fmt.Errorf("authenticated Salad client is required for the provider bridge")
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("create provider bridge token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start provider bridge: %w", err)
	}

	bridgeCtx, cancel := context.WithCancel(ctx)
	proxy := &ProviderProxy{cancel: cancel, token: token, url: "http://" + listener.Addr().String()}
	proxy.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.handle(bridgeCtx, client, provider, w, r)
	})}
	go func() {
		_ = proxy.server.Serve(listener)
	}()
	return proxy, nil
}

func (p *ProviderProxy) BaseURL() string { return p.url }

func (p *ProviderProxy) Environment() []string {
	return []string{"DEEPSEEK_API_KEY=" + p.token, "DEEPSEEK_BASE_URL=" + p.url}
}

func (p *ProviderProxy) Close(ctx context.Context) error {
	if p == nil || p.server == nil {
		return nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	return p.server.Shutdown(ctx)
}

func (p *ProviderProxy) handle(parent context.Context, client *api.Client, provider string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || (r.URL.Path != "/chat/completions" && r.URL.Path != "/v1/chat/completions") {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+p.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxProviderRequestBytes+1))
	if err != nil || len(body) > maxProviderRequestBytes {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	var envelope struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &envelope)
	requestCtx, cancel := context.WithTimeout(parent, 125*time.Second)
	defer cancel()
	doUpstream := func() (*http.Response, error) {
		upstream, requestErr := http.NewRequestWithContext(requestCtx, http.MethodPost, strings.TrimRight(client.BaseURL, "/")+"/api/harness/provider/v1/chat/completions", strings.NewReader(string(body)))
		if requestErr != nil {
			return nil, requestErr
		}
		upstream.Header.Set("Authorization", "Bearer "+client.AccessToken)
		upstream.Header.Set("Content-Type", "application/json")
		upstream.Header.Set("Accept", r.Header.Get("Accept"))
		if envelope.Stream {
			upstream.Header.Set("X-Salad-Harness-Stream", "1")
		}
		if strings.TrimSpace(provider) != "" {
			upstream.Header.Set("X-Salad-Harness-Provider", provider)
		}
		// Reuse the authenticated Salad client's transport. Creating a fresh
		// default http.Client here would re-enable the proxy HTTP/2 failure
		// that the normal Salad API client deliberately avoids.
		upstreamClient := http.Client{Timeout: 130 * time.Second}
		if client.HTTP != nil {
			upstreamClient = *client.HTTP
			upstreamClient.Timeout = 130 * time.Second
		}
		return upstreamClient.Do(upstream)
	}
	response, err := doUpstream()
	if err == nil && response.StatusCode == http.StatusUnauthorized && client.RefreshFunc != nil {
		_ = response.Body.Close()
		if refreshErr := client.RefreshFunc(requestCtx); refreshErr == nil {
			response, err = doUpstream()
		}
	}
	if shouldRetryProvider(response, err) && requestCtx.Err() == nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-requestCtx.Done():
			timer.Stop()
		case <-timer.C:
			response, err = doUpstream()
		}
	}
	if err != nil {
		http.Error(w, "provider request failed", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if envelope.Stream && response.StatusCode == http.StatusOK && strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		p.relayProviderStream(w, response.Body)
		return
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func shouldRetryProvider(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if response == nil {
		return true
	}
	return response.StatusCode == http.StatusBadGateway ||
		response.StatusCode == http.StatusServiceUnavailable ||
		response.StatusCode == http.StatusGatewayTimeout
}

// relayProviderStream holds only the initial SSE prelude until the first data
// event is known to be a completion. The Salad backend uses HTTP 200 so it can
// send keepalives while a provider works; when the provider fails, it emits a
// structured SSE error after those keepalives. Translating that case back to a
// real HTTP error prevents DSH from misreporting the failure as an empty model
// response.
func (p *ProviderProxy) relayProviderStream(w http.ResponseWriter, body io.Reader) {
	// The upstream may advertise a content length for the original stream;
	// error translation changes the body and must let net/http frame it.
	w.Header().Del("Content-Length")
	reader := bufio.NewReader(body)
	var prelude strings.Builder
	for {
		line, err := reader.ReadString('\n')
		prelude.WriteString(line)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			if code, ok := providerStreamErrorCode(strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))); ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, `{"error":"Salad model provider request failed","code":"`+code+`"}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, prelude.String())
			_, _ = io.Copy(w, reader)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"error":"Salad model provider stream ended unexpectedly"}`)
			return
		}
	}
}

func providerStreamErrorCode(data string) (string, bool) {
	if data == "" || data == "[DONE]" {
		return "", false
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil || strings.TrimSpace(envelope.Error.Code) == "" {
		return "", false
	}
	return strings.TrimSpace(envelope.Error.Code), true
}
