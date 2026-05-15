package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
)

// streamParser handles SSE (Server-Sent Events) parsing for streaming responses.
type streamParser struct {
	config client.StreamConfig
}

// newStreamParser creates a new stream parser.
func newStreamParser(config client.StreamConfig) *streamParser {
	return &streamParser{
		config: config,
	}
}

// sendEventWithBackpressure sends an event to the channel with optional backpressure control.
// It monitors buffer usage and logs warnings when the buffer is consistently full.
// Returns an error if the context is cancelled or the channel is closed.
func (p *streamParser) sendEventWithBackpressure(ctx context.Context, eventCh chan<- client.StreamEvent, event client.StreamEvent) error {
	if !p.config.EnableBackpressure {
		// No backpressure: try to send immediately, drop if buffer is full
		select {
		case <-ctx.Done():
			return ctx.Err()
		case eventCh <- event:
			return nil
		default:
			// Buffer full, drop event
			log.Printf("Warning: Stream buffer full, dropping event (type: %s)", event.Type)
			return nil
		}
	}

	// Backpressure enabled: block until space is available
	// First, check buffer usage and warn if needed
	bufferSize := cap(eventCh)
	currentLen := len(eventCh)
	usagePercent := float64(currentLen) / float64(bufferSize)

	if usagePercent >= p.config.BackpressureThreshold {
		log.Printf("Warning: Stream buffer usage high: %d/%d (%.1f%%) - applying backpressure",
			currentLen, bufferSize, usagePercent*100)
	}

	// Block until we can send or context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	case eventCh <- event:
		return nil
	}
}

// scanResult represents the result of a scanner.Scan() operation.
type scanResult struct {
	ok   bool
	line string
	err  error
}

// parse reads and parses an SSE stream, emitting events to the channel.
//
// Note: The scanner goroutine cannot be interrupted mid-scan due to bufio.Scanner
// limitations. In slow network conditions, cancellation may be delayed until the
// next read completes. The underlying reader should have appropriate timeouts configured
// to prevent indefinite blocking. For network streams, consider using http.Client with
// appropriate timeouts (ReadTimeout, IdleConnTimeout) to ensure the scanner can be
// interrupted in a reasonable time frame.
func (p *streamParser) parse(ctx context.Context, r io.Reader, eventCh chan<- client.StreamEvent) error {
	scanner := bufio.NewScanner(r)

	// Increase buffer size to handle large chunks (e.g., large tool call arguments).
	// Default bufio.Scanner buffer is 64KB, which is insufficient for large responses.
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	// Configure idle timeout if set
	var idleTimer *time.Timer
	var idleTimerCh <-chan time.Time
	if p.config.IdleTimeout > 0 {
		idleTimer = time.NewTimer(p.config.IdleTimeout)
		idleTimerCh = idleTimer.C
		defer idleTimer.Stop()
	}

	// Track accumulated tool calls for streaming
	toolCallAccumulator := newToolCallAccumulator()

	// Create a single goroutine for scanning that runs for the lifetime of this function.
	// This prevents goroutine leaks that would occur if we created a new goroutine on each iteration.
	//
	// IMPORTANT: bufio.Scanner.Scan() is a blocking operation that cannot be interrupted
	// mid-scan. The goroutine will only check for cancellation between scans. To ensure
	// timely cancellation:
	// 1. The underlying reader should have appropriate timeouts
	// 2. For HTTP streams, configure client with ReadTimeout/IdleConnTimeout
	// 3. Consider wrapping the reader with a context-aware reader that returns errors on ctx.Done()
	scanCh := make(chan scanResult)
	scanDone := make(chan struct{})
	defer close(scanDone)

	go func() {
		defer close(scanCh) // Close channel when goroutine exits
		for {
			// Check for cancellation before starting the blocking scan operation.
			// This provides a fast path for cancellation when the scanner is between lines.
			select {
			case <-scanDone:
				// Parse function returned, exit goroutine immediately
				return
			default:
				// No cancellation yet, proceed with scan
			}

			// Perform the blocking scan operation.
			// WARNING: This call cannot be interrupted! It will block until:
			// - A complete line is read
			// - EOF is reached
			// - An error occurs
			// - The underlying reader times out (if configured)
			ok := scanner.Scan()
			var line string
			var err error
			if ok {
				// Capture the line text immediately while it's still valid
				line = scanner.Text()
			} else {
				err = scanner.Err()
			}

			// Try to send result, but respect cancellation
			select {
			case scanCh <- scanResult{ok: ok, line: line, err: err}:
				if !ok {
					// Scanner is done, exit goroutine
					return
				}
				// Successfully sent, continue to next iteration
			case <-scanDone:
				// Parse function returned, exit goroutine
				return
			}
		}
	}()

	for {
		// Reset idle timer
		if idleTimer != nil {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(p.config.IdleTimeout)
		}

		// Wait for next line or timeout
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-idleTimerCh:
			err := client.NewIdleTimeoutError(p.config.IdleTimeout)
			_ = p.sendEventWithBackpressure(ctx, eventCh, client.StreamEvent{
				Type:  client.EventTypeError,
				Error: err,
			})
			return err

		case result := <-scanCh:
			if !result.ok {
				// End of stream
				if result.err != nil {
					return fmt.Errorf("stream scan error: %w", result.err)
				}
				return nil
			}

			// Use the line from the scan result (captured in the goroutine)
			line := result.line

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE line
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Check for [DONE] marker
			if data == "[DONE]" {
				// Stream completed successfully
				return nil
			}

			// Parse JSON chunk
			var chunk client.ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Log but continue - some providers send non-JSON comments
				continue
			}

			// Process chunk and emit events
			events := p.processChunk(&chunk, toolCallAccumulator)
			for _, event := range events {
				if err := p.sendEventWithBackpressure(ctx, eventCh, event); err != nil {
					return err
				}
			}
		}
	}
}

