package harness

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
)

func TestProviderProxyKeepsSaladTokenOutOfChildBoundary(t *testing.T) {
	const saladToken = "salad-session-token"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+saladToken {
			t.Fatalf("upstream authorization = %q", got)
		}
		if got := r.Header.Get("X-Salad-Harness-Provider"); got != "openai" {
			t.Fatalf("upstream provider = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer upstream.Close()

	client := api.New(upstream.URL, saladToken)
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()
	env := strings.Join(proxy.Environment(), "\n")
	if strings.Contains(env, saladToken) {
		t.Fatal("provider proxy environment contains the Salad access token")
	}
	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":false,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("proxy status = %d body=%s", response.StatusCode, body)
	}
}

func TestProviderProxyRefreshesExpiredSaladSession(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer refreshed-token" {
			t.Fatalf("refreshed upstream authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer upstream.Close()

	client := api.New(upstream.URL, "expired-token")
	client.RefreshFunc = func(context.Context) error {
		client.AccessToken = "refreshed-token"
		return nil
	}
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()

	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":false,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("proxy refresh status = %d body=%s", response.StatusCode, body)
	}
	if requests != 2 {
		t.Fatalf("upstream requests = %d, want 2", requests)
	}
}

func TestProviderProxyRetriesTransientGatewayFailure(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"recovered"}}]}`)
	}))
	defer upstream.Close()

	client := api.New(upstream.URL, "salad-token")
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()

	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":false,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("proxy retry status = %d, want 200", response.StatusCode)
	}
	if requests != 2 {
		t.Fatalf("upstream requests = %d, want 2", requests)
	}
}

func TestProviderProxyReusesSaladClientTransport(t *testing.T) {
	called := false
	client := api.New("https://provider.invalid", "salad-token")
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)),
			Request:    req,
		}, nil
	})
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()

	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":false,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("proxy status = %d, want 200", response.StatusCode)
	}
	if !called {
		t.Fatal("provider proxy did not reuse the authenticated Salad HTTP transport")
	}
}

func TestProviderProxySurfacesStreamProviderErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"error\":{\"code\":\"HARNESS_PROVIDER_REQUEST_FAILED\"}}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := api.New(upstream.URL, "salad-token")
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()

	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("proxy error status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "HARNESS_PROVIDER_REQUEST_FAILED") {
		t.Fatalf("proxy error body = %s", body)
	}
}

func TestProviderProxyForwardsStreamSuccess(t *testing.T) {
	const streamBody = ": keepalive\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, streamBody)
	}))
	defer upstream.Close()

	client := api.New(upstream.URL, "salad-token")
	proxy, err := StartProviderProxy(context.Background(), client, "openai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = proxy.Close(ctx)
	}()

	apiKey := strings.TrimPrefix(strings.Split(proxy.Environment()[0], "=")[1], "")
	req, err := http.NewRequest(http.MethodPost, proxy.BaseURL()+"/v1/chat/completions", strings.NewReader(`{"stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("proxy stream status = %d, want 200", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if string(body) != streamBody {
		t.Fatalf("proxy stream body = %q, want %q", body, streamBody)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
