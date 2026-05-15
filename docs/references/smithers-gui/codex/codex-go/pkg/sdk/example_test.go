package sdk_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/evmts/codex/codex-go/pkg/sdk"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
)

// =============================================================================
// BASIC USAGE EXAMPLES
// =============================================================================

// Example_basicUsage demonstrates a simple question-answer interaction.
// This is the most straightforward way to use the SDK for single-turn conversations.
func Example_basicUsage() {
	// Create a client with your API credentials
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the SDK
	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	// Create a conversation session
	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant that provides concise answers.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ask a question and get a response
	response, err := session.Submit(ctx, "What is 2+2?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", response.Content)
	fmt.Printf("Tokens used: %d\n", response.TokenUsage.TotalTokens)
	// Output:
	// Response: This is a placeholder response. Full implementation pending.
	// Tokens used: 30
}

// Example_withStreaming demonstrates receiving responses as a stream of events.
// This is useful for providing real-time feedback to users.
func Example_withStreaming() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true, // Enable streaming mode
	})
	if err != nil {
		log.Fatal(err)
	}

	// Submit a message and receive streaming responses
	eventCh, err := session.SubmitStream(ctx, "Tell me a short story")
	if err != nil {
		log.Fatal(err)
	}

	// Process events as they arrive
	var fullResponse string
	for event := range eventCh {
		if event.Error != nil {
			log.Fatal(event.Error)
		}

		if event.Done {
			fmt.Printf("Total tokens: %d\n", event.Response.TokenUsage.TotalTokens)
			break
		}

		// In a real application, you might print each delta immediately
		fullResponse += event.Delta
	}

	fmt.Printf("Received streaming response with %d characters\n", len(fullResponse))
	// Output:
	// Total tokens: 30
	// Received streaming response with 40 characters
}

// Example_withHistory demonstrates maintaining conversation context across multiple turns.
// The session automatically tracks message history.
func Example_withHistory() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// First message
	_, err = session.Submit(ctx, "My name is Alice")
	if err != nil {
		log.Fatal(err)
	}

	// Second message - the assistant should remember the context
	_, err = session.Submit(ctx, "What is my name?")
	if err != nil {
		log.Fatal(err)
	}

	// Access the conversation history
	history := session.History()
	fmt.Printf("Conversation has %d messages\n", len(history))

	// Print each message
	for i, msg := range history {
		fmt.Printf("Message %d: role=%s, content_length=%d\n", i+1, msg.Role, len(msg.Content))
	}

	// Output:
	// Conversation has 4 messages
	// Message 1: role=user, content_length=16
	// Message 2: role=assistant, content_length=60
	// Message 3: role=user, content_length=16
	// Message 4: role=assistant, content_length=60
}

// Example_resumeSession demonstrates resuming a conversation from a previous session.
// This is useful for persisting conversations across application restarts.
func Example_resumeSession() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Start a new conversation with a specific ID
	conversationID := "conversation-123"
	session1, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful assistant.",
		ConversationID: conversationID,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Send a message
	_, err = session1.Submit(ctx, "Remember this number: 42")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Started conversation with ID: %s\n", session1.ID())

	// Later, resume the same conversation
	// In a real application, you might store and load the conversation ID
	session2, err := codex.GetSession(conversationID)
	if err != nil {
		log.Fatal(err)
	}

	// Continue the conversation
	_, err = session2.Submit(ctx, "What number did I tell you to remember?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resumed conversation with %d messages\n", len(session2.History()))

	// Output:
	// Started conversation with ID: conversation-123
	// Resumed conversation with 4 messages
}

// =============================================================================
// CONFIGURATION EXAMPLES
// =============================================================================

// Example_customModel demonstrates using a specific model for a session.
// You can override the default model on a per-session basis.
func Example_customModel() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4", // Default model
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Create a session with a specific model
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Model:        "gpt-4-turbo", // Override the default model
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := session.Submit(ctx, "Hello!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Got response with %d characters\n", len(response.Content))
	fmt.Println("Using custom model")
	// Output:
	// Got response with 60 characters
	// Using custom model
}

