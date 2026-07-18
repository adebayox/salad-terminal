package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/config"
)

// Staging FE Google client ID (public). Override with SALAD_GOOGLE_CLIENT_ID.
const defaultGoogleClientID = "946937090982-cg1os1brpv4cidt8r37qkeudr546gnu3.apps.googleusercontent.com"

func googleClientID() string {
	if v := strings.TrimSpace(os.Getenv("SALAD_GOOGLE_CLIENT_ID")); v != "" {
		return v
	}
	return defaultGoogleClientID
}

func randomB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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

// LoginGoogleBrowser runs PKCE loopback Google OAuth, then exchanges via Salad mobile auth.
func LoginGoogleBrowser(baseURL string) error {
	verifier, err := randomB64(32)
	if err != nil {
		return err
	}
	state, err := randomB64(16)
	if err != nil {
		return err
	}
	challenge := codeChallenge(verifier)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch")
			return
		}
		if msg := r.URL.Query().Get("error"); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			errCh <- fmt.Errorf("google oauth: %s", msg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("missing oauth code")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body style="font-family:system-ui;padding:2rem;background:#fbfbfa;color:#202123">
<h2>∬alad Terminal</h2><p>Signed in. You can close this tab and return to the terminal.</p></body></html>`))
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	authURL := url.URL{
		Scheme: "https",
		Host:   "accounts.google.com",
		Path:   "/o/oauth2/v2/auth",
	}
	q := authURL.Query()
	q.Set("client_id", googleClientID())
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	authURL.RawQuery = q.Encode()

	fmt.Println("Opening Google sign-in in your browser…")
	fmt.Println(authURL.String())
	if err := openBrowser(authURL.String()); err != nil {
		fmt.Println("Could not open browser automatically. Open the URL above.")
	}

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Minute):
		return fmt.Errorf("timed out waiting for Google sign-in")
	}

	installID := uuid.NewString()
	if existing, err := config.LoadCredentials(); err == nil && existing.InstallID != "" {
		installID = existing.InstallID
	}
	client := api.New(baseURL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := client.LoginGoogle(ctx, code, verifier, redirectURI, DeviceInfo(installID))
	if err != nil {
		return fmt.Errorf("%w\nHint: add %s as an Authorized redirect URI on the Google OAuth client, or use email login", err, redirectURI)
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
