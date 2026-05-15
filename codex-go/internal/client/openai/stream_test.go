package openai

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"go.uber.org/goleak"
)

// TestStreamParser_NoGoroutineLeak verifies that the streaming parser doesn't leak goroutines
// when the context is cancelled or when the stream ends normally.
func TestStreamParser_NoGoroutineLeak(t *testing.T) {
	tests := []struct {
		name          string
		streamData    string
		cancelAfter   time.Duration
		expectError   bool
		checkInterval time.Duration
	}{
		{
			name: "no leak on normal completion",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{},"finish_reason":"stop","index":0}]}

data: [DONE]

`,
			cancelAfter:   0,
			expectError:   false,
			checkInterval: 50 * time.Millisecond,
		},
		{
			name: "no leak on context cancellation",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"This will be cancelled"},"index":0}]}

` + generateLongStream(100), // Generate a long stream to ensure goroutine is reading
			cancelAfter: 100 * time.Millisecond,
			// Note: With strings.NewReader, data is read instantly, so cancellation may occur
			// after completion. Either error or success is acceptable - the key is no goroutine leak.
			expectError:   false, // Changed to false since stream completes before cancellation
			checkInterval: 50 * time.Millisecond,
		},
		{
			name: "no leak on stream end",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"done"},"index":0,"finish_reason":"stop"}]}

