package reddit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/lepinkainen/feed-forge/internal/config"
	"golang.org/x/oauth2"
)

const (
	AuthPort = "8080"
)

var (
	AuthCodeChan = make(chan string)
	ServerWg     sync.WaitGroup
)

// handleAuthentication manages OAuth2 authentication flow
func handleAuthentication(cfg *config.Config) (*oauth2.Token, error) {
	if cfg.RedditOAuth.RefreshToken == "" {
		slog.Info("No refresh token found, starting browser authentication")
		return AuthenticateUser(cfg)
	}

	slog.Info("Refresh token found, attempting to refresh access token")
	token := &oauth2.Token{
		RefreshToken: cfg.RedditOAuth.RefreshToken,
		AccessToken:  cfg.RedditOAuth.AccessToken,
		Expiry:       cfg.RedditOAuth.ExpiresAt,
	}

	if !token.Valid() {
		slog.Info("Access token expired or invalid, refreshing")
		return RefreshAccessToken(cfg, token)
	}

	slog.Info("Access token is still valid")
	return token, nil
}

// AuthenticateUser starts a local web server, opens the browser for authentication,
// and retrieves the access and refresh tokens.
func AuthenticateUser(cfg *config.Config) (*oauth2.Token, error) {
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	oauthConfig := getOAuthConfig(cfg)

	ServerWg.Add(1)
	go func() {
		defer ServerWg.Done()
		http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			OAuth2CallbackHandler(w, r, AuthCodeChan)
		})
		slog.Info("Starting local HTTP server for OAuth2 callback", "port", AuthPort)
		server := &http.Server{Addr: ":" + AuthPort}

		go func() {
			<-serverCtx.Done()
			slog.Info("Received shutdown signal for local HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				slog.Error("Error shutting down HTTP server", "error", err)
			}
		}()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	authURL := oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("duration", "permanent"))

	slog.Info("Opening browser for Reddit authentication", "url", authURL)
	if err := OpenBrowser(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w. Please open the URL manually: %s", err, authURL)
	}

	authCode := <-AuthCodeChan

	if authCode == "" {
		return nil, fmt.Errorf("authentication failed: no authorization code received")
	}

	token, err := exchangeAuthCodeForTokens(oauthConfig, authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	cfg.RedditOAuth.AccessToken = token.AccessToken
	cfg.RedditOAuth.RefreshToken = token.RefreshToken
	cfg.RedditOAuth.ExpiresAt = token.Expiry
	if err := config.SaveConfig(cfg, ""); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	slog.Info("Authentication successful, tokens saved")

	// Cancel the server context to trigger shutdown
	serverCancel()
	ServerWg.Wait()
	return token, nil
}

// exchangeAuthCodeForTokens exchanges authorization code for tokens with retry logic
func exchangeAuthCodeForTokens(oauthConfig *oauth2.Config, authCode string) (*oauth2.Token, error) {
	const maxRetries = 5
	initialBackoff := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		token, err := oauthConfig.Exchange(ctx, authCode)
		if err == nil {
			return token, nil
		}

		if oe, ok := err.(*oauth2.RetrieveError); ok && oe.Response.StatusCode == http.StatusTooManyRequests {
			slog.Warn("Rate limited, retrying", "backoff", initialBackoff)
			time.Sleep(initialBackoff)
			initialBackoff *= 2
			continue
		}

		return nil, fmt.Errorf("failed to exchange authorization code for token after %d attempts: %w", i+1, err)
	}

	return nil, fmt.Errorf("failed to exchange authorization code for token after %d retries", maxRetries)
}

// OAuth2CallbackHandler handles the redirect from Reddit after user authentication.
func OAuth2CallbackHandler(w http.ResponseWriter, r *http.Request, authCodeChan chan<- string) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	errorParam := query.Get("error")

	if errorParam != "" {
		slog.Error("OAuth2 callback error", "error", errorParam)
		fmt.Fprintf(w, "Authentication failed: %s. Please check the console for details.", errorParam)
		authCodeChan <- ""
		return
	}

	if state != "state" {
		slog.Error("State mismatch", "expected", "state", "got", state)
		fmt.Fprint(w, "Authentication failed: State mismatch.")
		authCodeChan <- ""
		return
	}

	if code == "" {
		slog.Error("No authorization code received in callback")
		fmt.Fprint(w, "Authentication failed: No code received.")
		authCodeChan <- ""
		return
	}

	slog.Info("Authorization code received successfully")
	fmt.Fprint(w, "Authentication successful! You can close this browser tab.")
	authCodeChan <- code
}

// OpenBrowser opens the given URL in the default web browser.
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// RefreshAccessToken uses the refresh token to obtain a new access token.
func RefreshAccessToken(cfg *config.Config, token *oauth2.Token) (*oauth2.Token, error) {
	if token == nil || token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oauthConfig := getOAuthConfig(cfg)
	tokenSource := oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get new token from refresh token: %w", err)
	}

	cfg.RedditOAuth.AccessToken = newToken.AccessToken
	cfg.RedditOAuth.RefreshToken = newToken.RefreshToken
	cfg.RedditOAuth.ExpiresAt = newToken.Expiry

	if err := config.SaveConfig(cfg, ""); err != nil {
		return nil, fmt.Errorf("failed to save updated config: %w", err)
	}

	slog.Info("Access token refreshed successfully")
	return newToken, nil
}

func getOAuthConfig(cfg *config.Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.RedditOAuth.ClientID,
		ClientSecret: cfg.RedditOAuth.ClientSecret,
		RedirectURL:  cfg.RedditOAuth.RedirectURI,
		Scopes:       []string{"identity", "read", "history"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.reddit.com/api/v1/authorize",
			TokenURL: "https://www.reddit.com/api/v1/access_token",
		},
	}
}
