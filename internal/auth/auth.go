package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/config"
	"golang.org/x/term"
)

const signupURL = "https://salad.ink/?auth=signup"

// BuildVersion is stamped by the release workflow so auth telemetry identifies
// the exact terminal build that created a session.
var BuildVersion = "dev"

func EnsureInstallID(existing string) string {
	if strings.TrimSpace(existing) != "" {
		return existing
	}
	return uuid.NewString()
}

func DeviceInfo(installID string) api.DeviceInfo {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "salad-terminal"
	}
	return api.DeviceInfo{
		InstallID:  installID,
		Platform:   "terminal",
		AppVersion: BuildVersion + "-terminal",
		DeviceName: "Salad Terminal (" + hostname + ")",
	}
}

func LoginInteractive(baseURL string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("interactive sign-in needs a terminal; run `salad login --google` or pass `--email` and `--password`")
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Email address: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("could not read your email address: %w", err)
	}
	email = strings.TrimSpace(email)
	fmt.Print("Password (hidden): ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("could not read your password: %w", err)
	}
	password := string(passwordBytes)
	if email == "" || password == "" {
		return fmt.Errorf("email address and password are required")
	}
	return Login(baseURL, email, password)
}

// OpenSignup starts account creation in the user's browser. Account creation
// belongs to the web auth surface; the terminal resumes naturally after the
// user returns and runs `salad login`.
func OpenSignup() error {
	fmt.Println("Opening Salad account creation in your browser…")
	if err := OpenSignupBrowser(); err != nil {
		fmt.Println("Could not open the browser automatically.")
		fmt.Println("Open this URL:")
		fmt.Println(signupURL)
		return nil
	}
	fmt.Println("Create your account in the browser, then return here and run `salad login`.")
	return nil
}

func OpenSignupBrowser() error {
	return openBrowser(signupURL)
}

func Login(baseURL, email, password string) error {
	installID := uuid.NewString()
	if existing, err := config.LoadCredentials(); err == nil && existing.InstallID != "" {
		installID = existing.InstallID
	}
	client := api.New(baseURL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := client.Login(ctx, email, password, DeviceInfo(installID))
	if err != nil {
		return err
	}
	creds := &config.Credentials{
		AccessToken:  resp.Session.AccessToken,
		RefreshToken: resp.Session.RefreshToken,
		ExpiresAt:    resp.Session.ExpiresAt.Format(time.RFC3339),
		UserID:       firstNonEmpty(resp.Session.UserID, resp.User.ID),
		Email:        firstNonEmpty(resp.User.Email, email),
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

func Logout() error {
	creds, err := config.LoadCredentials()
	if err != nil {
		_ = config.ClearCredentials()
		fmt.Println("Logged out.")
		return nil
	}
	client := api.New(config.BaseURL(), creds.AccessToken)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = client.Logout(ctx, creds.RefreshToken)
	if err := config.ClearCredentials(); err != nil {
		return err
	}
	fmt.Println("Logged out.")
	return nil
}

func WhoAmI() error {
	client, creds, err := AuthedClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	user, err := client.Me(ctx)
	if err != nil {
		return fmt.Errorf("whoami failed: %w", err)
	}
	name := firstNonEmpty(user.Name, creds.Name, user.Email, creds.Email)
	email := firstNonEmpty(user.Email, creds.Email)
	fmt.Printf("%s <%s>\n", name, email)
	fmt.Printf("user_id=%s base_url=%s\n", firstNonEmpty(user.ID, creds.UserID), config.BaseURL())
	return nil
}

func AuthedClient() (*api.Client, *config.Credentials, error) {
	creds, err := config.LoadCredentials()
	if err != nil {
		return nil, nil, fmt.Errorf("not logged in (run: salad login)")
	}
	client := api.New(config.BaseURL(), creds.AccessToken)
	client.RefreshFunc = func(ctx context.Context) error {
		refreshClient := api.New(creds.BaseURL, "")
		response, refreshErr := refreshClient.Refresh(ctx, creds.RefreshToken, DeviceInfo(creds.InstallID))
		if refreshErr != nil {
			return refreshErr
		}
		if response.Session.AccessToken == "" || response.Session.RefreshToken == "" {
			return fmt.Errorf("refresh response did not contain a complete session")
		}
		creds.AccessToken = response.Session.AccessToken
		creds.RefreshToken = response.Session.RefreshToken
		creds.ExpiresAt = response.Session.ExpiresAt.Format(time.RFC3339)
		creds.UserID = firstNonEmpty(response.Session.UserID, response.User.ID, creds.UserID)
		creds.Email = firstNonEmpty(response.User.Email, creds.Email)
		creds.Name = firstNonEmpty(response.User.Name, creds.Name)
		creds.InstallID = firstNonEmpty(response.Session.InstallID, creds.InstallID)
		return config.SaveCredentials(creds)
	}
	return client, creds, nil
}

func displayName(creds *config.Credentials) string {
	return firstNonEmpty(creds.Name, creds.Email, creds.UserID, "salad user")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
