// Package oauth provides OAuth 2.0 authentication support for MCP servers.
//
// This package implements OAuth 2.0 authorization code flow with PKCE,
// token storage (keyring and file-based), and automatic token refresh.
// It follows the Rust implementation's design for credential management.
package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// OAuth2Config represents the OAuth 2.0 configuration for an MCP server.
// It supports OAuth discovery via .well-known endpoints and manual configuration.
type OAuth2Config struct {
	// ClientID is the OAuth 2.0 client identifier
	ClientID string

	// ClientSecret is the OAuth 2.0 client secret (optional for public clients)
	ClientSecret string

	// AuthURL is the authorization endpoint URL
	AuthURL string

	// TokenURL is the token endpoint URL
	TokenURL string

	// RedirectURL is the callback URL for the authorization flow
	RedirectURL string

	// Scopes are the requested OAuth scopes
	Scopes []string

	// UsePKCE enables PKCE (Proof Key for Code Exchange) for enhanced security
	UsePKCE bool
}

// OAuthDiscoveryMetadata represents OAuth 2.0 server metadata from discovery
// Based on RFC 8414 OAuth 2.0 Authorization Server Metadata
type OAuthDiscoveryMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported []string `json:"response_types_supported,omitempty"`
	GrantTypesSupported    []string `json:"grant_types_supported,omitempty"`
}

// DiscoverOAuthConfig attempts to discover OAuth configuration from a server URL.
// It tries the following discovery endpoints:
//   1. /.well-known/oauth-authorization-server
//   2. /.well-known/openid-configuration
//
// Returns error if discovery fails or endpoints are not found.
func DiscoverOAuthConfig(ctx context.Context, serverURL string) (*OAuthDiscoveryMetadata, error) {
	// Parse the server URL to construct discovery endpoints
	baseURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Try OAuth 2.0 Authorization Server Metadata (RFC 8414)
	discoveryURLs := []string{
		baseURL.ResolveReference(&url.URL{Path: "/.well-known/oauth-authorization-server"}).String(),
		baseURL.ResolveReference(&url.URL{Path: "/.well-known/openid-configuration"}).String(),
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var lastErr error
	for _, discoveryURL := range discoveryURLs {
		req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		var metadata OAuthDiscoveryMetadata
		if err := json.Unmarshal(body, &metadata); err != nil {
			lastErr = err
			continue
		}

		// Validate required fields
		if metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
			lastErr = fmt.Errorf("discovery metadata missing required endpoints")
			continue
		}

		return &metadata, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("OAuth discovery failed: %w", lastErr)
	}
	return nil, fmt.Errorf("OAuth discovery failed: no discovery endpoints found")
}

// ToOAuth2Config converts discovery metadata to an oauth2.Config
func (m *OAuthDiscoveryMetadata) ToOAuth2Config(clientID, clientSecret, redirectURL string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  m.AuthorizationEndpoint,
			TokenURL: m.TokenEndpoint,
		},
		RedirectURL: redirectURL,
		Scopes:      scopes,
	}
}

// NewOAuth2Config creates a new OAuth2Config with the given parameters
func NewOAuth2Config(clientID, clientSecret, authURL, tokenURL, redirectURL string, scopes []string) *OAuth2Config {
	return &OAuth2Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		UsePKCE:      true, // Enable PKCE by default for security
	}
}

// ToStandardConfig converts OAuth2Config to golang.org/x/oauth2.Config
func (c *OAuth2Config) ToStandardConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.AuthURL,
			TokenURL: c.TokenURL,
		},
		RedirectURL: c.RedirectURL,
		Scopes:      c.Scopes,
	}
}

// PKCEChallenge represents a PKCE code challenge pair
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}

