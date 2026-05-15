# Codex CLI

A command-line interface for interacting with AI assistants with real-time streaming responses.

## Features

- **Interactive TUI Mode**: Full-featured terminal interface with session management
- **Non-Interactive CLI Mode**: Single message mode for scripting and automation
- **Real-time Streaming**: See AI responses as they're generated
- **Session Management**: Create and switch between multiple conversation sessions
- **Tool Execution**: Watch tool calls and command outputs in real-time
- **Token Tracking**: Monitor token usage throughout conversations

## Installation

```bash
# Build from cmd/codex directory
go build -o codex

# Or from project root
go build -o codex ./cmd/codex
```

## Configuration

### Environment Variables

- `ANTHROPIC_API_KEY` - API key for Anthropic Claude models (required if using Claude)
- `OPENAI_API_KEY` - API key for OpenAI models (required if using OpenAI)
- `API_BASE_URL` - Base URL for the API (default: `https://api.anthropic.com/v1`)
- `MODEL` - Model to use (default: `claude-3-5-sonnet-20241022`)

### Example Setup

```bash
# For Anthropic Claude
export ANTHROPIC_API_KEY="your-api-key-here"

# For OpenAI
export OPENAI_API_KEY="your-api-key-here"
export API_BASE_URL="https://api.openai.com/v1"
export MODEL="gpt-4"
```

## Usage

### Interactive Mode (TUI)

Start the TUI by running without arguments:

```bash
./codex
```

### Non-Interactive Mode (CLI)

Send a single message and get a streaming response:

```bash
# Basic usage
./codex -m "What is 2+2?"

# With specific session
./codex -s "my-session" -m "Remember this conversation"

# Continue in same session
./codex -s "my-session" -m "What did we talk about?"

# With specific model
./codex --model "gpt-4" -m "Explain quantum computing"
```

#### Flags

- `-m, --message` - Message to send (required for non-interactive mode)
- `-s, --session` - Session ID to use (optional, auto-generated if not provided)
- `--model` - Model to use (overrides MODEL env var)

## Keyboard Shortcuts

### Navigation
- `↑`/`k`: Move up in lists
- `↓`/`j`: Move down in lists
- `Enter`: Select item / Submit message

### Actions
- `n`: Create new session
- `a`: Approve tool execution
- `d`: Deny tool execution
- `q` / `Ctrl+C`: Quit application

## Views

### Session List View

The initial view showing all available sessions. Press `n` to create a new session or select an existing one with `Enter`.

### Conversation View

The main chat interface where you interact with the AI assistant. Type your message and press `Enter` to submit.

### Tool Approval View

When the AI requests to execute a tool, an approval panel appears showing:
- Tool name
- Parameters
- Risk level

Press `a` to approve or `d` to deny the tool execution.

## Architecture

The TUI is built using [Bubble Tea](https://github.com/charmbracelet/bubbletea), a Go framework for building terminal applications using The Elm Architecture.

### Components

- **app.go**: Main Bubble Tea model and update logic
- **views.go**: Rendering functions for different UI views
- **keys.go**: Keyboard binding definitions

### Integration

The TUI integrates with internal packages:
- `internal/conversation/manager`: Session and conversation management
- `internal/protocol`: Protocol types for operations and events
- `internal/client`: AI model client interface

## Testing

Run the automated streaming test:

```bash
./scripts/test_streaming.sh
```

This validates:
1. Basic message sending and streaming
2. Real-time response streaming
3. Session persistence across multiple messages

## Architecture

### Event-Driven Streaming

The CLI uses an event-driven architecture for real-time streaming:

1. **User Input** → Submitted to conversation manager
2. **Turn Processing** → Manager processes turn in background
3. **Events Emitted** → Protocol events sent to registered handlers
4. **Event Types**:
   - `EventTaskStarted` - Turn begins
   - `EventAgentMessageDelta` - Streaming text chunks
   - `EventTokenCount` - Token usage updates
   - `EventExecCommandBegin/End` - Tool execution
   - `EventToolCallApprovalNeeded` - Approval required
   - `EventTaskComplete` - Turn complete

## Development

### Building

```bash
go build -o codex ./cmd/codex
```

### Running Tests

```bash
# Unit tests
go test ./cmd/codex/...

# Integration tests (requires API key)
cd test/integration
go test -v

# Streaming validation
./scripts/test_streaming.sh
```

## Troubleshooting

### No API Key Error

```
Error: API key required: set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable
```

**Solution**: Set the appropriate environment variable.

### No Response / Hanging

- Check if the model name is valid for your provider
- Verify the API key has permissions for the specified model
- Look for error messages in stderr

### Session Already Exists

This can happen if a previous instance didn't clean up properly. Check for running processes:

```bash
ps aux | grep codex
```
