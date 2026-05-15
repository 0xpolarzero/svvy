package client

import "encoding/json"

// ChatCompletionRequest represents a request to the chat completions API.
// Supports both the Responses API (experimental) and Chat Completions API.
type ChatCompletionRequest struct {
	// Model is the model identifier (e.g., "gpt-4", "gpt-5")
	Model string `json:"model"`

	// Messages contains the conversation history (Chat Completions API format)
	Messages []Message `json:"messages,omitempty"`

	// Instructions is the system prompt (Responses API format)
	Instructions string `json:"instructions,omitempty"`

	// Input contains the conversation items (Responses API format)
	Input []ResponseItem `json:"input,omitempty"`

	// Tools available for the model to call
	Tools []Tool `json:"tools,omitempty"`

	// ToolChoice controls tool selection: "auto", "required", "none", or specific tool
	ToolChoice interface{} `json:"tool_choice,omitempty"`

	// ParallelToolCalls enables parallel execution of tool calls
	ParallelToolCalls bool `json:"parallel_tool_calls,omitempty"`

	// Stream enables streaming responses via SSE
	Stream bool `json:"stream,omitempty"`

	// Reasoning configuration for models with reasoning capabilities
	Reasoning *Reasoning `json:"reasoning,omitempty"`

	// Text controls output formatting and verbosity (GPT-5 specific)
	Text *TextControls `json:"text,omitempty"`

	// Store controls whether to persist the conversation
	Store bool `json:"store,omitempty"`

	// Include specifies which optional fields to include in response
	Include []string `json:"include,omitempty"`

	// PromptCacheKey enables prompt caching for faster subsequent requests
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`

	// Temperature controls randomness (0.0 to 2.0, typically)
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxTokens limits the response length
	MaxTokens *int `json:"max_tokens,omitempty"`

	// TopP controls nucleus sampling
	TopP *float64 `json:"top_p,omitempty"`

	// Stop sequences that trigger completion
	Stop []string `json:"stop,omitempty"`
}

// ChatCompletionResponse represents a complete (non-streaming) response.
type ChatCompletionResponse struct {
	// ID is the unique identifier for this response
	ID string `json:"id"`

	// Object type (e.g., "chat.completion", "response")
	Object string `json:"object"`

	// Created is the Unix timestamp of creation
	Created int64 `json:"created"`

	// Model used to generate the response
	Model string `json:"model"`

	// Choices contains the generated completions (Chat Completions API)
	Choices []Choice `json:"choices,omitempty"`

	// Output contains the response items (Responses API)
	Output []ResponseItem `json:"output,omitempty"`

	// Usage contains token consumption details
	Usage *TokenUsage `json:"usage,omitempty"`

	// SystemFingerprint for tracking model version
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChunk represents a streaming response fragment.
type ChatCompletionChunk struct {
	// ID is the unique identifier for this response stream
	ID string `json:"id"`

	// Object type (typically "chat.completion.chunk")
	Object string `json:"object"`

	// Created is the Unix timestamp of creation
	Created int64 `json:"created"`

	// Model used to generate the response
	Model string `json:"model"`

	// Choices contains the generated completion deltas
	Choices []ChunkChoice `json:"choices"`

	// Usage contains token consumption (may be partial during streaming)
	Usage *TokenUsage `json:"usage,omitempty"`
}

// Message represents a single message in a conversation.
type Message struct {
	// Role identifies the message sender: "system", "user", "assistant", "tool"
	Role string `json:"role"`

	// Content is the message text (or structured content)
	Content interface{} `json:"content"`

	// Name is an optional identifier for the message sender
	Name string `json:"name,omitempty"`

	// ToolCalls contains function/tool invocations (assistant messages)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID references the tool call this message responds to (tool messages)
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Reasoning contains thinking/reasoning text (some models)
	Reasoning string `json:"reasoning,omitempty"`
}

// ResponseItem represents a single item in a conversation (Responses API format).
// This mirrors the Rust ResponseItem enum.
type ResponseItem struct {
	// Type identifies the kind of item
	Type string `json:"type"`

	// Role for message items ("user", "assistant")
	Role string `json:"role,omitempty"`

	// Content for message items
	Content []ContentItem `json:"content,omitempty"`

	// ID is a unique identifier for this item
	ID *string `json:"id,omitempty"`

	// Name for function calls
	Name string `json:"name,omitempty"`

	// Arguments for function calls (JSON string)
	Arguments string `json:"arguments,omitempty"`

	// CallID references the function call
	CallID string `json:"call_id,omitempty"`

	// Status for tool calls
	Status string `json:"status,omitempty"`

	// Action for shell commands
	Action string `json:"action,omitempty"`

	// Input for custom tool calls
	Input string `json:"input,omitempty"`

	// Output for tool responses
	Output interface{} `json:"output,omitempty"`

	// Summary for reasoning items
	Summary []ReasoningItemContent `json:"summary,omitempty"`

	// EncryptedContent for reasoning items
	EncryptedContent *string `json:"encrypted_content,omitempty"`
}

// ContentItem represents a piece of content within a message.
type ContentItem struct {
	// Type identifies the content type: "input_text", "output_text", "image", etc.
	Type string `json:"type"`

	// Text content (for text items)
	Text string `json:"text,omitempty"`

	// ImageURL for image content
	ImageURL *ImageURL `json:"image_url,omitempty"`

	// Additional fields for other content types
	Data json.RawMessage `json:"-"`
}

// ImageURL represents an image reference.
type ImageURL struct {
	// URL or data URI of the image
	URL string `json:"url"`

	// Detail level: "auto", "low", "high"
	Detail string `json:"detail,omitempty"`
}

// ReasoningItemContent represents reasoning/thinking content.
type ReasoningItemContent struct {
	// Type identifies the content type: "reasoning_text", "text"
	Type string `json:"type"`

	// Text contains the reasoning content
	Text string `json:"text,omitempty"`
}

// Choice represents a completion choice in a non-streaming response.
type Choice struct {
	// Index of this choice
	Index int `json:"index"`

	// Message is the generated message
	Message Message `json:"message"`

	// FinishReason indicates why generation stopped: "stop", "length", "tool_calls", etc.
	FinishReason string `json:"finish_reason"`

	// LogProbs contains token log probabilities (if requested)
	LogProbs interface{} `json:"logprobs,omitempty"`
}

// ChunkChoice represents a completion choice in a streaming response.
type ChunkChoice struct {
	// Index of this choice
	Index int `json:"index"`

	// Delta contains the incremental changes to the message
	Delta MessageDelta `json:"delta"`

	// FinishReason indicates why generation stopped (empty until final chunk)
	FinishReason string `json:"finish_reason,omitempty"`

	// LogProbs contains token log probabilities (if requested)
	LogProbs interface{} `json:"logprobs,omitempty"`
}

// MessageDelta represents incremental changes in a streaming response.
type MessageDelta struct {
	// Role is set in the first chunk
	Role string `json:"role,omitempty"`

	// Content contains incremental text
	Content string `json:"content,omitempty"`

	// ToolCalls contains incremental tool call information
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`

	// Reasoning contains incremental reasoning text
	Reasoning interface{} `json:"reasoning,omitempty"`
}