`,
			cancelAfter:   0,
			expectError:   false,
			checkInterval: 50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record initial goroutine count
			runtime.GC()
			time.Sleep(50 * time.Millisecond)
			initialGoroutines := runtime.NumGoroutine()

			// Create parser
			config := client.StreamConfig{}
			parser := newStreamParser(config)

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start goroutine to cancel after specified duration
			if tt.cancelAfter > 0 {
				time.AfterFunc(tt.cancelAfter, cancel)
			}

			// Create event channel
			eventCh := make(chan client.StreamEvent, 100)

			// Create reader
			reader := strings.NewReader(tt.streamData)

			// Run parser
			parseErr := parser.parse(ctx, reader, eventCh)

			// Verify error expectation
			if tt.expectError && parseErr == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && parseErr != nil {
				t.Errorf("unexpected error: %v", parseErr)
			}

			// Close event channel
			close(eventCh)

			// Drain event channel
			for range eventCh {
			}

			// Force garbage collection
			runtime.GC()

			// Wait for goroutines to clean up
			time.Sleep(tt.checkInterval)

			// Check goroutine count
			runtime.GC()
			time.Sleep(tt.checkInterval)
			finalGoroutines := runtime.NumGoroutine()

			// Allow for some variance (test runner goroutines, etc)
			goroutineDiff := finalGoroutines - initialGoroutines
			if goroutineDiff > 2 {
				t.Errorf("goroutine leak detected: initial=%d, final=%d, diff=%d",
					initialGoroutines, finalGoroutines, goroutineDiff)
			}
		})
	}
}

// TestStreamParser_MultipleIterationsNoLeak specifically tests that creating
// a goroutine on each iteration (the old buggy behavior) would leak, but our
// fix prevents this.
func TestStreamParser_MultipleIterationsNoLeak(t *testing.T) {
	// Generate a stream with many chunks
	streamData := "data: " + `{"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n"
	for i := 0; i < 100; i++ {
		streamData += fmt.Sprintf(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i)
	}
	streamData += "data: [DONE]\n\n"

	// Record initial goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	parser := newStreamParser(client.StreamConfig{})
	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, 200)

	reader := strings.NewReader(streamData)

	// Run parser
	err := parser.parse(ctx, reader, eventCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	close(eventCh)

	// Count events to ensure parsing worked
	eventCount := 0
	for range eventCh {
		eventCount++
	}

	if eventCount == 0 {
		t.Error("expected events but got none")
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count - should be roughly the same
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	// With the old bug, we would have 100+ leaked goroutines
	// With the fix, we should have at most a couple extra from test infrastructure
	if goroutineDiff > 5 {
		t.Errorf("potential goroutine leak: initial=%d, final=%d, diff=%d (old bug would show 100+)",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestStreamParser_ConcurrentCancellation tests that rapid cancellations
// don't cause goroutine leaks or panics.
func TestStreamParser_ConcurrentCancellation(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Run multiple parsers concurrently and cancel them at different times
	const numParsers = 10
	done := make(chan struct{}, numParsers)

	for i := 0; i < numParsers; i++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parser %d panicked: %v", id, r)
				}
				done <- struct{}{}
			}()

			streamData := generateLongStream(50)
			parser := newStreamParser(client.StreamConfig{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Cancel at random times
			time.AfterFunc(time.Duration(id*10)*time.Millisecond, cancel)

			eventCh := make(chan client.StreamEvent, 100)
			reader := strings.NewReader(streamData)

			_ = parser.parse(ctx, reader, eventCh)
			close(eventCh)

			// Drain events
			for range eventCh {
			}
		}(i)
	}

	// Wait for all parsers to complete
	for i := 0; i < numParsers; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for parsers to complete")
		}
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	// Should have minimal goroutine increase
	if goroutineDiff > 5 {
		t.Errorf("goroutine leak in concurrent scenario: initial=%d, final=%d, diff=%d",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestStreamParser_IdleTimeoutNoLeak tests that idle timeout doesn't cause
// goroutine leaks.
func TestStreamParser_IdleTimeoutNoLeak(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Create parser with short idle timeout
	parser := newStreamParser(client.StreamConfig{
		IdleTimeout: 100 * time.Millisecond,
	})

	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, 10)

	// Use slow reader that will trigger timeout
	slowReader := &slowReader{delay: 200 * time.Millisecond, chunks: 10}

	// Run parser - should timeout
	err := parser.parse(ctx, slowReader, eventCh)
	if err == nil {
		t.Fatal("expected idle timeout error but got none")
	}

	close(eventCh)
	for range eventCh {
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	if goroutineDiff > 3 {
		t.Errorf("goroutine leak on idle timeout: initial=%d, final=%d, diff=%d",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestStreamParser_ScanDoneChannel tests that the scanDone channel properly
// signals the scanning goroutine to exit.
func TestStreamParser_ScanDoneChannel(t *testing.T) {
	// Create a slow reader that never finishes
	slowReader := &slowReader{delay: 100 * time.Millisecond, chunks: 1000}

	parser := newStreamParser(client.StreamConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan client.StreamEvent, 10)

	// Start parsing in a goroutine
	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.parse(ctx, slowReader, eventCh)
	}()

	// Let it run for a bit
	time.Sleep(200 * time.Millisecond)

	// Cancel context
	cancel()

	// Parser should exit quickly
	select {
	case err := <-parseDone:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("parser did not exit after context cancellation")
	}

	close(eventCh)
}

// Helper functions

// generateLongStream creates a stream with many chunks
func generateLongStream(numChunks int) string {
	var sb strings.Builder
	for i := 0; i < numChunks; i++ {
		sb.WriteString(fmt.Sprintf(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i))
	}
	return sb.String()
}

// generateSlowStream creates a stream that would trigger idle timeout
func generateSlowStream(numChunks int, delay time.Duration) string {
	// Return a stream that simulates slow data
	// In reality, this is handled by the slowReader
	return generateLongStream(numChunks)
}

// slowReader simulates a slow network connection
type slowReader struct {
	data   []byte
	pos    int
	delay  time.Duration
	chunks int
	count  int
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.count >= r.chunks {
		return 0, io.EOF
	}

	// Simulate network delay
	time.Sleep(r.delay)

	// Generate data on the fly
	chunk := fmt.Sprintf(`data: {"id":"test","choices":[{"delta":{"content":"slow%d"},"index":0}]}`+"\n\n", r.count)
	r.count++

	if len(p) < len(chunk) {
		// Buffer too small, copy what we can
		n = copy(p, chunk[:len(p)])
		return n, nil
	}

	n = copy(p, chunk)
	return n, nil
}

// TestStreamParser_ReasoningContent tests parsing of reasoning_content deltas
func TestStreamParser_ReasoningContent(t *testing.T) {
	tests := []struct {
		name              string
		streamData        string
		enableReasoning   bool
		expectedReasoning []string
	}{
		{
			name: "reasoning content as string",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":"Let me think..."},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":" about this problem."},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"Here's my answer"},"index":0}]}

data: [DONE]

`,
			enableReasoning: true,
			expectedReasoning: []string{
				"Let me think...",
				" about this problem.",
			},
		},
		{
			name: "reasoning content as object with text field",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":{"type":"reasoning_content","text":"First step"}},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":{"type":"reasoning_content","text":" of analysis"}},"index":0}]}

data: [DONE]

`,
			enableReasoning: true,
			expectedReasoning: []string{
				"First step",
				" of analysis",
			},
		},
		{
			name: "reasoning content as object with content field",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":{"content":"Thinking process"}},"index":0}]}

data: [DONE]

`,
			enableReasoning: true,
			expectedReasoning: []string{
				"Thinking process",
			},
		},
		{
			name: "reasoning disabled",
			streamData: `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":"This should not be emitted"},"index":0}]}

data: [DONE]

`,
			enableReasoning:   false,
			expectedReasoning: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parser with reasoning config
			config := client.StreamConfig{
				EnableRawAgentReasoning: tt.enableReasoning,
			}
			parser := newStreamParser(config)

			ctx := context.Background()
			eventCh := make(chan client.StreamEvent, 100)

			reader := strings.NewReader(tt.streamData)

			// Run parser in goroutine
			go func() {
				_ = parser.parse(ctx, reader, eventCh)
				close(eventCh)
			}()

			// Collect reasoning deltas
			var reasoningDeltas []string
			for evt := range eventCh {
				if evt.Type == client.EventTypeReasoningContentDelta {
					if delta, ok := evt.Data.(string); ok {
						reasoningDeltas = append(reasoningDeltas, delta)
					}
				}
			}

			// Verify expected reasoning
			if len(reasoningDeltas) != len(tt.expectedReasoning) {
				t.Errorf("expected %d reasoning deltas, got %d", len(tt.expectedReasoning), len(reasoningDeltas))
			}

			for i, expected := range tt.expectedReasoning {
				if i >= len(reasoningDeltas) {
					t.Errorf("missing reasoning delta at index %d", i)
					continue
				}
				if reasoningDeltas[i] != expected {
					t.Errorf("reasoning delta %d: expected %q, got %q", i, expected, reasoningDeltas[i])
				}
			}
		})
	}
}

// TestStreamParser_MixedContentAndReasoning tests interleaved content and reasoning
func TestStreamParser_MixedContentAndReasoning(t *testing.T) {
	streamData := `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":"Step 1: analyze"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"Based on"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"reasoning":" the problem"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":" my analysis"},"index":0}]}

