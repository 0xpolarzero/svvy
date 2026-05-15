# Codex Go Architecture

This document describes the architecture of the Codex Go rewrite, including design principles, layering, and component organization.

## Overview

Codex Go is a complete rewrite of the Codex TUI in Go, using the Bubble Tea framework for terminal UI. The architecture follows a layered design pattern, with clear separation of concerns and dependency flow from core to UI.

## Design Principles

### 1. Layered Architecture

The codebase is organized in distinct layers, each with specific responsibilities:

```
┌─────────────────────────────────────┐
│         TUI (Bubble Tea)            │  User Interface Layer
├─────────────────────────────────────┤
│      SDK (Public Go API)            │  Public API Layer
├─────────────────────────────────────┤
│    Tools & MCP Integration          │  Integration Layer
├─────────────────────────────────────┤
│  Conversation & History Manager     │  Business Logic Layer
├─────────────────────────────────────┤
│      API Client & Streaming         │  API Layer
├─────────────────────────────────────┤
│   Protocol, Config, Errors          │  Foundation Layer
└─────────────────────────────────────┘
```

**Dependency Rules:**
- Higher layers depend on lower layers
- Lower layers never depend on higher layers
- Each layer has well-defined interfaces
- Dependencies are injected, not hardcoded

### 2. Test-Driven Development

All code is developed using TDD:
- Tests written before implementation
- High test coverage (target: >80%)
- Golden files for complex outputs
- Mocked external dependencies

### 3. Interface-Based Design

Core abstractions use interfaces for:
- Testability (easy mocking)
- Flexibility (multiple implementations)
- Loose coupling (dependency injection)

### 4. Explicit Error Handling

- No panics in production code
- Rich error context using custom error types
- Error wrapping with additional context
- Clear error propagation

## Layer Details

### Foundation Layer

#### `internal/protocol/`

Defines core protocol types used throughout the application.

**Key Types:**
- `Op`: Operation types (completion, exec, patch, etc.)
- `Event`: Streaming event types
- `Message`: Chat message structure
- `Tool`: Tool definition and result types

**Dependencies:** None (pure data structures)

#### `internal/config/`

Configuration loading and validation.

**Responsibilities:**
- Load config from files and environment
- Validate configuration schema
- Provide typed access to settings
- Support config overrides

**Key Types:**
- `Config`: Main configuration structure
- `APIConfig`: API endpoint and auth settings
- `ToolConfig`: Tool-specific configuration

#### `internal/errors/`

Custom error types with rich context.

**Key Types:**
- `Error`: Base error type with code and context
- `ErrorCode`: Enumeration of error types
- Helper functions for error wrapping

### API Layer

#### `internal/client/`

HTTP client for Anthropic API with streaming support.

**Responsibilities:**
- Make API requests to Claude API
- Handle streaming responses (SSE)
- Manage authentication
- Retry logic and error handling

**Key Interfaces:**
```go
type Client interface {
    Complete(ctx context.Context, req CompletionRequest) (*Response, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan Event, error)
}
```

**Key Features:**
- Streaming via Server-Sent Events (SSE)
- Request cancellation via context
- Rate limiting and backoff
- Token counting and usage tracking

#### `internal/tokencount/`

Token counting and usage tracking.

**Responsibilities:**
- Estimate token counts for messages
- Track token usage across sessions
- Calculate costs based on model pricing

### Business Logic Layer

#### `internal/conversation/`

Session and conversation state management.

**Responsibilities:**
- Manage conversation turns
- Track message history
- Handle tool use and results
- Maintain context window limits

**Key Types:**
- `Session`: Current conversation session
- `Turn`: Single user-assistant exchange
- `Context`: Conversation context with memory

**State Machine:**
```
User Input → Processing → Tool Execution → Assistant Response → Complete
                ↑__________________|
```

Multi-turn flow:

```
User Input
  ↓
Stream assistant deltas
  ↓
Output item done → parse tool_calls
  ↓
Execute tools (with approval if required) → emit begin/deltas/end
  ↓
Append tool results to messages → stream follow-up
  ↓
Repeat until no tool_calls or max turns reached
  ↓
Emit task_complete + cumulative token usage
```

Approval workflow:

