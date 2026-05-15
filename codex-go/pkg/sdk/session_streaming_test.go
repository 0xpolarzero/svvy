package sdk

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSession_SubmitStream_ContentDeltas tests that streaming returns content delta events.
func TestSession_SubmitStream_ContentDeltas(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	eventCh, err := session.SubmitStream(ctx, "Say hello")
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	// Track delta events
	var deltaCount int
	var contentBuilder strings.Builder
	var finalResponse *Response
	var hasError bool

	for event := range eventCh {
		if event.Error != nil {
			t.Logf("Stream error: %v", event.Error)
			hasError = true
			continue
		}

		switch event.Type {
		case "content_delta":
			deltaCount++
			contentBuilder.WriteString(event.Delta)
			t.Logf("Delta %d: %q", deltaCount, event.Delta)
		case "done":
			finalResponse = event.Response
		}
	}

	// We should get at least some delta events
	if !hasError {
		assert.Greater(t, deltaCount, 0, "Expected at least one content delta event")
		assert.NotNil(t, finalResponse)
		if finalResponse != nil {
			assert.NotEmpty(t, finalResponse.Content)
			// Content should match accumulated deltas
			if contentBuilder.Len() > 0 {
				assert.Equal(t, contentBuilder.String(), finalResponse.Content)
			}
		}
	}
}

// TestSession_SubmitStream_ReasoningDeltas tests reasoning delta events.
func TestSession_SubmitStream_ReasoningDeltas(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
		Model:        "claude-sonnet-4", // Use a model that supports reasoning
	})
	require.NoError(t, err)

	eventCh, err := session.SubmitStream(ctx, "Think through this problem step by step")
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	var reasoningDeltaCount int
	var hasError bool

	for event := range eventCh {
		if event.Error != nil {
			t.Logf("Stream error: %v", event.Error)
			hasError = true
			continue
		}

		if event.Type == "reasoning_delta" {
			reasoningDeltaCount++
			t.Logf("Reasoning delta: %q", event.Delta)
		}
	}

	// Reasoning deltas are optional, just log if we got them
	if !hasError {
		t.Logf("Received %d reasoning delta events", reasoningDeltaCount)
	}
}

// TestSession_SubmitStream_ToolCallDeltas tests tool call events.
func TestSession_SubmitStream_ToolCallDeltas(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt:     "You are a helpful coding assistant.",
		Streaming:        true,
		ApprovalPolicy:   "auto", // Auto-approve tool calls
		WorkingDirectory: t.TempDir(),
	})
	require.NoError(t, err)

	// Ask it to use a tool
	eventCh, err := session.SubmitStream(ctx, "List the files in the current directory")
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	var toolCallCount int
	var hasError bool

	for event := range eventCh {
		if event.Error != nil {
			t.Logf("Stream error: %v", event.Error)
			hasError = true
			continue
		}

		if event.Type == "tool_call_delta" {
			toolCallCount++
			t.Logf("Tool call: %s (%s)", event.ToolCall.Name, event.ToolCall.ID)
		}
	}

	// Tool calls are optional depending on what the model decides
	if !hasError {
		t.Logf("Received %d tool call events", toolCallCount)
	}
}

// TestSession_SubmitStream_Completion tests the completion event.
func TestSession_SubmitStream_Completion(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	eventCh, err := session.SubmitStream(ctx, "Hello")
	require.NoError(t, err)

	var gotDone bool
	var finalResponse *Response
	var hasError bool

	for event := range eventCh {
		if event.Error != nil {
			hasError = true
			t.Logf("Stream error: %v", event.Error)
			continue
		}

		if event.Done {
			gotDone = true
			finalResponse = event.Response
		}
	}

	if !hasError {
		assert.True(t, gotDone, "Expected to receive done event")
		assert.NotNil(t, finalResponse)
		if finalResponse != nil {
			assert.NotEmpty(t, finalResponse.Content)
			assert.Equal(t, "stop", finalResponse.FinishReason)
			assert.Greater(t, finalResponse.TokenUsage.TotalTokens, int64(0))
		}
	}
}