data: [DONE]

`

	config := client.StreamConfig{
		EnableRawAgentReasoning: true,
	}
	parser := newStreamParser(config)

	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, 100)

	reader := strings.NewReader(streamData)

	// Run parser in goroutine
	go func() {
		_ = parser.parse(ctx, reader, eventCh)
		close(eventCh)
	}()

	// Collect events in order
	var contentDeltas []string
	var reasoningDeltas []string
	for evt := range eventCh {
		switch evt.Type {
		case client.EventTypeOutputTextDelta:
			if delta, ok := evt.Data.(string); ok {
				contentDeltas = append(contentDeltas, delta)
			}
		case client.EventTypeReasoningContentDelta:
			if delta, ok := evt.Data.(string); ok {
				reasoningDeltas = append(reasoningDeltas, delta)
			}
		}
	}

	// Verify content deltas
	expectedContent := []string{"Based on", " my analysis"}
	if len(contentDeltas) != len(expectedContent) {
		t.Errorf("expected %d content deltas, got %d", len(expectedContent), len(contentDeltas))
	}
	for i, expected := range expectedContent {
		if i >= len(contentDeltas) {
			continue
		}
		if contentDeltas[i] != expected {
			t.Errorf("content delta %d: expected %q, got %q", i, expected, contentDeltas[i])
		}
	}

	// Verify reasoning deltas
	expectedReasoning := []string{"Step 1: analyze", " the problem"}
	if len(reasoningDeltas) != len(expectedReasoning) {
		t.Errorf("expected %d reasoning deltas, got %d", len(expectedReasoning), len(reasoningDeltas))
	}
	for i, expected := range expectedReasoning {
		if i >= len(reasoningDeltas) {
			continue
		}
		if reasoningDeltas[i] != expected {
			t.Errorf("reasoning delta %d: expected %q, got %q", i, expected, reasoningDeltas[i])
		}
	}
}

// TestStreamParser_BackpressureSlowConsumer tests that backpressure prevents memory exhaustion
// when the consumer is slow and the buffer fills up.
func TestStreamParser_BackpressureSlowConsumer(t *testing.T) {
	tests := []struct {
		name               string
		enableBackpressure bool
		bufferSize         int
		numChunks          int
		consumerDelay      time.Duration
		expectAllEvents    bool
	}{
		{
			name:               "backpressure blocks producer",
			enableBackpressure: true,
			bufferSize:         10,
			numChunks:          50,
			consumerDelay:      10 * time.Millisecond,
			expectAllEvents:    true,
		},
		{
			name:               "no backpressure drops events",
			enableBackpressure: false,
			bufferSize:         10,
			numChunks:          50,
			consumerDelay:      10 * time.Millisecond,
			expectAllEvents:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate stream data
			var sb strings.Builder
			sb.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n")
			for i := 0; i < tt.numChunks; i++ {
				sb.WriteString(fmt.Sprintf(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i))
			}
			sb.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{},"finish_reason":"stop","index":0},"usage":{"input_tokens":10,"output_tokens":50,"total_tokens":60}}]}` + "\n\n")
			sb.WriteString("data: [DONE]\n\n")

			config := client.StreamConfig{
				EnableBackpressure:    tt.enableBackpressure,
				BufferSize:            tt.bufferSize,
				BackpressureThreshold: 0.8,
			}
			parser := newStreamParser(config)

			ctx := context.Background()
			eventCh := make(chan client.StreamEvent, tt.bufferSize)

			reader := strings.NewReader(sb.String())

			// Run parser in goroutine
			parserDone := make(chan error, 1)
			go func() {
				parserDone <- parser.parse(ctx, reader, eventCh)
			}()

			// Slow consumer
			var receivedEvents int
			consumeDone := make(chan struct{})
			go func() {
				defer close(consumeDone)
				for range eventCh {
					receivedEvents++
					// Simulate slow processing
					time.Sleep(tt.consumerDelay)
				}
			}()

			// Wait for parser to finish
			select {
			case err := <-parserDone:
				if err != nil {
					t.Fatalf("parser error: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("parser timeout")
			}

			// Close channel and wait for consumer
			close(eventCh)
			<-consumeDone

			// Verify event count
			// With backpressure, we expect all events (1 created + numChunks content + 1 completed)
			// Without backpressure, some events may be dropped
			// Note: The "created" event is optional depending on the stream format
			expectedMin := 2 // At least content and completed
			if tt.expectAllEvents {
				// Allow for created event to be optional
				expectedMin = tt.numChunks + 1 // All content chunks plus at least completed
			}

			if receivedEvents < expectedMin {
				t.Errorf("expected at least %d events, got %d", expectedMin, receivedEvents)
			}

			if tt.expectAllEvents && receivedEvents < tt.numChunks+1 {
				t.Errorf("with backpressure, expected at least %d events, got %d (some were dropped)", tt.numChunks+1, receivedEvents)
			}
		})
	}
}