- Orchestrator requests approval via SessionApprovalHandler, which:
  - Transitions session to AwaitingApproval
  - Emits tool_call_approval_needed with risk context
  - Blocks until SubmitApproval or context cancellation
  - Denies on concurrent requests to avoid ambiguity

Persistence reconstruction:

- ReconstructStateFromHistory builds:
  - Conversation history (user/assistant/tool_result)
  - Token usage (cumulative)
  - Turn context (cwd, sandbox, approval policy, model)
  - Stats and validation flags (incomplete/interrupted turns)

#### `internal/history/`

Conversation persistence and loading.

**Responsibilities:**
- Save conversations to disk
- Load previous conversations
- Search conversation history
- Export/import conversations

**Storage Format:**
- JSON files in `~/.codex/history/`
- One file per conversation
- Indexed for fast search

### Integration Layer

#### `internal/tools/`

Tool runtime system and built-in tool implementations.

**Responsibilities:**
- Tool discovery and registration
- Tool execution and sandboxing
- Result formatting and validation

**Key Interfaces:**
```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, params map[string]interface{}) (Result, error)
}
```

**Built-in Tools:**
- `file/`: File operations (read, write, edit)
- `shell/`: Shell command execution
- `patch/`: Code patching with approval
- `runtime/`: Runtime information and control

#### `internal/sandbox/`

Process isolation and sandboxing for tool execution.

**Responsibilities:**
- Isolate tool execution
- Resource limits (CPU, memory, time)
- Security boundaries

**Features:**
- Command timeout enforcement
- Working directory isolation
- Environment variable control

#### `internal/mcp/`

Model Context Protocol (MCP) client for external tools.

**Responsibilities:**
- Discover MCP servers
- Connect to MCP tools
- Translate between Codex and MCP formats
- Manage MCP server lifecycle

**Key Features:**
- Auto-discovery of local MCP servers
- Tool schema translation
- Result marshaling

### Public API Layer

#### `pkg/sdk/`

Public Go SDK for embedding Codex in other applications.

**Responsibilities:**
- Provide stable public API
- Abstract internal implementation details
- Support embedding and scripting

**Key Types:**
```go
type Client interface {
    NewSession(opts SessionOptions) (Session, error)
}

type Session interface {
    Send(message string) error
    Receive() (<-chan Event, error)
    Close() error
}
```

**Use Cases:**
- Embedding in Go applications
- Building custom tools
- Automation scripts

### User Interface Layer

#### `internal/tui/`

Bubble Tea-based terminal user interface.

**Responsibilities:**
- Render UI components
- Handle user input
- Display streaming updates
- Manage UI state

**Structure:**
```
tui/
├── model/          # Bubble Tea model (app state)
├── views/          # Full-screen views (chat, history, settings)
├── components/     # Reusable UI components
│   ├── message.go      # Message bubble
│   ├── input.go        # User input area
│   ├── sidebar.go      # Status sidebar
│   └── viewer.go       # Image/file viewer
└── tui.go          # Main entry point
```

**Key Components:**

1. **ChatView**: Main conversation interface
   - Message history
   - Streaming updates
   - Tool execution indicators

2. **InputArea**: User input handling
   - Multi-line editing
   - Command completion
   - File attachment

3. **Sidebar**: Status and context
   - Token usage
   - Active tools
   - Session info

4. **ApprovalDialog**: Command/patch approval
   - Diff display
   - Approve/reject actions

#### `cmd/codex/`

Application entry point.

**Responsibilities:**
- Parse command-line flags
- Initialize configuration
- Start TUI or CLI mode
- Handle signals and cleanup

## Data Flow

### Typical Conversation Flow

```
1. User enters message in TUI
   ↓
2. TUI → Session: Send(message)
   ↓
3. Session → Client: Complete(request)
   ↓
4. Client → API: HTTP request with streaming
   ↓
5. API → Client: Stream events (SSE)
   ↓
6. Client → Session: Event channel
   ↓
7. Session processes events:
   - Text delta: Accumulate response
   - Tool use: Execute tool
   - Tool result: Send back to API
   ↓
8. Session → TUI: Update UI with events
   ↓
9. TUI renders updates in real-time
```

### Tool Execution Flow

