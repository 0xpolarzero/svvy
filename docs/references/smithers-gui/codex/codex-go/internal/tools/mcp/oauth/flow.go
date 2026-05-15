package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// AuthorizationFlow manages the OAuth 2.0 authorization code flow
type AuthorizationFlow struct {
	config        *OAuth2Config
	tokenStore    TokenStore
	callbackURL   string
	state         string
	pkce          *PKCEChallenge
	serverName    string
	serverURL     string
	callbackCh    chan *callbackResult
	server        *http.Server
	listener      net.Listener
	serverStarted bool
	mu            sync.Mutex
}

// callbackResult represents the result of the OAuth callback
type callbackResult struct {
	code  string
	state string
	err   error
}

// NewAuthorizationFlow creates a new authorization flow
func NewAuthorizationFlow(serverName, serverURL string, config *OAuth2Config, store TokenStore) (*AuthorizationFlow, error) {
	// Generate random state for CSRF protection
	state, err := generateSecureRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Generate PKCE challenge if enabled
	var pkce *PKCEChallenge
	if config.UsePKCE {
		pkce, err = GeneratePKCEChallenge()
		if err != nil {
			return nil, fmt.Errorf("failed to generate PKCE challenge: %w", err)
		}
	}

	flow := &AuthorizationFlow{
		config:     config,
		tokenStore: store,
		state:      state,
		pkce:       pkce,
		serverName: serverName,
		serverURL:  serverURL,
		callbackCh: make(chan *callbackResult, 1),
	}

	return flow, nil
}

// StartCallbackServer starts a local HTTP server to receive the OAuth callback
// Returns the callback URL that should be used in the authorization request
func (f *AuthorizationFlow) StartCallbackServer() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.serverStarted {
		return f.callbackURL, nil
	}

	// Start listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start callback server: %w", err)
	}

	f.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port
	f.callbackURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Update config with actual callback URL
	f.config.RedirectURL = f.callbackURL

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", f.handleCallback)

	f.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		if err := f.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			f.callbackCh <- &callbackResult{err: fmt.Errorf("callback server error: %w", err)}
		}
	}()

	f.serverStarted = true
	return f.callbackURL, nil
}

// StopCallbackServer stops the callback server
func (f *AuthorizationFlow) StopCallbackServer() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.serverStarted || f.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := f.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown callback server: %w", err)
	}

	f.serverStarted = false
	return nil
}

