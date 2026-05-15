package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// TokenStore defines the interface for storing and retrieving OAuth tokens.
// Implementations can use keyring, file system, or other storage backends.
type TokenStore interface {
	// Load retrieves a stored token for the given server
	Load(serverName, serverURL string) (*StoredOAuthToken, error)

	// Save stores a token for the given server
	Save(token *StoredOAuthToken) error

	// Delete removes a stored token for the given server
	Delete(serverName, serverURL string) error

	// Has checks if a token exists for the given server
	Has(serverName, serverURL string) (bool, error)
}

// FileTokenStore implements TokenStore using file-based storage.
// Tokens are stored in ~/.codex/.credentials.json matching the Rust implementation.
// This provides a fallback when OS keyring is not available.
type FileTokenStore struct {
	filePath string
	mu       sync.RWMutex
}

// credentialsFile represents the structure of the credentials file
// Maps store keys to credential entries
type credentialsFile map[string]StoredOAuthToken

// NewFileTokenStore creates a new file-based token store
// If codexHome is empty, it defaults to ~/.codex
func NewFileTokenStore(codexHome string) (*FileTokenStore, error) {
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		codexHome = filepath.Join(home, ".codex")
	}

	// Ensure the directory exists
	if err := os.MkdirAll(codexHome, 0700); err != nil {
		return nil, fmt.Errorf("failed to create codex home directory: %w", err)
	}

	filePath := filepath.Join(codexHome, ".credentials.json")

	return &FileTokenStore{
		filePath: filePath,
	}, nil
}

// Load retrieves a stored token for the given server
func (f *FileTokenStore) Load(serverName, serverURL string) (*StoredOAuthToken, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	key, err := ComputeStoreKey(serverName, serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compute store key: %w", err)
	}

	creds, err := f.readFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No credentials file yet
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Look for matching entry by key
	if token, ok := creds[key]; ok {
		return &token, nil
	}

	// Also check by matching server name and URL for backwards compatibility
	for _, token := range creds {
		if token.ServerName == serverName && token.ServerURL == serverURL {
			return &token, nil
		}
	}

	return nil, nil // Not found
}

// Save stores a token for the given server
func (f *FileTokenStore) Save(token *StoredOAuthToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key, err := ComputeStoreKey(token.ServerName, token.ServerURL)
	if err != nil {
		return fmt.Errorf("failed to compute store key: %w", err)
	}

	creds, err := f.readFile()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	if creds == nil {
		creds = make(credentialsFile)
	}

	creds[key] = *token

	if err := f.writeFile(creds); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// Delete removes a stored token for the given server
func (f *FileTokenStore) Delete(serverName, serverURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key, err := ComputeStoreKey(serverName, serverURL)
	if err != nil {
		return fmt.Errorf("failed to compute store key: %w", err)
	}

	creds, err := f.readFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to delete
		}
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	delete(creds, key)

	// If no credentials remain, delete the file
	if len(creds) == 0 {
		if err := os.Remove(f.filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove credentials file: %w", err)
		}
		return nil
	}

	if err := f.writeFile(creds); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// Has checks if a token exists for the given server
func (f *FileTokenStore) Has(serverName, serverURL string) (bool, error) {
	token, err := f.Load(serverName, serverURL)
	if err != nil {
		return false, err
	}
	return token != nil, nil
}

// readFile reads and parses the credentials file
func (f *FileTokenStore) readFile() (credentialsFile, error) {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return nil, err
	}

	var creds credentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	return creds, nil
}

// writeFile writes the credentials to disk with secure permissions
func (f *FileTokenStore) writeFile(creds credentialsFile) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize credentials: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(f.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// GetFilePath returns the path to the credentials file
func (f *FileTokenStore) GetFilePath() string {
	return f.filePath
}

// MemoryTokenStore implements TokenStore using in-memory storage.
// Useful for testing and temporary token storage.
type MemoryTokenStore struct {
	tokens map[string]*StoredOAuthToken
	mu     sync.RWMutex
}

// NewMemoryTokenStore creates a new in-memory token store
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		tokens: make(map[string]*StoredOAuthToken),
	}
}

