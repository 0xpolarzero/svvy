package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// TestOAuthDiscovery tests OAuth discovery functionality
func TestOAuthDiscovery(t *testing.T) {
	t.Run("successful discovery", func(t *testing.T) {
		// Create mock OAuth server
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/oauth-authorization-server" {
				metadata := OAuthDiscoveryMetadata{
					Issuer:                srv.URL,
					AuthorizationEndpoint: srv.URL + "/oauth/authorize",
					TokenEndpoint:         srv.URL + "/oauth/token",
					ScopesSupported:       []string{"read", "write"},
					ResponseTypesSupported: []string{"code"},
					GrantTypesSupported:   []string{"authorization_code", "refresh_token"},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(metadata)
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		ctx := context.Background()
		metadata, err := DiscoverOAuthConfig(ctx, srv.URL)
		require.NoError(t, err)
		require.NotNil(t, metadata)

		assert.Equal(t, metadata.Issuer, srv.URL)
		assert.Equal(t, metadata.AuthorizationEndpoint, srv.URL+"/oauth/authorize")
		assert.Equal(t, metadata.TokenEndpoint, srv.URL+"/oauth/token")
		assert.Contains(t, metadata.ScopesSupported, "read")
		assert.Contains(t, metadata.ScopesSupported, "write")
	})

	t.Run("fallback to openid-configuration", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				metadata := OAuthDiscoveryMetadata{
					Issuer:                srv.URL,
					AuthorizationEndpoint: srv.URL + "/auth",
					TokenEndpoint:         srv.URL + "/token",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(metadata)
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		ctx := context.Background()
		metadata, err := DiscoverOAuthConfig(ctx, srv.URL)
		require.NoError(t, err)
		require.NotNil(t, metadata)

		assert.Equal(t, metadata.AuthorizationEndpoint, srv.URL+"/auth")
		assert.Equal(t, metadata.TokenEndpoint, srv.URL+"/token")
	})

	t.Run("discovery failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := DiscoverOAuthConfig(ctx, server.URL)
		assert.Error(t, err)
	})

	t.Run("invalid server URL", func(t *testing.T) {
		ctx := context.Background()
		_, err := DiscoverOAuthConfig(ctx, "://invalid-url")
		assert.Error(t, err)
	})
}

// TestOAuth2Config tests OAuth2Config functionality
func TestOAuth2Config(t *testing.T) {
	t.Run("create and convert config", func(t *testing.T) {
		config := NewOAuth2Config(
			"test-client-id",
			"test-client-secret",
			"https://example.com/oauth/authorize",
			"https://example.com/oauth/token",
			"http://localhost:8080/callback",
			[]string{"read", "write"},
		)

		assert.Equal(t, "test-client-id", config.ClientID)
		assert.Equal(t, "test-client-secret", config.ClientSecret)
		assert.True(t, config.UsePKCE)
		assert.Len(t, config.Scopes, 2)

		stdConfig := config.ToStandardConfig()
		assert.Equal(t, config.ClientID, stdConfig.ClientID)
		assert.Equal(t, config.ClientSecret, stdConfig.ClientSecret)
		assert.Equal(t, config.AuthURL, stdConfig.Endpoint.AuthURL)
		assert.Equal(t, config.TokenURL, stdConfig.Endpoint.TokenURL)
	})

	t.Run("auth code URL generation", func(t *testing.T) {
		config := NewOAuth2Config(
			"test-client-id",
			"",
			"https://example.com/oauth/authorize",
			"https://example.com/oauth/token",
			"http://localhost:8080/callback",
			[]string{"read"},
		)

		pkce, err := GeneratePKCEChallenge()
		require.NoError(t, err)

		authURL := config.AuthCodeURL("test-state", pkce)
		assert.Contains(t, authURL, "https://example.com/oauth/authorize")
		assert.Contains(t, authURL, "client_id=test-client-id")
		assert.Contains(t, authURL, "state=test-state")
		assert.Contains(t, authURL, "code_challenge=")
		assert.Contains(t, authURL, "code_challenge_method=S256")
	})
}