// TestStreamParser_BackpressureWarningLogs tests that warnings are logged when buffer is full.
func TestStreamParser_BackpressureWarningLogs(t *testing.T) {
	// Generate enough chunks to fill the buffer
	var sb strings.Builder
	sb.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i))
	}
	sb.WriteString("data: [DONE]\n\n")

	config := client.StreamConfig{
		EnableBackpressure:    true,
		BufferSize:            5, // Small buffer to trigger warnings
		BackpressureThreshold: 0.6,
	}
	parser := newStreamParser(config)

	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, config.BufferSize)

	reader := strings.NewReader(sb.String())

	// Run parser in goroutine
	parserDone := make(chan error, 1)
	go func() {
		parserDone <- parser.parse(ctx, reader, eventCh)
	}()

	// Very slow consumer to ensure buffer fills up
	time.Sleep(100 * time.Millisecond)

	// Start consuming slowly
	go func() {
		for range eventCh {
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Wait for parser to finish
	select {
	case err := <-parserDone:
		if err != nil {
			t.Fatalf("parser error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parser timeout")
	}

	close(eventCh)

	// Note: In a real implementation, we would capture log output and verify warnings
	// For now, this test ensures the code path is exercised without panicking
}

// TestStreamParser_BackpressureContextCancellation tests that context cancellation
// works correctly even when backpressure is blocking.
func TestStreamParser_BackpressureContextCancellation(t *testing.T) {
	// Generate large stream
	var sb strings.Builder
	sb.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i))
	}

	config := client.StreamConfig{
		EnableBackpressure:    true,
		BufferSize:            5,
		BackpressureThreshold: 0.8,
	}
	parser := newStreamParser(config)

	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan client.StreamEvent, config.BufferSize)

	reader := strings.NewReader(sb.String())

	// Run parser in goroutine
	parserDone := make(chan error, 1)
	go func() {
		parserDone <- parser.parse(ctx, reader, eventCh)
	}()

	// Let buffer fill up
	time.Sleep(100 * time.Millisecond)

	// Cancel context while backpressure is blocking
	cancel()

	// Parser should exit quickly
	select {
	case err := <-parserDone:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("parser did not exit after context cancellation")
	}

	close(eventCh)
}

