package harness

import (
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

	proxy := &ProviderProxy{token: token, url: "http://" + listener.Addr().String()}
	proxy.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.handle(ctx, client, provider, w, r)
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
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