// Tool represents a function or tool available to the model.
type Tool struct {
	// Type identifies the tool type: "function", "local_shell", "web_search", "custom"
	Type string `json:"type"`

	// Function definition (for function tools)
	Function *FunctionDefinition `json:"function,omitempty"`

	// Custom tool definition (for custom tools)
	Custom *CustomToolDefinition `json:"custom,omitempty"`
}

// FunctionDefinition defines a function the model can call.
type FunctionDefinition struct {
	// Name of the function
	Name string `json:"name"`

	// Description of what the function does
	Description string `json:"description"`

	// Parameters schema (JSON Schema)
	Parameters json.RawMessage `json:"parameters"`

	// Strict enables strict schema validation (OpenAI specific)
	Strict bool `json:"strict,omitempty"`
}

// CustomToolDefinition defines a custom/freeform tool.
type CustomToolDefinition struct {
	// Name of the tool
	Name string `json:"name"`

	// Description of what the tool does
	Description string `json:"description"`

	// Format specifies the input format
	Format *CustomToolFormat `json:"format,omitempty"`
}

// CustomToolFormat specifies the format for a custom tool.
type CustomToolFormat struct {
	// Type of format (e.g., "text")
	Type string `json:"type"`

	// Syntax (e.g., "markdown")
	Syntax string `json:"syntax"`

	// Definition is the tool's input specification
	Definition string `json:"definition"`
}