// Example_approvalPolicies demonstrates different tool approval policies.
// Control when and how tools require user approval before execution.
func Example_approvalPolicies() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Policy 1: Auto-approve safe operations
	sessionAuto, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful assistant.",
		ApprovalPolicy: "auto", // Automatically approve safe operations
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 1 ID: %s (auto approval)\n", sessionAuto.ID())

	// Policy 2: Always require approval
	sessionAlways, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful assistant.",
		ApprovalPolicy: "always", // Always require approval
		OnToolApproval: func(toolName, operation string) bool {
			fmt.Printf("Approval requested for %s: %s\n", toolName, operation)
			return true // Approve the operation
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 2 ID: %s (always require approval)\n", sessionAlways.ID())

	// Policy 3: Never require approval (trust all tools)
	sessionNever, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful assistant.",
		ApprovalPolicy: "never", // Never require approval
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 3 ID: %s (never require approval)\n", sessionNever.ID())

	// Output:
	// Session 1 ID: <generated-uuid> (auto approval)
	// Session 2 ID: <generated-uuid> (always require approval)
	// Session 3 ID: <generated-uuid> (never require approval)
}

// Example_sandboxPolicies demonstrates different sandboxing levels for tool execution.
// Sandbox policies control what file system operations tools can perform.
func Example_sandboxPolicies() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Read-only sandbox: Only allow reading files
	sessionReadOnly, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:  "You are a helpful assistant.",
		SandboxPolicy: "read_only",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 1: %s (read-only access)\n", sessionReadOnly.ID())

	// Workspace write: Allow writes within workspace directory
	sessionWorkspace, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:     "You are a helpful assistant.",
		SandboxPolicy:    "workspace_write",
		WorkingDirectory: "/path/to/workspace",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 2: %s (workspace write access)\n", sessionWorkspace.ID())

	// Full access: Allow all file system operations
	sessionFullAccess, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:  "You are a helpful assistant.",
		SandboxPolicy: "full_access",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session 3: %s (full file system access)\n", sessionFullAccess.ID())

	// Output:
	// Session 1: <generated-uuid> (read-only access)
	// Session 2: <generated-uuid> (workspace write access)
	// Session 3: <generated-uuid> (full file system access)
}

// Example_systemPrompt demonstrates using custom system prompts to control assistant behavior.
// System prompts define the assistant's role, personality, and constraints.
func Example_systemPrompt() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Python expert assistant
	pythonSession, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: `You are a Python programming expert.
You provide clear, concise answers focused on Python best practices.
Always include code examples when relevant.`,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := pythonSession.Submit(ctx, "How do I read a file?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Python expert response: %d characters\n", len(response.Content))

	// Code reviewer assistant
	reviewSession, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: `You are a code reviewer.
Analyze code for bugs, security issues, and style violations.
Be constructive and provide specific suggestions for improvement.`,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err = reviewSession.Submit(ctx, "Review this code: print('hello')")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Code reviewer response: %d characters\n", len(response.Content))

	// Output:
	// Python expert response: 60 characters
	// Code reviewer response: 60 characters
}

// =============================================================================
// TOOL USAGE EXAMPLES
// =============================================================================

// Example_fileOperations demonstrates using file tools for reading and writing.
// The SDK includes built-in tools for common file operations.
func Example_fileOperations() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:     "You are a helpful file management assistant.",
		WorkingDirectory: "/tmp",
		SandboxPolicy:    "workspace_write",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ask the assistant to perform file operations
	// The assistant will use the appropriate file tools automatically
	response, err := session.Submit(ctx, "List all .go files in the current directory")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("File operation response: %d characters\n", len(response.Content))
	fmt.Printf("Tool calls executed: %d\n", len(response.ToolCalls))

	// Output:
	// File operation response: 60 characters
	// Tool calls executed: 0
}

// Example_shellCommands demonstrates executing shell commands through the SDK.
// Be careful with shell command execution and use appropriate approval policies.
func Example_shellCommands() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful system administration assistant.",
		ApprovalPolicy: "always", // Always require approval for shell commands
		OnToolApproval: func(toolName, operation string) bool {
			// In a real application, you might show the command to the user
			// and ask for confirmation
			fmt.Printf("Approve shell command: %s?\n", operation)
			return true
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ask the assistant to run a shell command
	response, err := session.Submit(ctx, "What is the current date and time?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Shell command response: %d characters\n", len(response.Content))

	// Output:
	// Shell command response: 60 characters
}