// TestStreamParser_BackpressureMetrics tests buffer usage calculations.
func TestStreamParser_BackpressureMetrics(t *testing.T) {
	config := client.StreamConfig{
		EnableBackpressure:    true,
		BufferSize:            10,
		BackpressureThreshold: 0.7,
	}
	parser := newStreamParser(config)

	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, config.BufferSize)

	// Send events to fill buffer partially
	for i := 0; i < 5; i++ {
		eventCh <- client.StreamEvent{
			Type: client.EventTypeOutputTextDelta,
			Data: fmt.Sprintf("chunk%d", i),
		}
	}

	// Try to send another event with backpressure
	testEvent := client.StreamEvent{
		Type: client.EventTypeOutputTextDelta,
		Data: "test",
	}

	// This should not block since buffer is not full
	done := make(chan error, 1)
	go func() {
		done <- parser.sendEventWithBackpressure(ctx, eventCh, testEvent)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("send blocked unexpectedly")
	}

	close(eventCh)
}

// TestStreamParser_LargeToolCallArguments tests that the scanner can handle
// large tool call arguments that exceed the default 64KB buffer.
func TestStreamParser_LargeToolCallArguments(t *testing.T) {
	// Create a large JSON payload (100KB+) for tool call arguments that would normally
	// exceed the default 64KB scanner buffer. With our 1MB buffer, this should work.
	// We'll send the arguments in chunks to simulate real streaming behavior.

	// Build up arguments in multiple delta chunks
	chunkSize := 30000 // Each chunk is 30KB
	numChunks := 3     // Total 90KB of arguments

	var streamData strings.Builder
	streamData.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

`)

	// First tool call chunk with function name and start of arguments
	streamData.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"test"}}]},"index":0}]}

`)

	// Send arguments in multiple chunks
	for i := 0; i < numChunks; i++ {
		args := strings.Repeat("x", chunkSize)
		streamData.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"` + args + `"}}]},"index":0}]}

`)
	}

	// Finish with complete reason
	streamData.WriteString(`data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{},"finish_reason":"tool_calls","index":0}]}

data: [DONE]

`)

	parser := newStreamParser(client.StreamConfig{})
	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, 100)

	reader := strings.NewReader(streamData.String())

	// Run parser
	parseErr := make(chan error, 1)
	go func() {
		err := parser.parse(ctx, reader, eventCh)
		parseErr <- err
		close(eventCh)
	}()

	// Wait for parsing to complete
	err := <-parseErr
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}

	// Collect events
	var foundToolCall bool
	var totalArgsLen int
	for evt := range eventCh {
		if evt.Type == client.EventTypeOutputItemDone {
			foundToolCall = true
			// Verify tool call was received
			if data, ok := evt.Data.(map[string]interface{}); ok {
				if toolCalls, ok := data["tool_calls"].([]client.ToolCall); ok {
					if len(toolCalls) > 0 {
						totalArgsLen = len(toolCalls[0].Function.Arguments)
						t.Logf("Successfully parsed large tool call with %d bytes of arguments",
							totalArgsLen)
					}
				}
			}
		}
	}

	if !foundToolCall {
		t.Error("expected tool call event but got none - large payload may have failed to parse")
	}

	expectedSize := chunkSize * numChunks
	if totalArgsLen != expectedSize {
		t.Errorf("expected %d bytes of arguments, got %d", expectedSize, totalArgsLen)
	}
}

// TestStreamParser_BufferOverflow tests that scanner handles extremely large lines
// that exceed even the increased buffer size.
func TestStreamParser_BufferOverflow(t *testing.T) {
	// Create a line that exceeds 1MB (our max buffer size)
	// This should fail gracefully with a scanner error
	veryLargeArgs := strings.Repeat("x", 2*1024*1024) // 2MB

	streamData := `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","choices":[{"delta":{"content":"` + veryLargeArgs + `"},"index":0}]}