// ToolCall represents a function/tool invocation.
type ToolCall struct {
	// ID is a unique identifier for this tool call
	ID string `json:"id"`

	// Type identifies the tool type: "function", "local_shell_call", "custom"
	Type string `json:"type"`

	// Function contains function call details
	Function *FunctionCall `json:"function,omitempty"`

	// Custom contains custom tool call details
	Custom *CustomToolCall `json:"custom,omitempty"`

	// Status for local_shell_call
	Status string `json:"status,omitempty"`

	// Action for local_shell_call
	Action string `json:"action,omitempty"`
}

// FunctionCall represents a function invocation.
type FunctionCall struct {
	// Name of the function being called
	Name string `json:"name"`

	// Arguments as a JSON string
	Arguments string `json:"arguments"`
}

// CustomToolCall represents a custom tool invocation.
type CustomToolCall struct {
	// Name of the custom tool
	Name string `json:"name"`

	// Input for the custom tool
	Input string `json:"input"`
}

// ToolCallDelta represents incremental tool call information in streaming.
type ToolCallDelta struct {
	// Index of this tool call
	Index int `json:"index"`

	// ID of the tool call (set in first chunk)
	ID string `json:"id,omitempty"`

	// Type of tool call (set in first chunk)
	Type string `json:"type,omitempty"`

	// Function contains incremental function call data
	Function *FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta represents incremental function call data.
type FunctionCallDelta struct {
	// Name of the function (set in first chunk)
	Name string `json:"name,omitempty"`

	// Arguments contains incremental JSON string fragments
	Arguments string `json:"arguments,omitempty"`
}

// Reasoning configures reasoning behavior for capable models.
type Reasoning struct {
	// Effort controls reasoning depth: "low", "medium", "high"
	Effort *string `json:"effort,omitempty"`

	// Summary controls reasoning summary: "off", "auto", "auto_with_summary_part_added_events"
	Summary *string `json:"summary,omitempty"`
}

// TextControls configures text output (GPT-5 specific).
type TextControls struct {
	// Verbosity controls output verbosity: "low", "medium", "high"
	Verbosity *string `json:"verbosity,omitempty"`

	// Format specifies structured output format
	Format *TextFormat `json:"format,omitempty"`
}

// TextFormat specifies structured output format.
type TextFormat struct {
	// Type is typically "json_schema"
	Type string `json:"type"`

	// Strict enables strict schema validation
	Strict bool `json:"strict"`

	// Schema is the JSON schema
	Schema json.RawMessage `json:"schema"`

	// Name identifies this schema
	Name string `json:"name"`
}

// FunctionCallOutput represents the result of a function call.
type FunctionCallOutput struct {
	// Content is the output from the function
	Content string `json:"content"`
}

// Helper methods for creating common message types

// NewUserMessage creates a user message.
func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// NewAssistantMessage creates an assistant message.
func NewAssistantMessage(content string) Message {
	return Message{
		Role:    "assistant",
		Content: content,
	}
}

// NewSystemMessage creates a system message.
func NewSystemMessage(content string) Message {
	return Message{
		Role:    "system",
		Content: content,
	}
}

// NewToolMessage creates a tool response message.
func NewToolMessage(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// NewFunctionTool creates a function tool definition.
func NewFunctionTool(name, description string, parameters json.RawMessage) Tool {
	return Tool{
		Type: "function",
		Function: &FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  parameters,
			Strict:      false,
		},
	}
}

// NewLocalShellTool creates a local shell tool.
func NewLocalShellTool() Tool {
	return Tool{
		Type: "local_shell",
	}
}

// NewWebSearchTool creates a web search tool.
func NewWebSearchTool() Tool {
	return Tool{
		Type: "web_search",
	}
}