```
1. API requests tool use
   ↓
2. Session → Tools: Execute(tool_name, params)
   ↓
3. Tools → Sandbox: Run isolated
   ↓
4. Sandbox executes with limits
   ↓
5. Sandbox → Tools: Result
   ↓
6. Tools → Session: Formatted result
   ↓
7. Session → Client: Send tool result to API
   ↓
8. API processes and continues response
```

## Key Design Patterns

### 1. Repository Pattern

Used in `history/` for conversation persistence:

```go
type Repository interface {
    Save(ctx context.Context, conv Conversation) error
    Load(ctx context.Context, id string) (Conversation, error)
    List(ctx context.Context) ([]Conversation, error)
}
```

### 2. Strategy Pattern

Used in `tools/` for different tool implementations:

```go
type Tool interface {
    Execute(ctx context.Context, params Params) (Result, error)
}

// Different strategies for different tools
type FileReader struct {}
type ShellExecutor struct {}
type PatchApplier struct {}
```

### 3. Observer Pattern

Used in `client/` for streaming events:

```go
// Client publishes events
events := client.Stream(ctx, req)

// Observers consume events
for event := range events {
    session.HandleEvent(event)
}
```

### 4. Factory Pattern

Used in `tools/` for tool creation:

```go
type ToolFactory interface {
    Create(name string) (Tool, error)
}

func NewToolFactory(config Config) ToolFactory {
    return &defaultFactory{config}
}
```

### 5. Dependency Injection

Used throughout for testability:

```go
type Session struct {
    client Client        // Injected
    tools  ToolRegistry  // Injected
    history Repository   // Injected
}

func NewSession(client Client, tools ToolRegistry, history Repository) *Session {
    return &Session{client, tools, history}
}
```

## Testing Strategy

### Unit Tests

Each package has comprehensive unit tests:
- Test all public functions
- Test error cases
- Use table-driven tests
- Mock external dependencies

### Integration Tests

Test interaction between layers:
- API client with mock server
- Session with mock client and tools
- TUI with mock session

### Golden File Tests

For complex outputs:
- JSON structures
- ANSI rendering
- Diff generation

See [TESTING.md](TESTING.md) for details.

## Performance Considerations

### 1. Streaming

All API responses use streaming to provide immediate feedback:
- Start rendering as soon as first token arrives
- Update UI incrementally
- Low latency user experience

### 2. Concurrency

Go routines for parallel operations:
- API streaming in background
- Tool execution doesn't block UI
- History saves asynchronously

### 3. Memory Management

- Limit conversation context window
- Prune old messages when needed
- Stream large files instead of loading fully

### 4. Caching

- Cache tokenization results
- Reuse HTTP connections
- Cache tool schemas

## Security Considerations

### 1. Sandboxing

Tool execution is sandboxed:
- Limited filesystem access
- Resource limits (CPU, memory, time)
- No network access by default

### 2. Approval Workflow

Dangerous operations require approval:
- Shell command execution
- File modifications
- Patch application

### 3. API Key Management

- Store API keys securely
- Never log API keys
- Support environment variables

### 4. Input Validation

- Validate all user inputs
- Sanitize tool parameters
- Escape shell commands

## Extension Points

### Adding New Tools

1. Implement `Tool` interface
2. Register with `ToolRegistry`
3. Add tests
4. Document in tool's package

### Adding New MCP Servers

1. Configure in `~/.codex/config.toml`
2. MCP client auto-discovers
3. Tools appear in tool list

### Custom Rendering

1. Implement custom view in `tui/views/`
2. Add to view router
3. Handle in main model

## Migration from Rust

See [MIGRATION.md](MIGRATION.md) for details on migrating from the Rust implementation.

## Future Enhancements

### Planned Features

- [ ] Plugin system for custom tools
- [ ] Web UI alongside TUI
- [ ] Multi-session management
- [ ] Collaborative conversations
- [ ] Voice input support

### Performance Improvements

- [ ] Connection pooling
- [ ] Response caching
- [ ] Parallel tool execution
- [ ] Incremental rendering

## Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Anthropic API Documentation](https://docs.anthropic.com/)
- [Model Context Protocol Spec](https://modelcontextprotocol.io/)