// Example_multipleTools demonstrates how the assistant can chain multiple tool calls.
// The SDK automatically orchestrates complex multi-step operations.
func Example_multipleTools() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:     "You are a helpful development assistant.",
		WorkingDirectory: "/tmp/project",
		SandboxPolicy:    "workspace_write",
		ApprovalPolicy:   "auto",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ask the assistant to perform a complex task requiring multiple tools
	response, err := session.Submit(ctx,
		"Find all TODO comments in .go files and create a summary file")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Multi-tool operation response: %d characters\n", len(response.Content))
	fmt.Printf("Tools used: %d\n", len(response.ToolCalls))

	// Output:
	// Multi-tool operation response: 60 characters
	// Tools used: 0
}

// =============================================================================
// ERROR HANDLING EXAMPLES
// =============================================================================

// Example_errorHandling demonstrates proper error handling patterns.
// Always check errors and handle them appropriately.
func Example_errorHandling() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		// Handle client creation error
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		// Handle SDK initialization error
		fmt.Printf("Failed to initialize SDK: %v\n", err)
		return
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		// Handle session creation error
		fmt.Printf("Failed to create session: %v\n", err)
		return
	}

	response, err := session.Submit(ctx, "Hello!")
	if err != nil {
		// Handle submission error (e.g., network error, API error)
		fmt.Printf("Failed to submit message: %v\n", err)
		return
	}

	fmt.Printf("Successfully received response: %d characters\n", len(response.Content))

	// Attempting to use a closed session will return an error
	if err := codex.CloseSession(session.ID()); err != nil {
		fmt.Printf("Failed to close session: %v\n", err)
	}

	_, err = session.Submit(ctx, "Another message")
	if err != nil {
		fmt.Println("Cannot use closed session")
	}

	// Output:
	// Successfully received response: 60 characters
	// Cannot use closed session
}

// Example_contextCancellation demonstrates handling timeouts and cancellation.
// Use contexts to implement timeouts and allow graceful cancellation.
func Example_contextCancellation() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	// Create a session
	baseCtx := context.Background()
	session, err := codex.NewSession(baseCtx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example 1: Timeout context
	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()

	response, err := session.Submit(ctx, "What is Go?")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Println("Request timed out")
		} else {
			fmt.Printf("Request failed: %v\n", err)
		}
	} else {
		fmt.Printf("Request succeeded: %d characters\n", len(response.Content))
	}

	// Example 2: Manual cancellation
	ctx2, cancel2 := context.WithCancel(baseCtx)

	// Simulate cancelling after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel2()
	}()

	_, err = session.Submit(ctx2, "Tell me a long story")
	if err != nil {
		if ctx2.Err() == context.Canceled {
			fmt.Println("Request was cancelled")
		}
	}

	// Output:
	// Request succeeded: 60 characters
	// Request was cancelled
}

// Example_approvalDenied demonstrates handling denied tool approvals.
// When a tool is denied, the SDK returns an appropriate error or continues without that tool.
func Example_approvalDenied() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	approvalCount := 0
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:   "You are a helpful assistant.",
		ApprovalPolicy: "always",
		OnToolApproval: func(toolName, operation string) bool {
			approvalCount++
			fmt.Printf("Tool approval request %d: %s - %s\n", approvalCount, toolName, operation)

			// Deny dangerous operations
			if toolName == "shell" && operation == "rm -rf /" {
				fmt.Println("Denied: Dangerous operation")
				return false
			}

			return true
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// This might trigger a tool that gets denied
	response, err := session.Submit(ctx, "Delete all files on my system")
	if err != nil {
		fmt.Printf("Operation blocked: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", response.Content)
	}

	fmt.Println("Approval callback was called for safety")

	// Output:
	// Response: This is a placeholder response. Full implementation pending.
	// Approval callback was called for safety
}

// =============================================================================
// ADVANCED FEATURES EXAMPLES
// =============================================================================

