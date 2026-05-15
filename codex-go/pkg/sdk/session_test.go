package sdk

import (
	"testing"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_Submit(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create a non-streaming session
	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    false,
	})
	require.NoError(t, err)

	// Submit a message
	resp, err := session.Submit(ctx, "Hello, how are you?")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Greater(t, resp.TokenUsage.TotalTokens, int64(0))

	// Check history
	history := session.History()
	assert.Len(t, history, 2) // user message + assistant response
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "Hello, how are you?", history[0].Content)
	assert.Equal(t, "assistant", history[1].Role)
}

func TestSession_SubmitStream(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create a streaming session
	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})
	require.NoError(t, err)

	// Submit a message and get stream
	eventCh, err := session.SubmitStream(ctx, "Hello, how are you?")
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	// Collect stream events
	var events []StreamEvent
	var finalResponse *Response

	for event := range eventCh {
		events = append(events, event)
		if event.Done {
			finalResponse = event.Response
		}
		if event.Error != nil {
			t.Fatalf("Stream error: %v", event.Error)
		}
	}

	// Verify we got events
	assert.NotEmpty(t, events)
	assert.NotNil(t, finalResponse)
	assert.NotEmpty(t, finalResponse.Content)

	// Check history
	history := session.History()
	assert.Len(t, history, 2) // user message + assistant response
}

func TestSession_SubmitError_WrongMode(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	t.Run("Submit on streaming session", func(t *testing.T) {
		session, err := codex.NewSession(ctx, SessionOptions{
			SystemPrompt: "You are a helpful assistant.",
			Streaming:    true,
		})
		require.NoError(t, err)

		// Try to use Submit on a streaming session
		_, err = session.Submit(ctx, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "streaming")
	})

	t.Run("SubmitStream on non-streaming session", func(t *testing.T) {
		session, err := codex.NewSession(ctx, SessionOptions{
			SystemPrompt: "You are a helpful assistant.",
			Streaming:    false,
		})
		require.NoError(t, err)

		// Try to use SubmitStream on a non-streaming session
		_, err = session.SubmitStream(ctx, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured for streaming")
	})
}

func TestSession_SubmitError_Closed(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    false,
	})
	require.NoError(t, err)

	// Close the session
	err = session.close()
	require.NoError(t, err)

	// Try to submit
	_, err = session.Submit(ctx, "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestSession_History(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    false,
	})
	require.NoError(t, err)

	// Initially empty
	history := session.History()
	assert.Empty(t, history)

	// Submit a message
	_, err = session.Submit(ctx, "First message")
	require.NoError(t, err)

	history = session.History()
	assert.Len(t, history, 2)

	// Submit another message
	_, err = session.Submit(ctx, "Second message")
	require.NoError(t, err)

	history = session.History()
	assert.Len(t, history, 4)

	// Verify history is immutable (returns a copy)
	history[0].Content = "Modified"
	history2 := session.History()
	assert.NotEqual(t, "Modified", history2[0].Content)
}

func TestSession_IsClosed(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	require.NoError(t, err)

	// Initially not closed
	assert.False(t, session.IsClosed())

	// Close it
	err = session.close()
	require.NoError(t, err)

	// Now closed
	assert.True(t, session.IsClosed())

	// Try to close again
	err = session.close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already closed")
}