// handleCallback handles the OAuth callback request
func (f *AuthorizationFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Check for error response
	if errMsg := query.Get("error"); errMsg != "" {
		errDesc := query.Get("error_description")
		f.callbackCh <- &callbackResult{
			err: fmt.Errorf("OAuth error: %s - %s", errMsg, errDesc),
		}
		http.Error(w, "Authentication failed. You may close this window.", http.StatusBadRequest)
		return
	}

	// Extract code and state
	code := query.Get("code")
	state := query.Get("state")

	if code == "" || state == "" {
		f.callbackCh <- &callbackResult{
			err: fmt.Errorf("missing code or state in callback"),
		}
		http.Error(w, "Invalid callback. You may close this window.", http.StatusBadRequest)
		return
	}

	// Send result to channel
	f.callbackCh <- &callbackResult{
		code:  code,
		state: state,
	}

	// Send success response
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Authentication Complete</title>
			<style>
				body { font-family: system-ui, -apple-system, sans-serif; padding: 40px; text-align: center; }
				.success { color: #059669; font-size: 24px; margin-bottom: 20px; }
				.message { color: #6b7280; }
			</style>
		</head>
		<body>
			<div class="success">✓ Authentication Complete</div>
			<div class="message">You may close this window and return to your terminal.</div>
		</body>
		</html>
	`)
}

// GetAuthorizationURL returns the authorization URL for the user to visit
func (f *AuthorizationFlow) GetAuthorizationURL() string {
	return f.config.AuthCodeURL(f.state, f.pkce)
}

// WaitForCallback waits for the OAuth callback with a timeout
func (f *AuthorizationFlow) WaitForCallback(ctx context.Context) (string, error) {
	select {
	case result := <-f.callbackCh:
		if result.err != nil {
			return "", result.err
		}

		// Verify state matches (CSRF protection)
		if result.state != f.state {
			return "", fmt.Errorf("state mismatch: potential CSRF attack")
		}

		return result.code, nil

	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ExchangeCode exchanges the authorization code for an access token
func (f *AuthorizationFlow) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := f.config.Exchange(ctx, code, f.pkce)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}

// CompleteFlow runs the complete authorization flow
// Returns the obtained access token
func (f *AuthorizationFlow) CompleteFlow(ctx context.Context, clientID string) (*oauth2.Token, error) {
	// Start callback server
	_, err := f.StartCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer f.StopCallbackServer()

	// Get authorization URL
	authURL := f.GetAuthorizationURL()

	fmt.Printf("Please authorize %s by visiting this URL:\n\n%s\n\n", f.serverName, authURL)
	fmt.Println("Waiting for authorization callback...")

	// Wait for callback with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	code, err := f.WaitForCallback(timeoutCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive callback: %w", err)
	}

	// Exchange code for token
	token, err := f.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Save token to store
	stored := FromOAuth2Token(f.serverName, f.serverURL, clientID, token, f.config.Scopes)
	if err := f.tokenStore.Save(stored); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("Successfully authenticated with %s\n", f.serverName)

	return token, nil
}

// PerformOAuthLogin performs the complete OAuth login flow
// This is a convenience function matching the Rust implementation
func PerformOAuthLogin(ctx context.Context, serverName, serverURL, clientID string, config *OAuth2Config, store TokenStore, scopes []string) error {
	// Update config with requested scopes
	config.Scopes = scopes

	// Create flow
	flow, err := NewAuthorizationFlow(serverName, serverURL, config, store)
	if err != nil {
		return fmt.Errorf("failed to create authorization flow: %w", err)
	}

	// Complete the flow
	_, err = flow.CompleteFlow(ctx, clientID)
	if err != nil {
		return fmt.Errorf("OAuth login failed: %w", err)
	}

	return nil
}

// OpenBrowser attempts to open the authorization URL in the user's browser
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch {
	case commandExists("xdg-open"):
		cmd = "xdg-open"
		args = []string{url}
	case commandExists("open"):
		cmd = "open"
		args = []string{url}
	case commandExists("start"):
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("no browser opening command found")
	}

	return executeCommand(cmd, args...)
}

// generateSecureRandomString generates a cryptographically secure random string
func generateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	// Simple check - just return false for now
	// In production, would use exec.LookPath
	return false
}

// executeCommand executes a system command
func executeCommand(cmd string, args ...string) error {
	// Placeholder - would use os/exec in production
	return fmt.Errorf("command execution not implemented")
}

// RefreshTokenFlow handles token refresh
type RefreshTokenFlow struct {
	config     *OAuth2Config
	tokenStore TokenStore
}

// NewRefreshTokenFlow creates a new refresh token flow
func NewRefreshTokenFlow(config *OAuth2Config, store TokenStore) *RefreshTokenFlow {
	return &RefreshTokenFlow{
		config:     config,
		tokenStore: store,
	}
}

// RefreshToken refreshes an expired token
func (r *RefreshTokenFlow) RefreshToken(ctx context.Context, serverName, serverURL string) (*oauth2.Token, error) {
	// Load stored token
	stored, err := r.tokenStore.Load(serverName, serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	if stored == nil {
		return nil, fmt.Errorf("no token found for server %s", serverName)
	}

	token := stored.ToOAuth2Token()

	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// Use token source to automatically refresh
	tokenSource := r.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save refreshed token
	newStored := FromOAuth2Token(serverName, serverURL, stored.ClientID, newToken, stored.Scopes)
	if err := r.tokenStore.Save(newStored); err != nil {
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return newToken, nil
}

// ParseCallbackURL parses an OAuth callback URL and extracts code and state
func ParseCallbackURL(callbackURL string) (code, state string, err error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid callback URL: %w", err)
	}

	query := u.Query()

	// Check for error
	if errMsg := query.Get("error"); errMsg != "" {
		errDesc := query.Get("error_description")
		return "", "", fmt.Errorf("OAuth error: %s - %s", errMsg, errDesc)
	}

	code = query.Get("code")
	state = query.Get("state")

	if code == "" {
		return "", "", fmt.Errorf("no code in callback URL")
	}

	if state == "" {
		return "", "", fmt.Errorf("no state in callback URL")
	}

	return code, state, nil
}