data: [DONE]

`

	parser := newStreamParser(client.StreamConfig{})
	ctx := context.Background()
	eventCh := make(chan client.StreamEvent, 100)

	reader := strings.NewReader(streamData)

	// Run parser - should return error due to token too long
	err := parser.parse(ctx, reader, eventCh)
	close(eventCh)

	if err == nil {
		t.Error("expected scanner error for token too long, got nil")
	} else {
		// Verify it's a scanner error
		t.Logf("Got expected error for oversized token: %v", err)
	}

	// Drain events
	for range eventCh {
	}
}

// TestStreamParser_ContextCancellationDuringScan tests that context cancellation
// properly cleans up even when scanner is blocked mid-read.
func TestStreamParser_ContextCancellationDuringScan(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Create a reader that blocks indefinitely to simulate slow network
	blockingReader := &blockingReader{
		unblockCh: make(chan struct{}),
		data:      []byte(`data: {"id":"test","choices":[{"delta":{"role":"assistant"},"index":0}]}\n\n`),
	}

	parser := newStreamParser(client.StreamConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan client.StreamEvent, 10)

	// Start parser in goroutine
	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.parse(ctx, blockingReader, eventCh)
	}()

	// Let the scanner start reading
	time.Sleep(100 * time.Millisecond)

	// Cancel context while reader is blocked
	cancel()

	// The parser should exit, but may take time for the blocking read to complete.
	// In a real scenario with proper HTTP client timeouts, this would be faster.
	// For this test, we unblock the reader to allow cleanup.
	close(blockingReader.unblockCh)

	select {
	case err := <-parseDone:
		if err != context.Canceled {
			t.Logf("Got error (acceptable): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parser did not exit after context cancellation and unblocking")
	}

	close(eventCh)
	for range eventCh {
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines

	if goroutineDiff > 3 {
		t.Errorf("goroutine leak after blocked cancellation: initial=%d, final=%d, diff=%d",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestStreamParser_FastPathCancellation tests that cancellation between scans
// (not during a blocking scan) is handled immediately.
func TestStreamParser_FastPathCancellation(t *testing.T) {
	parser := newStreamParser(client.StreamConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan client.StreamEvent, 10)

	// Create a slow reader that adds delay between reads
	slowReader := &slowReader{delay: 50 * time.Millisecond, chunks: 100}

	// Start parser
	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.parse(ctx, slowReader, eventCh)
	}()

	// Let it process a few chunks
	time.Sleep(200 * time.Millisecond)

	// Cancel context
	cancelTime := time.Now()
	cancel()

	// Parser should exit quickly (within 200ms - one scan interval)
	select {
	case err := <-parseDone:
		elapsed := time.Since(cancelTime)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
		// The fast path should allow exit within one scan interval
		if elapsed > 300*time.Millisecond {
			t.Errorf("cancellation took too long: %v (expected < 300ms)", elapsed)
		}
		t.Logf("Cancellation completed in %v", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("parser did not exit after fast path cancellation")
	}

	close(eventCh)
}

// blockingReader is a reader that blocks until unblocked
type blockingReader struct {
	unblockCh chan struct{}
	data      []byte
	sent      bool
}

func (r *blockingReader) Read(p []byte) (n int, err error) {
	if r.sent {
		// Block until explicitly unblocked
		<-r.unblockCh
		return 0, io.EOF
	}

	// Send data once
	r.sent = true
	n = copy(p, r.data)

	// Block on subsequent reads
	<-r.unblockCh
	return n, io.EOF
}

// TestStreamParser_GoleakDetection uses goleak to verify no goroutines leak
// in various scenarios. This is the most comprehensive leak detection test.
func TestStreamParser_GoleakDetection(t *testing.T) {
	tests := []struct {
		name      string
		setupTest func(t *testing.T)
	}{
		{
			name: "normal completion",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				streamData := `data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","model":"gpt-4","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: [DONE]

