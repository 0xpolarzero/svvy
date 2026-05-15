# Notification System

The notification system allows Codex to execute external scripts when specific events occur during conversation turns. This enables integration with external tools, logging systems, or notification services.

## Features

- **Event Triggers**: Run scripts on turn completion, errors, approval requests, and interrupts
- **Background Execution**: Scripts run asynchronously with configurable timeouts (default: 5 seconds)
- **Environment Variables**: Full event context passed as environment variables
- **Custom Environment**: Support for custom environment variables per trigger
- **Fire-and-Forget**: Non-blocking execution that doesn't affect conversation flow

## Configuration

Add notification configuration to your `~/.codex/config.toml`:

```toml
[notify]
# Optional: Set script timeout in seconds (default: 5)
script_timeout_sec = 10.0

# Trigger when a turn completes successfully
[notify.on_turn_complete]
enabled = true
command = "/path/to/notify-complete.sh"

# Optional: Custom environment variables for this trigger
[notify.on_turn_complete.env]
SLACK_WEBHOOK = "https://hooks.slack.com/..."
NOTIFICATION_LEVEL = "info"

# Trigger when a turn encounters an error
[notify.on_error]
enabled = true
command = "/path/to/notify-error.sh"

# Trigger when user approval is needed
[notify.on_approval_needed]
enabled = true
command = "/path/to/notify-approval.sh"

# Trigger when a turn is interrupted/aborted
[notify.on_turn_aborted]
enabled = true
command = "/path/to/notify-aborted.sh"
```

## Environment Variables

All notification scripts receive the following environment variables:

### Standard Variables

- `CODEX_EVENT_TYPE`: Event type (`turn_complete`, `turn_error`, `approval_needed`, `turn_aborted`)
- `CODEX_SESSION_ID`: Unique session identifier
- `CODEX_TURN_ID`: Unique turn identifier
- `CODEX_TIMESTAMP`: Event timestamp (RFC3339 format)
- `CODEX_STATUS`: Turn status (`success`, `error`, `aborted`, `waiting`)
- `CODEX_MESSAGE`: Optional human-readable message
- `CODEX_ERROR_MESSAGE`: Error message (populated when status is `error`)

### Metadata Variables

Additional metadata is passed as `CODEX_<KEY>` environment variables. For example:
- `CODEX_TOOL_NAME`: Tool requiring approval (for approval events)
- `CODEX_RISK_LEVEL`: Risk level assessment

## Example Scripts

### Slack Notification

```bash
#!/bin/bash
# notify-complete.sh - Send Slack notification on turn completion

MESSAGE="✅ Codex turn completed
Session: $CODEX_SESSION_ID
Turn: $CODEX_TURN_ID
Status: $CODEX_STATUS
Message: $CODEX_MESSAGE"

curl -X POST "$SLACK_WEBHOOK" \
  -H 'Content-Type: application/json' \
  -d "{\"text\":\"$MESSAGE\"}"
```

### Desktop Notification (macOS)

```bash
#!/bin/bash
# notify-error.sh - Show desktop notification on error

osascript -e "display notification \"$CODEX_ERROR_MESSAGE\" with title \"Codex Error\" sound name \"Basso\""
```

### Logging to File

```bash
#!/bin/bash
# log-event.sh - Log events to a file

LOG_FILE="$HOME/.codex/event.log"
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) [$CODEX_EVENT_TYPE] Session=$CODEX_SESSION_ID Turn=$CODEX_TURN_ID Status=$CODEX_STATUS" >> "$LOG_FILE"
```

### Sound Alert (macOS)

```bash
#!/bin/bash
# sound-alert.sh - Play sound on approval needed

if [ "$CODEX_EVENT_TYPE" = "approval_needed" ]; then
  afplay /System/Library/Sounds/Glass.aiff
fi
```

## Architecture

### Components

1. **Event Types** (`event.go`): Defines notification event types and structures
2. **Script Executor** (`script.go`): Handles script execution with timeouts and environment setup
3. **Notifier** (`notify.go`): Main notification manager that dispatches events to scripts
4. **Configuration** (in `config/config.go`): TOML configuration structure

### Integration Points

The notification system is integrated into the conversation manager:

- **Turn Completion**: Notifies when `session.CompleteTurn()` succeeds
- **Turn Error**: Notifies when turn processing or completion fails
- **Turn Abort**: Notifies when user sends an interrupt operation
- **Approval Needed**: Can be triggered when tool approval is required

### Design Principles

1. **Non-blocking**: Scripts execute in background goroutines
2. **Fire-and-Forget**: Script failures don't affect conversation flow
3. **Timeout Protection**: Scripts are killed after timeout (default 5s)
4. **Thread-safe**: Concurrent notifications are properly synchronized

## Testing

The package includes comprehensive tests:

```bash
cd codex-go
go test ./internal/notify/... -v
```

Tests cover:
- Notifier lifecycle (enable/disable, config updates)
- All event types (complete, error, approval, abort)
- Script execution with timeouts
- Environment variable passing
- Concurrent notifications
- Command parsing

## Performance Considerations

- Scripts run asynchronously to avoid blocking conversation turns
- Default 5-second timeout prevents hung scripts from accumulating
- Each script runs in its own process with isolated environment
- No output capture or error checking (fire-and-forget model)

## Security Notes

- Scripts run with the same permissions as the Codex process
- Be cautious with commands from configuration files
- Avoid passing sensitive data through environment variables in shared environments
- Consider script permissions (chmod +x) for executable scripts