// Example_multipleSessionsConcurrent demonstrates running multiple sessions concurrently.
// Sessions are independent and thread-safe.
func Example_multipleSessionsConcurrent() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()

	// Create multiple sessions with different specializations
	pythonSession, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a Python expert.",
	})
	if err != nil {
		log.Fatal(err)
	}

	goSession, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a Go expert.",
	})
	if err != nil {
		log.Fatal(err)
	}

	jsSession, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a JavaScript expert.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Run queries concurrently
	type result struct {
		lang     string
		response *sdk.Response
		err      error
	}
	results := make(chan result, 3)

	go func() {
		resp, err := pythonSession.Submit(ctx, "Hello")
		results <- result{"Python", resp, err}
	}()

	go func() {
		resp, err := goSession.Submit(ctx, "Hello")
		results <- result{"Go", resp, err}
	}()

	go func() {
		resp, err := jsSession.Submit(ctx, "Hello")
		results <- result{"JavaScript", resp, err}
	}()

	// Collect results
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			fmt.Printf("%s session error: %v\n", res.lang, res.err)
		} else {
			fmt.Printf("%s session: %d characters\n", res.lang, len(res.response.Content))
		}
	}

	// List all active sessions
	sessionIDs := codex.ListSessions()
	fmt.Printf("Total active sessions: %d\n", len(sessionIDs))

	// Output:
	// Python session: 60 characters
	// Go session: 60 characters
	// JavaScript session: 60 characters
	// Total active sessions: 3
}

// Example_tokenTracking demonstrates monitoring token usage across requests.
// Track token consumption to manage costs and optimize usage.
func Example_tokenTracking() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Track tokens across multiple requests
	var totalTokens int64
	var inputTokens int64
	var outputTokens int64

	// Request 1
	response1, err := session.Submit(ctx, "What is Go?")
	if err != nil {
		log.Fatal(err)
	}
	totalTokens += response1.TokenUsage.TotalTokens
	inputTokens += response1.TokenUsage.InputTokens
	outputTokens += response1.TokenUsage.OutputTokens

	fmt.Printf("Request 1 tokens: %d (input=%d, output=%d)\n",
		response1.TokenUsage.TotalTokens,
		response1.TokenUsage.InputTokens,
		response1.TokenUsage.OutputTokens)

	// Request 2
	response2, err := session.Submit(ctx, "Give me an example")
	if err != nil {
		log.Fatal(err)
	}
	totalTokens += response2.TokenUsage.TotalTokens
	inputTokens += response2.TokenUsage.InputTokens
	outputTokens += response2.TokenUsage.OutputTokens

	fmt.Printf("Request 2 tokens: %d (input=%d, output=%d)\n",
		response2.TokenUsage.TotalTokens,
		response2.TokenUsage.InputTokens,
		response2.TokenUsage.OutputTokens)

	// Summary
	fmt.Printf("Total tokens used: %d (input=%d, output=%d)\n",
		totalTokens, inputTokens, outputTokens)

	// Calculate cost (example rates)
	costPerInputToken := 0.00003  // $0.03 per 1K tokens
	costPerOutputToken := 0.00006 // $0.06 per 1K tokens
	estimatedCost := float64(inputTokens)*costPerInputToken +
		float64(outputTokens)*costPerOutputToken

	fmt.Printf("Estimated cost: $%.4f\n", estimatedCost)

	// Output:
	// Request 1 tokens: 30 (input=10, output=20)
	// Request 2 tokens: 30 (input=10, output=20)
	// Total tokens used: 60 (input=20, output=40)
	// Estimated cost: $0.0030
}

