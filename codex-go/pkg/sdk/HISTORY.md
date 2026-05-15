# History Persistence in Codex SDK

This document explains how to use history persistence in the Codex SDK to save and resume conversation sessions.

## Overview

The Codex SDK provides built-in support for persisting conversation history to disk. This allows you to:

- Save conversations across application restarts
- Resume previous conversations from where they left off
- Manage multiple persistent sessions
- Track session metadata and history

## Enabling History Persistence

To enable history persistence, set `EnableHistory: true` when creating the SDK:

```go
package main

import (
    "context"
    "log"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    // Create client
    c, err := client.FromEnv()
    if err != nil {
        log.Fatal(err)
    }

    // Create SDK with history enabled
    s, err := sdk.New(sdk.Options{
        Client:        c,
        EnableHistory: true,
        // HistoryPath is optional; defaults to ~/.codex/sessions
        HistoryPath: "/path/to/history",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    // Your code here...
}
```

## History Storage

By default, history is stored in `~/.codex/sessions/`. Each session gets its own directory named by its session ID:

```
~/.codex/sessions/
├── session_1/
│   └── history.jsonl
├── session_2/
│   └── history.jsonl
└── abc123-def456/
    └── history.jsonl
```

The `history.jsonl` file contains a sequence of operations and events in JSON Lines format, allowing for efficient append-only writes and easy streaming reads.

## Creating a Session

Sessions are created the same way whether history is enabled or not:

```go
ctx := context.Background()

session, err := s.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt:     "You are a helpful coding assistant",
    Model:            "claude-3-5-sonnet-20241022",
    WorkingDirectory: ".",
    ApprovalPolicy:   "auto",
})
if err != nil {
    log.Fatal(err)
}

sessionID := session.ID()
log.Printf("Created session: %s", sessionID)
```

## Automatic History Persistence

When history is enabled, all session activity is automatically persisted:

- User messages
- Assistant responses
- Tool invocations and results
- Turn context (working directory, approval policy, etc.)
- Token usage statistics
- Errors and interruptions

The history is written incrementally after each operation, ensuring minimal data loss even if the application crashes.

## Resuming a Session

To resume a previous session, use `ResumeSession()`:

```go
// Resume an existing session by ID
session, err := s.ResumeSession(ctx, "session_1")
if err != nil {
    log.Fatal(err)
}

// The session is fully restored with:
// - Conversation history
// - Turn context (working directory, policies)
// - Token usage statistics
// - Last agent message

// Continue the conversation
response, err := session.Submit(ctx, "Can you remind me what we were working on?")
if err != nil {
    log.Fatal(err)
}

fmt.Println(response.Content)
```

## Listing Persisted Sessions

You can list all sessions with persisted history:

```go
sessions, err := s.ListPersistedSessions()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d persisted sessions:\n", len(sessions))
for _, sessionID := range sessions {
    fmt.Printf("  - %s\n", sessionID)
}
```

## Getting Session Metadata

Retrieve metadata about a persisted session:

```go
metadata, err := s.GetSessionMetadata("session_1")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Session: %s\n", metadata.SessionID)
fmt.Printf("Path: %s\n", metadata.Path)
fmt.Printf("Last Modified: %s\n", metadata.LastModified)
fmt.Printf("Turns: %d\n", metadata.TurnCount)
fmt.Printf("Messages: %d\n", metadata.MessageCount)
```

Note: Turn and message counts may be -1 if the information couldn't be determined without parsing the entire history file.

## Deleting Session History

To permanently delete a session's persisted history:

```go
err := s.DeleteSession("session_1")
if err != nil {
    log.Fatal(err)
}

fmt.Println("Session history deleted")
```

**Important**: This only deletes the persisted history. If the session is currently active in memory, you should close it first with `CloseSession()`.

## Complete Example

