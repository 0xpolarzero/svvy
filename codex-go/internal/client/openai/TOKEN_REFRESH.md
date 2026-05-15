# Token Refresh Feature

## Overview

The OpenAI client now supports automatic token refresh when receiving 401 Unauthorized responses from the API. This feature allows the client to automatically obtain a new token and retry the request, providing seamless authentication without manual intervention.

## How It Works

1. When a request receives a **401 Unauthorized** response
2. If a `TokenRefreshFunc` is configured, the client calls it with the current token
3. The refresh function returns a new token or an error
4. If successful, the client updates its internal token and retries the original request
5. Only **one retry attempt** is made per request to prevent infinite loops

## Configuration

To enable token refresh, provide a `TokenRefreshFunc` when creating the client:

```go
import (
    "context"
    "fmt"
    "github.com/evmts/codex/codex-go/internal/client"
    "github.com/evmts/codex/codex-go/internal/client/openai"
)

cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  "initial-token",
    Model:   "gpt-4",
    TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
        // Call your token refresh service
        newToken, err := refreshTokenService.Refresh(ctx, oldToken)
        if err != nil {
            return "", fmt.Errorf("failed to refresh token: %w", err)
        }
        return newToken, nil
    },
}

client, err := openai.NewClient(cfg)
if err != nil {
    // handle error
}
```

## Token Refresh Function

The `TokenRefreshFunc` has the following signature:

```go
type TokenRefreshFunc func(ctx context.Context, oldToken string) (newToken string, err error)
```

### Parameters
- `ctx`: Context for the refresh operation (can be used for timeouts)
- `oldToken`: The current token that was rejected by the API

### Returns
- `newToken`: The refreshed token to use for subsequent requests
- `err`: Error if refresh failed

### Best Practices

1. **Implement proper timeout handling** using the context
2. **Return meaningful errors** to help diagnose refresh failures
3. **Secure token storage** - don't log tokens or store them insecurely
4. **Handle refresh service failures** gracefully

## Example: OAuth2 Token Refresh

```go
import (
    "context"
    "golang.org/x/oauth2"
)

func createRefreshFunc(tokenSource oauth2.TokenSource) client.TokenRefreshFunc {
    return func(ctx context.Context, oldToken string) (string, error) {
        // Get fresh token from OAuth2 token source
        token, err := tokenSource.Token()
        if err != nil {
            return "", fmt.Errorf("oauth2 token refresh failed: %w", err)
        }
        return token.AccessToken, nil
    }
}

cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  initialToken.AccessToken,
    Model:   "gpt-4",
    TokenRefreshFunc: createRefreshFunc(tokenSource),
}
```

## Error Handling

### Without Token Refresh

When `TokenRefreshFunc` is not configured, 401 responses return an `UnauthorizedError`:

```go
resp, err := client.Complete(ctx, req)
if err != nil {
    var authErr *client.UnauthorizedError
    if errors.As(err, &authErr) {
        fmt.Printf("Authorization failed: %s\n", authErr.Message)
        fmt.Printf("Can refresh: %v\n", authErr.CanRefresh) // false
    }
}
```

### With Token Refresh

If token refresh fails, the error includes details:

```go
resp, err := client.Complete(ctx, req)
if err != nil {
    if strings.Contains(err.Error(), "token refresh failed") {
        // Handle refresh failure
        fmt.Printf("Token refresh failed: %v\n", err)
    }
}
```

### Refresh Success but Still Unauthorized

If the refresh succeeds but the new token is also invalid, you'll get an `UnauthorizedError`:

```go
resp, err := client.Complete(ctx, req)
if err != nil {
    var authErr *client.UnauthorizedError
    if errors.As(err, &authErr) {
        // New token was also invalid
        fmt.Printf("Authentication failed even after refresh: %s\n", authErr.Message)
    }
}
```

## Thread Safety

The token refresh implementation is **thread-safe**:

- Token reads and updates use a `sync.RWMutex`
- Multiple concurrent requests can safely access and update the token
- If multiple requests receive 401 simultaneously, each will attempt refresh independently
- The last successful refresh will be used by all subsequent requests

## Behavior Details

### Retry Logic

- **Maximum retries**: 1 per request (prevents infinite loops)
- **Scope**: Token refresh is separate from the general retry mechanism
- **Status codes**: Only 401 Unauthorized triggers token refresh
- **Streaming**: Works for both streaming and non-streaming requests

### Performance Impact

- **Minimal overhead**: Token refresh only occurs on 401 responses
- **No preemptive refresh**: Tokens are not proactively refreshed
- **Thread-safe locking**: Uses efficient RWMutex for concurrent access

## Testing

The implementation includes comprehensive tests covering:

- ✅ 401 without refresh function configured
- ✅ Successful token refresh and retry
- ✅ Failed token refresh
- ✅ Refresh succeeds but new token still invalid
- ✅ Streaming with token refresh
- ✅ Concurrent requests with token refresh
- ✅ Thread safety

## Migration Guide

### Existing Code (No Changes Required)

Existing code continues to work without modification:

```go
// This still works exactly as before
cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  "your-api-key",
    Model:   "gpt-4",
}
```

### Adding Token Refresh

To add token refresh to existing code:

```go
cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  "your-api-key",
    Model:   "gpt-4",
    // Add this field:
    TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
        return yourRefreshLogic(ctx, oldToken)
    },
}
```

## Limitations

1. **Single retry**: Only one refresh attempt per request
2. **No preemptive refresh**: Tokens are not refreshed before expiration
3. **No token caching**: Each client instance manages its own token
4. **Synchronous refresh**: Blocks the request while refreshing

## Future Enhancements

Potential improvements for future versions:

- Preemptive token refresh based on expiration time
- Token refresh locking to prevent concurrent refresh attempts
- Configurable retry limits for token refresh
- Token refresh callbacks for monitoring/logging