// TestPKCE tests PKCE challenge generation
func TestPKCE(t *testing.T) {
	t.Run("generate PKCE challenge", func(t *testing.T) {
		pkce, err := GeneratePKCEChallenge()
		require.NoError(t, err)
		require.NotNil(t, pkce)

		assert.NotEmpty(t, pkce.Verifier)
		assert.NotEmpty(t, pkce.Challenge)
		assert.Equal(t, "S256", pkce.Method)
		assert.Len(t, pkce.Verifier, 64)
	})

	t.Run("PKCE challenges are unique", func(t *testing.T) {
		pkce1, err := GeneratePKCEChallenge()
		require.NoError(t, err)

		pkce2, err := GeneratePKCEChallenge()
		require.NoError(t, err)

		assert.NotEqual(t, pkce1.Verifier, pkce2.Verifier)
		assert.NotEqual(t, pkce1.Challenge, pkce2.Challenge)
	})
}

// TestTokenStore tests token storage implementations
func TestFileTokenStore(t *testing.T) {
	t.Run("save and load token", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileTokenStore(tempDir)
		require.NoError(t, err)

		token := &StoredOAuthToken{
			ServerName:   "test-server",
			ServerURL:    "https://test.example.com",
			ClientID:     "test-client",
			AccessToken:  "test-access-token",
			TokenType:    "Bearer",
			RefreshToken: "test-refresh-token",
			Expiry:       time.Now().Add(1 * time.Hour),
			Scopes:       []string{"read", "write"},
		}

		// Save token
		err = store.Save(token)
		require.NoError(t, err)

		// Verify file exists with secure permissions
		credPath := store.GetFilePath()
		info, err := os.Stat(credPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Load token
		loaded, err := store.Load("test-server", "https://test.example.com")
		require.NoError(t, err)
		require.NotNil(t, loaded)

		assert.Equal(t, token.ServerName, loaded.ServerName)
		assert.Equal(t, token.ServerURL, loaded.ServerURL)
		assert.Equal(t, token.ClientID, loaded.ClientID)
		assert.Equal(t, token.AccessToken, loaded.AccessToken)
		assert.Equal(t, token.RefreshToken, loaded.RefreshToken)
	})

	t.Run("delete token", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileTokenStore(tempDir)
		require.NoError(t, err)

		token := &StoredOAuthToken{
			ServerName:  "test-server",
			ServerURL:   "https://test.example.com",
			ClientID:    "test-client",
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
		}

		// Save and delete
		err = store.Save(token)
		require.NoError(t, err)

		err = store.Delete("test-server", "https://test.example.com")
		require.NoError(t, err)

		// Verify deleted
		loaded, err := store.Load("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.Nil(t, loaded)
	})

	t.Run("has token", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileTokenStore(tempDir)
		require.NoError(t, err)

		// Should not have token initially
		has, err := store.Has("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.False(t, has)

		// Save token
		token := &StoredOAuthToken{
			ServerName:  "test-server",
			ServerURL:   "https://test.example.com",
			ClientID:    "test-client",
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
		}
		err = store.Save(token)
		require.NoError(t, err)

		// Should have token now
		has, err = store.Has("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("multiple tokens", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := NewFileTokenStore(tempDir)
		require.NoError(t, err)

		// Save multiple tokens
		token1 := &StoredOAuthToken{
			ServerName:  "server1",
			ServerURL:   "https://server1.example.com",
			ClientID:    "client1",
			AccessToken: "token1",
			TokenType:   "Bearer",
		}
		token2 := &StoredOAuthToken{
			ServerName:  "server2",
			ServerURL:   "https://server2.example.com",
			ClientID:    "client2",
			AccessToken: "token2",
			TokenType:   "Bearer",
		}

		err = store.Save(token1)
		require.NoError(t, err)
		err = store.Save(token2)
		require.NoError(t, err)

		// Load each token
		loaded1, err := store.Load("server1", "https://server1.example.com")
		require.NoError(t, err)
		assert.Equal(t, "token1", loaded1.AccessToken)

		loaded2, err := store.Load("server2", "https://server2.example.com")
		require.NoError(t, err)
		assert.Equal(t, "token2", loaded2.AccessToken)
	})
}

// TestMemoryTokenStore tests in-memory token storage
func TestMemoryTokenStore(t *testing.T) {
	t.Run("save and load", func(t *testing.T) {
		store := NewMemoryTokenStore()

		token := &StoredOAuthToken{
			ServerName:  "test-server",
			ServerURL:   "https://test.example.com",
			ClientID:    "test-client",
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
		}

		err := store.Save(token)
		require.NoError(t, err)

		loaded, err := store.Load("test-server", "https://test.example.com")
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, token.AccessToken, loaded.AccessToken)
	})

	t.Run("clear", func(t *testing.T) {
		store := NewMemoryTokenStore()

		token := &StoredOAuthToken{
			ServerName:  "test-server",
			ServerURL:   "https://test.example.com",
			ClientID:    "test-client",
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
		}

		err := store.Save(token)
		require.NoError(t, err)

		store.Clear()

		loaded, err := store.Load("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.Nil(t, loaded)
	})
}

// TestTokenConversion tests token conversion between formats
func TestTokenConversion(t *testing.T) {
	t.Run("StoredOAuthToken to oauth2.Token", func(t *testing.T) {
		expiry := time.Now().Add(1 * time.Hour)
		expiryMs := expiry.UnixMilli()

		stored := &StoredOAuthToken{
			ServerName:   "test-server",
			ServerURL:    "https://test.example.com",
			ClientID:     "test-client",
			AccessToken:  "test-access-token",
			TokenType:    "Bearer",
			RefreshToken: "test-refresh-token",
			ExpiresAt:    &expiryMs,
			Scopes:       []string{"read", "write"},
		}

		token := stored.ToOAuth2Token()
		require.NotNil(t, token)

		assert.Equal(t, stored.AccessToken, token.AccessToken)
		assert.Equal(t, stored.TokenType, token.TokenType)
		assert.Equal(t, stored.RefreshToken, token.RefreshToken)
		assert.WithinDuration(t, expiry, token.Expiry, time.Second)
	})

	t.Run("oauth2.Token to StoredOAuthToken", func(t *testing.T) {
		expiry := time.Now().Add(1 * time.Hour)
		token := &oauth2.Token{
			AccessToken:  "test-access-token",
			TokenType:    "Bearer",
			RefreshToken: "test-refresh-token",
			Expiry:       expiry,
		}
		scopes := []string{"read", "write"}

		stored := FromOAuth2Token("test-server", "https://test.example.com", "test-client", token, scopes)
		require.NotNil(t, stored)

		assert.Equal(t, "test-server", stored.ServerName)
		assert.Equal(t, "https://test.example.com", stored.ServerURL)
		assert.Equal(t, "test-client", stored.ClientID)
		assert.Equal(t, token.AccessToken, stored.AccessToken)
		assert.Equal(t, token.TokenType, stored.TokenType)
		assert.Equal(t, token.RefreshToken, stored.RefreshToken)
		assert.Equal(t, scopes, stored.Scopes)
		assert.NotNil(t, stored.ExpiresAt)
	})
}

// TestComputeStoreKey tests the store key computation
func TestComputeStoreKey(t *testing.T) {
	t.Run("compute key", func(t *testing.T) {
		key, err := ComputeStoreKey("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, key)
		assert.Contains(t, key, "test-server|")
		assert.Len(t, key, len("test-server|")+16) // server name + pipe + 16 hex chars
	})

	t.Run("same inputs produce same key", func(t *testing.T) {
		key1, err := ComputeStoreKey("test-server", "https://test.example.com")
		require.NoError(t, err)

		key2, err := ComputeStoreKey("test-server", "https://test.example.com")
		require.NoError(t, err)

		assert.Equal(t, key1, key2)
	})

	t.Run("different URLs produce different keys", func(t *testing.T) {
		key1, err := ComputeStoreKey("test-server", "https://test1.example.com")
		require.NoError(t, err)

		key2, err := ComputeStoreKey("test-server", "https://test2.example.com")
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2)
	})
}

// TestTokenValidation tests token validation
func TestTokenValidation(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		}

		err := ValidateToken(token)
		assert.NoError(t, err)
	})

	t.Run("nil token", func(t *testing.T) {
		err := ValidateToken(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("empty access token", func(t *testing.T) {
		token := &oauth2.Token{
			TokenType: "Bearer",
			Expiry:    time.Now().Add(1 * time.Hour),
		}

		err := ValidateToken(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("expired token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(-1 * time.Hour),
		}

		err := ValidateToken(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

// TestInjectOAuthHeader tests OAuth header injection
func TestInjectOAuthHeader(t *testing.T) {
	t.Run("inject valid token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		}

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = InjectOAuthHeader(req, token)
		require.NoError(t, err)

		assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
	})

	t.Run("reject invalid token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(-1 * time.Hour),
		}

		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)

		err = InjectOAuthHeader(req, token)
		assert.Error(t, err)
	})
}

// TestParseScopes tests scope parsing
func TestParseScopes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "comma-separated",
			input:    "read,write,admin",
			expected: []string{"read", "write", "admin"},
		},
		{
			name:     "space-separated",
			input:    "read write admin",
			expected: []string{"read", "write", "admin"},
		},
		{
			name:     "comma with spaces",
			input:    "read, write, admin",
			expected: []string{"read", "write", "admin"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single scope",
			input:    "read",
			expected: []string{"read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseScopes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFilePermissions tests file permission handling
func TestFilePermissions(t *testing.T) {
	t.Run("validate secure permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")

		// Create file with secure permissions
		err := os.WriteFile(filePath, []byte("test"), 0600)
		require.NoError(t, err)

		err = ValidateFilePermissions(filePath)
		assert.NoError(t, err)
	})

	t.Run("detect insecure permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")

		// Create file with insecure permissions
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)

		err = ValidateFilePermissions(filePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insecure")
	})

	t.Run("secure file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")

		// Create file with insecure permissions
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)

		// Secure it
		err = SecureCredentialsFile(filePath)
		require.NoError(t, err)

		// Verify it's now secure
		err = ValidateFilePermissions(filePath)
		assert.NoError(t, err)
	})
}

