package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/evmts/codex/codex-go/internal/client"
)

// ExampleClient_Stream demonstrates basic streaming usage.
func ExampleClient_Stream() {
	// This example shows the interface usage pattern.
	// Actual implementation will be in a separate package (e.g., internal/client/openai)

	var c client.Client // Assume this is initialized with an implementation

	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewSystemMessage("You are a helpful assistant."),
			client.NewUserMessage("What is the capital of France?"),
		},
		Stream: true,
	}

	ctx := context.Background()
	events, err := c.Stream(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	// Process streaming events
	for event := range events {
		if event.Error != nil {
			fmt.Printf("Error: %v\n", event.Error)
			break
		}

		switch event.Type {
		case client.EventTypeOutputTextDelta:
			if delta, ok := event.Data.(string); ok {
				fmt.Print(delta)
			}
		case client.EventTypeCompleted:
			if completed, ok := event.Data.(*client.CompletedEvent); ok {
				fmt.Printf("\nResponse ID: %s\n", completed.ResponseID)
			}
		}
	}
}

// ExampleClient_Complete demonstrates non-streaming usage.
func ExampleClient_Complete() {
	var c client.Client // Assume this is initialized

	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewUserMessage("Hello!"),
		},
		Stream: false,
	}

	ctx := context.Background()
	resp, err := c.Complete(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("Response: %v\n", resp.Choices[0].Message.Content)
	}
}

// ExampleNewFunctionTool demonstrates creating a function tool.
func ExampleNewFunctionTool() {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"location": {
				"type": "string",
				"description": "City and state, e.g. San Francisco, CA"
			},
			"unit": {
				"type": "string",
				"enum": ["celsius", "fahrenheit"]
			}
		},
		"required": ["location"]
	}`)

	tool := client.NewFunctionTool(
		"get_weather",
		"Get the current weather for a location",
		schema,
	)

	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewUserMessage("What's the weather in Boston?"),
		},
		Tools:  []client.Tool{tool},
		Stream: true,
	}

	_ = req // Use in actual client call
}

// ExampleStreamConfig shows configuring streaming behavior.
func ExampleStreamConfig() {
	config := client.DefaultStreamConfig()

	// Customize for your needs
	config.EnableRawAgentReasoning = true // Stream reasoning tokens
	config.BufferSize = 3200              // Larger buffer for high-volume streams

	_ = config // Use when creating client
}

// ExampleRetryConfig shows configuring retry behavior.
func ExampleRetryConfig() {
	config := client.DefaultRetryConfig()

	// Customize retry strategy
	config.MaxRetries = 5
	config.InitialBackoff = 500 * 1000000 // 500ms in nanoseconds
	config.RespectRetryAfter = true

	_ = config // Use when creating client
}

// ExampleMessage demonstrates different message types.
func ExampleMessage() {
	messages := []client.Message{
		// System message sets behavior
		client.NewSystemMessage("You are a helpful coding assistant."),

		// User messages contain prompts
		client.NewUserMessage("How do I iterate over a map in Go?"),

		// Assistant messages contain responses
		client.NewAssistantMessage("You can use a for-range loop..."),

		// Tool messages contain function results
		client.NewToolMessage("call_123", `{"temperature": 72, "condition": "sunny"}`),
	}

	_ = messages // Use in request
}

// ExampleResponseItem shows the Responses API format.
func ExampleResponseItem() {
	// Responses API uses ResponseItem instead of Message
	items := []client.ResponseItem{
		{
			Type: "message",
			Role: "user",
			Content: []client.ContentItem{
				{
					Type: "input_text",
					Text: "Explain Go channels",
				},
			},
		},
		{
			Type:      "function_call",
			Name:      "search_docs",
			Arguments: `{"query": "go channels"}`,
			CallID:    "call_456",
		},
		{
			Type:   "function_call_output",
			CallID: "call_456",
			Output: map[string]interface{}{
				"content": "Documentation about channels...",
			},
		},
	}

	req := &client.ChatCompletionRequest{
		Model:        "gpt-5",
		Instructions: "You are a Go expert.",
		Input:        items,
		Stream:       true,
	}

	_ = req
}

// ExampleReasoning shows configuring reasoning for capable models.
func ExampleReasoning() {
	effort := "high"
	summary := "auto"

	req := &client.ChatCompletionRequest{
		Model: "gpt-5",
		Messages: []client.Message{
			client.NewUserMessage("Solve this complex math problem..."),
		},
		Reasoning: &client.Reasoning{
			Effort:  &effort,
			Summary: &summary,
		},
		Stream: true,
	}

	_ = req
}

// ExampleTokenUsage shows tracking token consumption.
func ExampleTokenUsage() {
	var c client.Client // Assume initialized

	req := &client.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []client.Message{client.NewUserMessage("Hello")},
		Stream:   true,
	}

	ctx := context.Background()
	events, _ := c.Stream(ctx, req)

	for event := range events {
		if event.Type == client.EventTypeCompleted {
			if completed, ok := event.Data.(*client.CompletedEvent); ok {
				usage := completed.TokenUsage
				fmt.Printf("Input tokens: %d\n", usage.InputTokens)
				fmt.Printf("Cached tokens: %d\n", usage.CachedInputTokens)
				fmt.Printf("Output tokens: %d\n", usage.OutputTokens)
				fmt.Printf("Reasoning tokens: %d\n", usage.ReasoningOutputTokens)
				fmt.Printf("Total tokens: %d\n", usage.TotalTokens)
			}
		}
	}
}
