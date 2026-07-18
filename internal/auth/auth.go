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
		Platform:   "unknown",
		AppVersion: "0.1.0",
		DeviceName: "Salad Terminal (" + hostname + ")",
	}
}

func LoginInteractive(baseURL string) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return err
	}
	password := string(passwordBytes)
	if email == "" || password == "" {
		return fmt.Errorf("email and password are required")
	}
	return Login(baseURL, email, password)
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
	creds, err := config.LoadCredentials()
	if err != nil {
		return fmt.Errorf("not logged in (run: salad login)")
	}
	client := api.New(config.BaseURL(), creds.AccessToken)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	user, err := client.Me(ctx)
	if err != nil {
		// Fall back to cached credentials if /me shape differs.
		fmt.Printf("%s <%s>\n", displayName(creds), creds.Email)
		fmt.Printf("user_id=%s base_url=%s\n", creds.UserID, config.BaseURL())
		return nil
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
	return api.New(config.BaseURL(), creds.AccessToken), creds, nil
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
