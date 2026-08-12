package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/config"
)

func randomB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

type browserCallbackPayload struct {
	State string `json:"state"`
	Token string `json:"token"`
	Error string `json:"error"`
}

const browserCallbackHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>Salad Terminal</title></head>
<body style="font-family:system-ui;padding:2rem;background:#fbfbfa;color:#202123">
<h2>Salad Terminal</h2><p id="message">Completing sign-in…</p>
<script>
(async () => {
  const query = new URLSearchParams(window.location.search)
  const hash = new URLSearchParams(window.location.hash.slice(1))
  const response = await fetch('/callback', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({state: query.get('state'), token: hash.get('token'), error: hash.get('error')})
  })
  if (!response.ok) throw new Error('The terminal could not finish sign-in.')
  document.getElementById('message').textContent = hash.get('error')
    ? 'Sign-in failed. You can close this tab and return to the terminal.'
    : 'Signed in. You can close this tab and return to the terminal.'
})().catch((error) => {
  document.getElementById('message').textContent = error.message
})
</script></body></html>`

// LoginGoogleBrowser uses Salad's web OAuth callback, then exchanges the
// short-lived web session for a terminal session. This keeps Google redirect
// configuration on the backend and preserves terminal device telemetry.
func LoginGoogleBrowser(baseURL string) error {
	state, err := randomB64(32)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback?state=%s", port, url.QueryEscape(state))

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("state") != state {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, browserCallbackHTML)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload browserCallbackPayload
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&payload); err != nil {
			http.Error(w, "invalid callback", http.StatusBadRequest)
			return
		}
		if payload.State != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if payload.Error != "" {
			select {
			case errCh <- fmt.Errorf("browser sign-in: %s", payload.Error):
			default:
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if payload.Token == "" {
			http.Error(w, "missing sign-in token", http.StatusBadRequest)
			return
		}
		select {
		case tokenCh <- payload.Token:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	authURL, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/auth/google")
	if err != nil {
		return fmt.Errorf("invalid Salad API URL: %w", err)
	}
	query := authURL.Query()
	query.Set("desktop_redirect_uri", redirectURI)
	authURL.RawQuery = query.Encode()

	fmt.Println("Opening Salad sign-in in your browser…")
	fmt.Println(authURL.String())
	if err := openBrowser(authURL.String()); err != nil {
		fmt.Println("Could not open browser automatically. Open the URL above.")
	}

	var webToken string
	select {
	case webToken = <-tokenCh:
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Minute):
		return fmt.Errorf("timed out waiting for browser sign-in")
	}

	installID := uuid.NewString()
	if existing, err := config.LoadCredentials(); err == nil && existing.InstallID != "" {
		installID = existing.InstallID
	}
	device := DeviceInfo(installID)
	client := api.New(baseURL, webToken)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := client.ExchangeMobileSession(ctx, device)
	if err != nil {
		return fmt.Errorf("finish browser sign-in: %w", err)
	}
	creds := &config.Credentials{
		AccessToken:  resp.Session.AccessToken,
		RefreshToken: resp.Session.RefreshToken,
		ExpiresAt:    resp.Session.ExpiresAt.Format(time.RFC3339),
		UserID:       firstNonEmpty(resp.Session.UserID, resp.User.ID),
		Email:        resp.User.Email,
		Name:         resp.User.Name,
		InstallID:    firstNonEmpty(resp.Session.InstallID, installID),
		BaseURL:      baseURL,
	}
	if err := config.SaveCredentials(creds); err != nil {
		return err
	}
	fmt.Printf("Logged in as %s\n", displayName(creds))
	return nil
}