// processChunk converts a ChatCompletionChunk into StreamEvents.
func (p *streamParser) processChunk(chunk *client.ChatCompletionChunk, accumulator *toolCallAccumulator) []client.StreamEvent {
	var events []client.StreamEvent

	// Process each choice
	for _, choice := range chunk.Choices {
		// Handle role (first chunk)
		if choice.Delta.Role != "" {
			events = append(events, client.StreamEvent{
				Type: client.EventTypeCreated,
				Data: map[string]interface{}{
					"id":    chunk.ID,
					"model": chunk.Model,
					"role":  choice.Delta.Role,
				},
			})
		}

		// Handle content delta
		if choice.Delta.Content != "" {
			events = append(events, client.StreamEvent{
				Type: client.EventTypeOutputTextDelta,
				Data: choice.Delta.Content,
			})
		}

		// Handle reasoning delta (if present)
		if choice.Delta.Reasoning != nil {
			if p.config.EnableRawAgentReasoning {
				// Extract reasoning content text
				var reasoningText string
				switch v := choice.Delta.Reasoning.(type) {
				case string:
					reasoningText = v
				case map[string]interface{}:
					// Handle structured reasoning content (e.g., {"type": "reasoning_content", "text": "..."})
					if text, ok := v["text"].(string); ok {
						reasoningText = text
					} else if content, ok := v["content"].(string); ok {
						reasoningText = content
					}
				}

				if reasoningText != "" {
					events = append(events, client.StreamEvent{
						Type: client.EventTypeReasoningContentDelta,
						Data: reasoningText,
					})
				}
			}
		}

		// Handle tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, toolCallDelta := range choice.Delta.ToolCalls {
				accumulator.add(toolCallDelta)
			}
		}

		// Handle finish reason
		if choice.FinishReason != "" {
			// Emit any accumulated tool calls
			toolCalls := accumulator.getCompleted()
			if len(toolCalls) > 0 {
				events = append(events, client.StreamEvent{
					Type: client.EventTypeOutputItemDone,
					Data: map[string]interface{}{
						"tool_calls": toolCalls,
					},
				})
			}

			// Emit completion event with usage
			completedEvent := &client.CompletedEvent{
				ResponseID: chunk.ID,
			}
			if chunk.Usage != nil {
				completedEvent.TokenUsage = chunk.Usage
			}

			events = append(events, client.StreamEvent{
				Type: client.EventTypeCompleted,
				Data: completedEvent,
			})
		}
	}

	return events
}

// toolCallAccumulator accumulates streaming tool call fragments.
type toolCallAccumulator struct {
	calls map[int]*client.ToolCall
}

// newToolCallAccumulator creates a new tool call accumulator.
func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{
		calls: make(map[int]*client.ToolCall),
	}
}

// add processes a tool call delta and accumulates it.
func (a *toolCallAccumulator) add(delta client.ToolCallDelta) {
	// Get or create tool call
	call, exists := a.calls[delta.Index]
	if !exists {
		call = &client.ToolCall{
			ID:   delta.ID,
			Type: delta.Type,
		}
		a.calls[delta.Index] = call
	}

	// Update ID and type if present (first chunk)
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Type != "" {
		call.Type = delta.Type
	}

	// Accumulate function call data
	if delta.Function != nil {
		if call.Function == nil {
			call.Function = &client.FunctionCall{}
		}

		if delta.Function.Name != "" {
			call.Function.Name = delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			call.Function.Arguments += delta.Function.Arguments
		}
	}
}

// getCompleted returns all completed tool calls.
func (a *toolCallAccumulator) getCompleted() []client.ToolCall {
	var calls []client.ToolCall

	// Return calls in index order
	maxIndex := -1
	for idx := range a.calls {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	for i := 0; i <= maxIndex; i++ {
		if call, exists := a.calls[i]; exists {
			calls = append(calls, *call)
		}
	}

	return calls
}

// reset clears the accumulator.
// nolint:unused // Reserved for multi-turn tool call handling
func (a *toolCallAccumulator) reset() {
	a.calls = make(map[int]*client.ToolCall)
}