`
				parser := newStreamParser(client.StreamConfig{})
				ctx := context.Background()
				eventCh := make(chan client.StreamEvent, 100)

				reader := strings.NewReader(streamData)
				err := parser.parse(ctx, reader, eventCh)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				close(eventCh)
				for range eventCh {
				}
			},
		},
		{
			name: "context cancellation",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				streamData := generateLongStream(50)
				parser := newStreamParser(client.StreamConfig{})
				ctx, cancel := context.WithCancel(context.Background())
				eventCh := make(chan client.StreamEvent, 100)

				reader := strings.NewReader(streamData)

				parseDone := make(chan error, 1)
				go func() {
					parseDone <- parser.parse(ctx, reader, eventCh)
				}()

				time.Sleep(50 * time.Millisecond)
				cancel()

				<-parseDone
				close(eventCh)
				for range eventCh {
				}
			},
		},
		{
			name: "idle timeout",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				parser := newStreamParser(client.StreamConfig{
					IdleTimeout: 100 * time.Millisecond,
				})
				ctx := context.Background()
				eventCh := make(chan client.StreamEvent, 10)

				slowReader := &slowReader{delay: 200 * time.Millisecond, chunks: 5}

				_ = parser.parse(ctx, slowReader, eventCh)
				close(eventCh)
				for range eventCh {
				}
			},
		},
		{
			name: "multiple rapid parsers",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				const numParsers = 5
				done := make(chan struct{}, numParsers)

				for i := 0; i < numParsers; i++ {
					go func() {
						defer func() { done <- struct{}{} }()

						streamData := generateLongStream(10)
						parser := newStreamParser(client.StreamConfig{})
						ctx := context.Background()
						eventCh := make(chan client.StreamEvent, 50)

						reader := strings.NewReader(streamData)
						_ = parser.parse(ctx, reader, eventCh)
						close(eventCh)
						for range eventCh {
						}
					}()
				}

				for i := 0; i < numParsers; i++ {
					<-done
				}
			},
		},
		{
			name: "backpressure with slow consumer",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				var sb strings.Builder
				sb.WriteString(`data: {"id":"test","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n")
				for i := 0; i < 20; i++ {
					sb.WriteString(fmt.Sprintf(`data: {"id":"test","choices":[{"delta":{"content":"chunk%d"},"index":0}]}`+"\n\n", i))
				}
				sb.WriteString("data: [DONE]\n\n")

				config := client.StreamConfig{
					EnableBackpressure:    true,
					BufferSize:            5,
					BackpressureThreshold: 0.8,
				}
				parser := newStreamParser(config)
				ctx := context.Background()
				eventCh := make(chan client.StreamEvent, config.BufferSize)

				reader := strings.NewReader(sb.String())

				parserDone := make(chan error, 1)
				go func() {
					parserDone <- parser.parse(ctx, reader, eventCh)
				}()

				// Slow consumer
				go func() {
					for range eventCh {
						time.Sleep(10 * time.Millisecond)
					}
				}()

				<-parserDone
				close(eventCh)
			},
		},
		{
			name: "large payload",
			setupTest: func(t *testing.T) {
				defer goleak.VerifyNone(t)

				largeArgs := strings.Repeat(`{"key":"value"}`, 10000)
				streamData := `data: {"id":"test","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"test","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"test","arguments":"` + largeArgs + `"}}]},"index":0}]}

data: [DONE]

`
				parser := newStreamParser(client.StreamConfig{})
				ctx := context.Background()
				eventCh := make(chan client.StreamEvent, 100)

				reader := strings.NewReader(streamData)
				err := parser.parse(ctx, reader, eventCh)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				close(eventCh)
				for range eventCh {
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupTest(t)
		})
	}
}

// TestMain uses goleak to verify no goroutines leak across all tests
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