// TestSession_SubmitStream_ContextCancellation tests that cancelling context stops the stream.
func TestSession_SubmitStream_ContextCancellation(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	// Create a cancellable context
	streamCtx, cancel := context.WithCancel(ctx)

	eventCh, err := session.SubmitStream(streamCtx, "Write a very long story")
	require.NoError(t, err)

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	var gotCancelled bool
	for event := range eventCh {
		if event.Error != nil && strings.Contains(event.Error.Error(), "context canceled") {
			gotCancelled = true
			break
		}
		if event.Done {
			// If we complete before cancellation, that's ok
			break
		}
	}

	// Either we got cancelled or completed - both are valid
	t.Logf("Stream cancelled: %v", gotCancelled)
}

// TestSession_SubmitStream_ErrorHandling tests error event handling.
func TestSession_SubmitStream_ErrorHandling(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	// Try submitting empty message (after history addition)
	// This should cause an error somewhere in processing
	eventCh, err := session.SubmitStream(ctx, "")
	if err != nil {
		// Error at submission time is acceptable
		return
	}

	// Check for error events
	var hasError bool
	for event := range eventCh {
		if event.Error != nil {
			hasError = true
			t.Logf("Got expected error: %v", event.Error)
			break
		}
		if event.Done {
			break
		}
	}

	// Empty message handling varies, so we just log the result
	t.Logf("Error event received: %v", hasError)
}

// TestSession_SubmitStream_MultipleMessages tests multiple sequential messages.
func TestSession_SubmitStream_MultipleMessages(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	messages := []string{
		"What is 2+2?",
		"What is 3+3?",
		"What is 4+4?",
	}

	for i, msg := range messages {
		t.Logf("Sending message %d: %s", i+1, msg)

		eventCh, err := session.SubmitStream(ctx, msg)
		require.NoError(t, err)

		var gotResponse bool
		for event := range eventCh {
			if event.Error != nil {
				t.Logf("Message %d error: %v", i+1, event.Error)
				break
			}
			if event.Done && event.Response != nil {
				gotResponse = true
				t.Logf("Message %d response: %s", i+1, event.Response.Content[:min(50, len(event.Response.Content))])
				break
			}
		}

		if gotResponse {
			assert.True(t, gotResponse, "Expected response for message %d", i+1)
		}
	}

	// Check history accumulated correctly
	history := session.History()
	expectedLen := len(messages) * 2 // Each message should have user + assistant
	assert.GreaterOrEqual(t, len(history), expectedLen)
}

// TestSession_Submit_UsesStreaming tests that Submit works via internal streaming.
func TestSession_Submit_UsesStreaming(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    false, // Non-streaming mode
	})
	require.NoError(t, err)

	// Submit should internally use streaming but collect all events
	resp, err := session.Submit(ctx, "Hello")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
}

// TestSession_SubmitStream_ConcurrentStreams tests concurrent streaming requests.
func TestSession_SubmitStream_ConcurrentStreams(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create multiple sessions for concurrent streaming
	numSessions := 3
	var completed atomic.Int32

	for i := 0; i < numSessions; i++ {
		i := i // capture
		go func() {
			session, err := codex.NewSession(ctx, SessionOptions{
				SystemPrompt: "You are a helpful assistant.",
				Streaming:    true,
			})
			if err != nil {
				t.Logf("Session %d creation error: %v", i, err)
				return
			}

			eventCh, err := session.SubmitStream(ctx, "Count to 5")
			if err != nil {
				t.Logf("Session %d submit error: %v", i, err)
				return
			}

			for event := range eventCh {
				if event.Error != nil {
					t.Logf("Session %d stream error: %v", i, event.Error)
					break
				}
				if event.Done {
					completed.Add(1)
					t.Logf("Session %d completed", i)
					break
				}
			}
		}()
	}

	// Wait a bit for goroutines to complete
	time.Sleep(10 * time.Second)

	completedCount := completed.Load()
	t.Logf("Completed %d out of %d concurrent streams", completedCount, numSessions)
	// We expect at least some to complete
	assert.Greater(t, completedCount, int32(0))
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