Here's a complete example demonstrating the full lifecycle:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    // Create client from environment variables
    c, err := client.FromEnv()
    if err != nil {
        log.Fatal(err)
    }

    // Create SDK with history enabled
    s, err := sdk.New(sdk.Options{
        Client:        c,
        EnableHistory: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    ctx := context.Background()

    // Check for existing sessions
    sessions, err := s.ListPersistedSessions()
    if err != nil {
        log.Fatal(err)
    }

    var session *sdk.Session

    if len(sessions) > 0 {
        // Resume the most recent session
        sessionID := sessions[len(sessions)-1]
        fmt.Printf("Resuming session: %s\n", sessionID)

        session, err = s.ResumeSession(ctx, sessionID)
        if err != nil {
            log.Fatal(err)
        }

        // Get session metadata
        metadata, err := s.GetSessionMetadata(sessionID)
        if err == nil {
            fmt.Printf("Session last modified: %s\n", metadata.LastModified)
        }
    } else {
        // Create a new session
        fmt.Println("Creating new session")

        session, err = s.NewSession(ctx, sdk.SessionOptions{
            SystemPrompt:     "You are a helpful coding assistant",
            Model:            "claude-3-5-sonnet-20241022",
            WorkingDirectory: ".",
            ApprovalPolicy:   "auto",
        })
        if err != nil {
            log.Fatal(err)
        }

        fmt.Printf("Created session: %s\n", session.ID())
    }

    // Have a conversation
    response, err := session.Submit(ctx, "What can you help me with?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Assistant: %s\n", response.Content)

    // Close the session (history is automatically saved)
    err = s.CloseSession(session.ID())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Session saved successfully")
}
```

## Session Lifecycle

### 1. Session Creation

When you create a session with history enabled:

1. The SDK generates a unique session ID (or uses the provided ID)
2. A session directory is created in the history path
3. A `history.jsonl` file is initialized
4. The session configuration event is written to history

### 2. During Conversation

As the conversation progresses:

1. Each user message is written to history
2. Assistant responses (including tool use) are written to history
3. Tool execution results are written to history
4. Token usage is tracked and persisted
5. All writes are atomic and use fsync for durability

### 3. Session Closure

When you close a session:

1. Any pending writes are flushed to disk
2. The history file is closed properly
3. The session is removed from active memory
4. History remains on disk for future resumption

### 4. Session Resumption

When you resume a session:

1. The history file is loaded from disk
2. All submissions and events are replayed
3. The conversation state is reconstructed:
   - Message history
   - Turn context (working directory, policies)
   - Token usage
   - Last agent message
4. The session is ready to continue

## Security Considerations

### File Permissions

Session directories and history files are created with restricted permissions:

- Session directories: `0700` (owner read/write/execute only)
- History files: `0600` (owner read/write only)

This ensures that only the user running the application can access the session data.

### Path Validation

The SDK performs comprehensive path validation to prevent:

- Path traversal attacks (`..` components)
- Symlink attacks
- Access outside the configured history directory

All session IDs and paths are sanitized before use.

### Sensitive Data

Be aware that history files contain:

- User messages (which may include sensitive data)
- Assistant responses
- Tool execution results (which may include file contents, command outputs)
- Working directory paths
- Token usage statistics

Ensure the history directory is:

- Located on an encrypted filesystem if sensitive data is involved
- Backed up securely if needed
- Excluded from version control (add to .gitignore)
- Cleaned up when no longer needed

## Best Practices

### 1. Use Meaningful Session IDs

While the SDK generates UUIDs by default, you can provide your own session IDs for better organization:

```go
session, err := s.NewSession(ctx, sdk.SessionOptions{
    ConversationID: "my-project-session-1",
    // ... other options
})
```

### 2. Clean Up Old Sessions

Periodically clean up old session history to save disk space:

```go
sessions, _ := s.ListPersistedSessions()

for _, sessionID := range sessions {
    metadata, err := s.GetSessionMetadata(sessionID)
    if err != nil {
        continue
    }

    // Delete sessions older than 30 days
    if isOlderThan(metadata.LastModified, 30*24*time.Hour) {
        _ = s.DeleteSession(sessionID)
    }
}
```

### 3. Handle Resume Errors

Session resumption can fail if history is corrupted or incompatible:

```go
session, err := s.ResumeSession(ctx, sessionID)
if err != nil {
    log.Printf("Failed to resume session: %v", err)

    // Create a new session instead
    session, err = s.NewSession(ctx, sdk.SessionOptions{
        // ... options
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### 4. Close Sessions Properly

Always close sessions when done to ensure history is written:

```go
defer func() {
    if err := s.CloseSession(sessionID); err != nil {
        log.Printf("Error closing session: %v", err)
    }
}()
```

### 5. Use Context Cancellation

Respect context cancellation for long-running operations:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

response, err := session.Submit(ctx, "Long running task...")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Operation timed out")
    }
    log.Fatal(err)
}
```

## Troubleshooting

### History Not Being Saved

1. Verify history is enabled: `EnableHistory: true`
2. Check the history path exists and is writable
3. Ensure you're calling `CloseSession()` or `Close()`
4. Check logs for permission errors

### Cannot Resume Session

1. Verify the session ID is correct
2. Check the history file exists and is readable
3. Ensure the history file is not corrupted
4. Check for version incompatibilities

### Disk Space Issues

History files grow over time. Consider:

1. Implementing automatic cleanup of old sessions
2. Using compression on the history directory
3. Monitoring disk usage
4. Implementing session archival strategies

## API Reference

### SDK Methods

- `New(Options) (*SDK, error)` - Create SDK with history configuration
- `NewSession(context.Context, SessionOptions) (*Session, error)` - Create new session
- `ResumeSession(context.Context, string) (*Session, error)` - Resume existing session
- `ListPersistedSessions() ([]string, error)` - List all persisted sessions
- `DeleteSession(string) error` - Delete session history
- `GetSessionMetadata(string) (*SessionMetadata, error)` - Get session metadata
- `CloseSession(string) error` - Close and persist session
- `Close() error` - Close all sessions and SDK

### Options

```go
type Options struct {
    Client        *client.Client  // Required
    EnableHistory bool            // Enable history persistence
    HistoryPath   string          // Optional, defaults to ~/.codex/sessions
    Tools         []runtime.ToolRuntime
    ToolRegistry  *runtime.ToolRegistry
}
```

### SessionMetadata

```go
type SessionMetadata struct {
    SessionID    string  // Unique session identifier
    Path         string  // Absolute path to session directory
    TurnCount    int     // Number of turns (-1 if unknown)
    MessageCount int     // Number of messages (-1 if unknown)
    LastModified string  // Last modification timestamp
}
```

## Related Documentation

- [SDK README](./README.md) - General SDK documentation
- [Session Management](./SESSION.md) - Session lifecycle and management
- [Internal History Persistence](../../internal/history/persistence/README.md) - Low-level persistence implementation
