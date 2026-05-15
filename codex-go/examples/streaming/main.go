package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/evmts/codex/codex-go/pkg/sdk"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable not set")
	}

	// Create client
	apiClient, err := client.NewAnthropicClient(apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create SDK
	codex, err := sdk.New(sdk.Options{
		Client: apiClient,
	})
	if err != nil {
		log.Fatalf("Failed to create SDK: %v", err)
	}
	defer codex.Close()

	// Run both examples
	fmt.Println("=== Non-Streaming Example ===")
	nonStreamingExample(codex)

	fmt.Println("\n=== Streaming Example ===")
	streamingExample(codex)
}

func nonStreamingExample(codex *sdk.SDK) {
	ctx := context.Background()

	// Create non-streaming session
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful coding assistant. Be concise.",
		Streaming:    false,
		Model:        "claude-sonnet-4-5",
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Submit message and wait for complete response
	response, err := session.Submit(ctx, "Write a simple hello world in Go")
	if err != nil {
		log.Fatalf("Failed to submit: %v", err)
	}

	fmt.Printf("Response:\n%s\n", response.Content)
	fmt.Printf("\nTokens used: %d input + %d output = %d total\n",
		response.TokenUsage.InputTokens,
		response.TokenUsage.OutputTokens,
		response.TokenUsage.TotalTokens)
}

func streamingExample(codex *sdk.SDK) {
	ctx := context.Background()

	// Create streaming session
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful coding assistant. Be concise.",
		Streaming:    true,
		Model:        "claude-sonnet-4-5",
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Submit message and get streaming channel
	eventCh, err := session.SubmitStream(ctx, "Write a simple hello world in Go")
	if err != nil {
		log.Fatalf("Failed to submit stream: %v", err)
	}

	fmt.Println("Response (streaming):")

	// Process streaming events
	var contentBuilder string
	for event := range eventCh {
		// Check for errors
		if event.Error != nil {
			log.Printf("Stream error: %v", event.Error)
			continue
		}

		// Handle different event types
		switch event.Type {
		case "content_delta":
			// Print content as it arrives
			fmt.Print(event.Delta)
			contentBuilder += event.Delta

		case "reasoning_delta":
			// Show AI's reasoning (if available)
			log.Printf("[Thinking] %s", event.Delta)

		case "tool_call_delta":
			// Show tool execution
			if event.ToolCall != nil {
				log.Printf("[Tool] %s: %s", event.ToolCall.Name, event.ToolCall.ID)
			}

		case "done":
			// Final response with statistics
			if event.Response != nil {
				fmt.Printf("\n\n---\n")
				fmt.Printf("Tokens used: %d input + %d output = %d total\n",
					event.Response.TokenUsage.InputTokens,
					event.Response.TokenUsage.OutputTokens,
					event.Response.TokenUsage.TotalTokens)
				fmt.Printf("Finish reason: %s\n", event.Response.FinishReason)
			}
		}
	}
}