// Load retrieves a stored token for the given server
func (m *MemoryTokenStore) Load(serverName, serverURL string) (*StoredOAuthToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, err := ComputeStoreKey(serverName, serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compute store key: %w", err)
	}

	if token, ok := m.tokens[key]; ok {
		return token, nil
	}

	return nil, nil
}

// Save stores a token for the given server
func (m *MemoryTokenStore) Save(token *StoredOAuthToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, err := ComputeStoreKey(token.ServerName, token.ServerURL)
	if err != nil {
		return fmt.Errorf("failed to compute store key: %w", err)
	}

	m.tokens[key] = token
	return nil
}

// Delete removes a stored token for the given server
func (m *MemoryTokenStore) Delete(serverName, serverURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, err := ComputeStoreKey(serverName, serverURL)
	if err != nil {
		return fmt.Errorf("failed to compute store key: %w", err)
	}

	delete(m.tokens, key)
	return nil
}

// Has checks if a token exists for the given server
func (m *MemoryTokenStore) Has(serverName, serverURL string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, err := ComputeStoreKey(serverName, serverURL)
	if err != nil {
		return false, fmt.Errorf("failed to compute store key: %w", err)
	}

	_, ok := m.tokens[key]
	return ok, nil
}

// Clear removes all stored tokens
func (m *MemoryTokenStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = make(map[string]*StoredOAuthToken)
}

// TokenManager manages OAuth tokens with automatic refresh
type TokenManager struct {
	store  TokenStore
	config *OAuth2Config
}

// NewTokenManager creates a new token manager
func NewTokenManager(store TokenStore, config *OAuth2Config) *TokenManager {
	return &TokenManager{
		store:  store,
		config: config,
	}
}

// GetToken retrieves a valid token, refreshing if necessary
func (tm *TokenManager) GetToken(serverName, serverURL string) (*oauth2.Token, error) {
	stored, err := tm.store.Load(serverName, serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	if stored == nil {
		return nil, fmt.Errorf("no token found for server %s", serverName)
	}

	token := stored.ToOAuth2Token()

	// Check if token is still valid
	if token.Valid() {
		return token, nil
	}

	// Token is expired, try to refresh
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("token expired and no refresh token available")
	}

	// Refresh the token
	tokenSource := tm.config.TokenSource(nil, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the refreshed token
	newStored := FromOAuth2Token(serverName, serverURL, stored.ClientID, newToken, stored.Scopes)
	if err := tm.store.Save(newStored); err != nil {
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return newToken, nil
}

// SaveToken stores a new token
func (tm *TokenManager) SaveToken(serverName, serverURL, clientID string, token *oauth2.Token, scopes []string) error {
	stored := FromOAuth2Token(serverName, serverURL, clientID, token, scopes)
	return tm.store.Save(stored)
}

// DeleteToken removes a stored token
func (tm *TokenManager) DeleteToken(serverName, serverURL string) error {
	return tm.store.Delete(serverName, serverURL)
}

// HasToken checks if a token exists
func (tm *TokenManager) HasToken(serverName, serverURL string) (bool, error) {
	return tm.store.Has(serverName, serverURL)
}

// ValidateFilePermissions checks if the credentials file has secure permissions
func ValidateFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet
		}
		return fmt.Errorf("failed to stat credentials file: %w", err)
	}

	mode := info.Mode()

	// Check if file is readable by group or others (beyond owner)
	if mode.Perm()&0077 != 0 {
		return fmt.Errorf("credentials file has insecure permissions %v (should be 0600)", mode.Perm())
	}

	return nil
}

// SecureCredentialsFile ensures the credentials file has secure permissions
func SecureCredentialsFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet
		}
		return fmt.Errorf("failed to stat credentials file: %w", err)
	}

	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set secure permissions on credentials file: %w", err)
	}

	return nil
}