// TestTokenManager tests the TokenManager functionality
func TestTokenManager(t *testing.T) {
	t.Run("get valid token", func(t *testing.T) {
		store := NewMemoryTokenStore()
		config := NewOAuth2Config(
			"test-client",
			"test-secret",
			"https://example.com/auth",
			"https://example.com/token",
			"http://localhost:8080/callback",
			[]string{"read"},
		)
		manager := NewTokenManager(store, config)

		// Save a valid token
		token := &oauth2.Token{
			AccessToken:  "test-token",
			TokenType:    "Bearer",
			RefreshToken: "refresh-token",
			Expiry:       time.Now().Add(1 * time.Hour),
		}
		err := manager.SaveToken("test-server", "https://test.example.com", "test-client", token, []string{"read"})
		require.NoError(t, err)

		// Get the token
		retrieved, err := manager.GetToken("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.Equal(t, token.AccessToken, retrieved.AccessToken)
	})

	t.Run("has token", func(t *testing.T) {
		store := NewMemoryTokenStore()
		config := NewOAuth2Config(
			"test-client",
			"test-secret",
			"https://example.com/auth",
			"https://example.com/token",
			"http://localhost:8080/callback",
			[]string{"read"},
		)
		manager := NewTokenManager(store, config)

		has, err := manager.HasToken("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.False(t, has)

		// Save a token
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		}
		err = manager.SaveToken("test-server", "https://test.example.com", "test-client", token, []string{"read"})
		require.NoError(t, err)

		has, err = manager.HasToken("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("delete token", func(t *testing.T) {
		store := NewMemoryTokenStore()
		config := NewOAuth2Config(
			"test-client",
			"test-secret",
			"https://example.com/auth",
			"https://example.com/token",
			"http://localhost:8080/callback",
			[]string{"read"},
		)
		manager := NewTokenManager(store, config)

		// Save and delete
		token := &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		}
		err := manager.SaveToken("test-server", "https://test.example.com", "test-client", token, []string{"read"})
		require.NoError(t, err)

		err = manager.DeleteToken("test-server", "https://test.example.com")
		require.NoError(t, err)

		has, err := manager.HasToken("test-server", "https://test.example.com")
		require.NoError(t, err)
		assert.False(t, has)
	})
}

// TestParseCallbackURL tests OAuth callback URL parsing
func TestParseCallbackURL(t *testing.T) {
	t.Run("successful parse", func(t *testing.T) {
		callbackURL := "http://localhost:8080/callback?code=test-code&state=test-state"

		code, state, err := ParseCallbackURL(callbackURL)
		require.NoError(t, err)
		assert.Equal(t, "test-code", code)
		assert.Equal(t, "test-state", state)
	})

	t.Run("error in callback", func(t *testing.T) {
		callbackURL := "http://localhost:8080/callback?error=access_denied&error_description=User+denied+access"

		_, _, err := ParseCallbackURL(callbackURL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access_denied")
	})

	t.Run("missing code", func(t *testing.T) {
		callbackURL := "http://localhost:8080/callback?state=test-state"

		_, _, err := ParseCallbackURL(callbackURL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no code")
	})

	t.Run("invalid URL", func(t *testing.T) {
		callbackURL := "://invalid"

		_, _, err := ParseCallbackURL(callbackURL)
		assert.Error(t, err)
	})
}