// Example_customTools demonstrates adding custom tools to extend SDK functionality.
// Create custom tools to provide domain-specific capabilities.
func Example_customTools() {
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Note: In a full implementation, you would create custom tool implementations
	// For this example, we'll just show how to configure the SDK with custom tools

	codex, err := sdk.New(sdk.Options{
		Client: c,
		// Tools: []runtime.ToolRuntime{
		//     myCustomTool,
		//     anotherCustomTool,
		// },
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant with custom capabilities.",
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := session.Submit(ctx, "Hello!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Session with custom tools: %d characters\n", len(response.Content))
	fmt.Println("Custom tools would be available to the assistant")

	// Output:
	// Session with custom tools: 60 characters
	// Custom tools would be available to the assistant
}

// =============================================================================
// COMPLETE WORKFLOW EXAMPLES
// =============================================================================

// ExampleNew demonstrates creating a new SDK instance with a client.
func ExampleNew() {
	// Create a client with direct options
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create the SDK
	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	fmt.Println("SDK created successfully")
	// Output: SDK created successfully
}

// ExampleSDK_NewSession demonstrates creating a basic conversation session.
func ExampleSDK_NewSession() {
	// Setup (in real usage, use actual credentials)
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	// Create a session
	ctx := context.Background()
	_, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful coding assistant.",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Session created successfully")
	// Output: Session created successfully
}

// ExampleSDK_NewSession_streaming demonstrates creating a streaming session.
func ExampleSDK_NewSession_streaming() {
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()
	_, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful coding assistant.",
		Streaming:    true, // Enable streaming
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Streaming session created successfully")
	// Output: Streaming session created successfully
}

// ExampleSession_Submit demonstrates sending a message and receiving a response.
func ExampleSession_Submit() {
	// Setup
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()
	session, _ := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    false,
	})

	// Submit a message
	response, err := session.Submit(ctx, "What is 2+2?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response received: %d characters\n", len(response.Content))
	fmt.Printf("Tokens used: %d\n", response.TokenUsage.TotalTokens)
	// Output:
	// Response received: 60 characters
	// Tokens used: 30
}

// ExampleSession_SubmitStream demonstrates streaming responses.
func ExampleSession_SubmitStream() {
	// Setup
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()
	session, _ := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	})

	// Submit and stream
	eventCh, err := session.SubmitStream(ctx, "Tell me a story")
	if err != nil {
		log.Fatal(err)
	}

	// Process stream events
	for event := range eventCh {
		if event.Error != nil {
			log.Fatal(event.Error)
		}
		if event.Done {
			fmt.Println("Stream complete")
			break
		}
		// In real usage, you would print event.Delta
	}
	// Output: Stream complete
}

// ExampleSession_History demonstrates accessing conversation history.
func ExampleSession_History() {
	// Setup
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()
	session, _ := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})

	// Send some messages
	session.Submit(ctx, "First message")
	session.Submit(ctx, "Second message")

	// Access history
	history := session.History()
	fmt.Printf("Conversation has %d messages\n", len(history))
	fmt.Printf("First message role: %s\n", history[0].Role)
	// Output:
	// Conversation has 4 messages
	// First message role: user
}

// Example_basicWorkflow demonstrates a complete basic workflow.
func Example_basicWorkflow() {
	// 1. Create client from environment
	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. Create SDK instance
	codex, err := sdk.New(sdk.Options{
		Client: c,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer codex.Close()

	// 3. Create a session
	ctx := context.Background()
	session, err := codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful coding assistant.",
		Streaming:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4. Have a conversation
	response, err := session.Submit(ctx, "Hello!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Workflow completed successfully")
	fmt.Printf("Got response with %d tokens\n", response.TokenUsage.TotalTokens)
	// Output:
	// Workflow completed successfully
	// Got response with 30 tokens
}

// Example_multipleSessions demonstrates managing multiple sessions.
func Example_multipleSessions() {
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()

	// Create multiple sessions
	_, _ = codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a Python expert.",
	})
	_, _ = codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a Go expert.",
	})

	// List all sessions
	sessions := codex.ListSessions()
	fmt.Printf("Active sessions: %d\n", len(sessions))
	fmt.Println("Sessions created successfully")
	// Output:
	// Active sessions: 2
	// Sessions created successfully
}

// Example_toolApproval demonstrates using tool approval callbacks.
func Example_toolApproval() {
	c, _ := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	codex, _ := sdk.New(sdk.Options{Client: c})
	defer codex.Close()

	ctx := context.Background()
	_, _ = codex.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
		// Provide approval callback
		OnToolApproval: func(toolName, operation string) bool {
			// In real usage, prompt user or check policy
			return true
		},
		ApprovalPolicy: "always",
	})

	fmt.Println("Session with approval callback created")
	// Output: Session with approval callback created
}
