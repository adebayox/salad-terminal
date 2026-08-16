package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/salad-ai/salad-terminal/internal/config"
)

func TestLoginGoogleBrowserCompletesLoopbackCallback(t *testing.T) {
	t.Setenv(config.EnvConfigDir, t.TempDir())
	defer func() { _ = config.ClearCredentials() }()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mobile/auth/exchange" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer browser-session-token" {
			t.Fatalf("exchange authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"session":{"accessToken":"terminal-access","refreshToken":"terminal-refresh","expiresAt":"2030-01-01T00:00:00Z","userId":"user-1","installId":"install-1"},"user":{"email":"qa@example.test","name":"QA"}}`)
	}))
	defer apiServer.Close()

	previousBrowser := browserOpenForLogin
	defer func() { browserOpenForLogin = previousBrowser }()
	browserOpenForLogin = func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		callback, err := url.Parse(parsed.Query().Get("desktop_redirect_uri"))
		if err != nil || callback.Query().Get("state") == "" {
			t.Fatalf("desktop callback URL = %q, err=%v", callback, err)
		}

		getResponse, err := http.Get(callback.String())
		if err != nil {
			return err
		}
		getBody, _ := io.ReadAll(getResponse.Body)
		_ = getResponse.Body.Close()
		if getResponse.StatusCode != http.StatusOK || !bytes.Contains(getBody, []byte("Finishing sign-in")) {
			t.Fatalf("callback GET status=%d body=%s", getResponse.StatusCode, getBody)
		}

		payload, _ := json.Marshal(browserCallbackPayload{State: callback.Query().Get("state"), Token: "browser-session-token"})
		postResponse, err := http.Post(callback.String(), "application/json", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer postResponse.Body.Close()
		if postResponse.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(postResponse.Body)
			t.Fatalf("callback POST status=%d body=%s", postResponse.StatusCode, body)
		}
		return nil
	}

	if err := LoginGoogleBrowser(apiServer.URL); err != nil {
		t.Fatalf("LoginGoogleBrowser: %v", err)
	}
	credentials, err := config.LoadCredentials()
	if err != nil {
		t.Fatalf("load saved credentials: %v", err)
	}
	if credentials.Email != "qa@example.test" || credentials.AccessToken != "terminal-access" || !strings.HasPrefix(credentials.ExpiresAt, "2030-01-01") {
		t.Fatalf("unexpected saved credentials: %+v", credentials)
	}
}