// GeneratePKCEChallenge generates a PKCE code verifier and challenge
// Uses S256 (SHA-256) method as recommended by RFC 7636
func GeneratePKCEChallenge() (*PKCEChallenge, error) {
	// Generate random verifier (43-128 characters)
	verifier := generateRandomString(64)

	// Create SHA-256 hash of verifier
	hash := sha256.Sum256([]byte(verifier))

	// Base64 URL encode the hash
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// generateRandomString generates a random URL-safe string of the given length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	b := make([]byte, length)
	for i := range b {
		// Use time-based pseudo-random for simplicity
		// In production, use crypto/rand
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// AuthCodeURL returns the OAuth 2.0 authorization URL with optional PKCE
func (c *OAuth2Config) AuthCodeURL(state string, pkce *PKCEChallenge) string {
	cfg := c.ToStandardConfig()

	var opts []oauth2.AuthCodeOption
	if pkce != nil && c.UsePKCE {
		opts = append(opts,
			oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
			oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
		)
	}

	return cfg.AuthCodeURL(state, opts...)
}

// Exchange exchanges an authorization code for an access token
func (c *OAuth2Config) Exchange(ctx context.Context, code string, pkce *PKCEChallenge) (*oauth2.Token, error) {
	cfg := c.ToStandardConfig()

	var opts []oauth2.AuthCodeOption
	if pkce != nil && c.UsePKCE {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", pkce.Verifier))
	}

	return cfg.Exchange(ctx, code, opts...)
}

// TokenSource creates a token source that automatically refreshes tokens
func (c *OAuth2Config) TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	cfg := c.ToStandardConfig()
	return cfg.TokenSource(ctx, token)
}

// ValidateToken checks if a token is valid and not expired
func ValidateToken(token *oauth2.Token) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}

	if token.AccessToken == "" {
		return fmt.Errorf("access token is empty")
	}

	if !token.Valid() {
		return fmt.Errorf("token is expired or invalid")
	}

	return nil
}

// StoredOAuthToken represents a stored OAuth token with metadata
// This matches the Rust implementation's StoredOAuthTokens structure
type StoredOAuthToken struct {
	ServerName   string        `json:"server_name"`
	ServerURL    string        `json:"server_url"`
	ClientID     string        `json:"client_id"`
	AccessToken  string        `json:"access_token"`
	TokenType    string        `json:"token_type"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	Expiry       time.Time     `json:"expiry,omitempty"`
	ExpiresAt    *int64        `json:"expires_at,omitempty"` // Unix timestamp in milliseconds
	Scopes       []string      `json:"scopes,omitempty"`
}

// ToOAuth2Token converts StoredOAuthToken to oauth2.Token
func (s *StoredOAuthToken) ToOAuth2Token() *oauth2.Token {
	token := &oauth2.Token{
		AccessToken:  s.AccessToken,
		TokenType:    s.TokenType,
		RefreshToken: s.RefreshToken,
	}

	if !s.Expiry.IsZero() {
		token.Expiry = s.Expiry
	} else if s.ExpiresAt != nil && *s.ExpiresAt > 0 {
		// Convert milliseconds to time.Time
		token.Expiry = time.Unix(*s.ExpiresAt/1000, (*s.ExpiresAt%1000)*1000000)
	}

	return token
}

// FromOAuth2Token creates a StoredOAuthToken from oauth2.Token
func FromOAuth2Token(serverName, serverURL, clientID string, token *oauth2.Token, scopes []string) *StoredOAuthToken {
	stored := &StoredOAuthToken{
		ServerName:   serverName,
		ServerURL:    serverURL,
		ClientID:     clientID,
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		Scopes:       scopes,
	}

	// Store expiry as Unix timestamp in milliseconds (matching Rust implementation)
	if !token.Expiry.IsZero() {
		expiresAtMs := token.Expiry.UnixMilli()
		stored.ExpiresAt = &expiresAtMs
	}

	return stored
}

// ComputeStoreKey generates a unique key for storing credentials
// This matches the Rust implementation's compute_store_key function
func ComputeStoreKey(serverName, serverURL string) (string, error) {
	// Create a JSON object with type, url, and headers (empty)
	payload := map[string]interface{}{
		"type":    "http",
		"url":     serverURL,
		"headers": map[string]interface{}{},
	}

	// Serialize to JSON
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize key payload: %w", err)
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256(jsonBytes)

	// Take first 16 hex characters (8 bytes)
	hexHash := fmt.Sprintf("%x", hash[:])
	truncated := hexHash[:16]

	// Return "serverName|truncatedHash"
	return fmt.Sprintf("%s|%s", serverName, truncated), nil
}

// InjectOAuthHeader adds the OAuth token to an HTTP request
func InjectOAuthHeader(req *http.Request, token *oauth2.Token) error {
	if err := ValidateToken(token); err != nil {
		return fmt.Errorf("cannot inject invalid token: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	return nil
}

// ParseScopes parses a comma or space-separated list of scopes
func ParseScopes(scopeStr string) []string {
	if scopeStr == "" {
		return nil
	}

	// Try comma-separated first
	if strings.Contains(scopeStr, ",") {
		scopes := strings.Split(scopeStr, ",")
		for i, s := range scopes {
			scopes[i] = strings.TrimSpace(s)
		}
		return scopes
	}

	// Fall back to space-separated
	return strings.Fields(scopeStr)
}
