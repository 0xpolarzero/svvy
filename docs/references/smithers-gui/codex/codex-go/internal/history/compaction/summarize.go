package compaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/evmts/codex/codex-go/internal/client"
)

// Summarizer uses an LLM to create concise summaries of conversation history.
// This enables aggressive compaction while preserving semantic meaning.
type Summarizer struct {
	// Client for making LLM API calls
	Client client.Client

	// Policy determines which messages can be summarized
	Policy *Policy

	// BatchSize controls how many messages to summarize at once
	// Larger batches are more efficient but may lose detail
	BatchSize int

	// Model override for summarization (if different from main model)
	Model string

	// SystemPrompt for the summarization task
	SystemPrompt string

	// MaxSummaryTokens limits the length of generated summaries
	MaxSummaryTokens int

	// PreserveTurnStructure maintains user/assistant turn boundaries
	PreserveTurnStructure bool
}

// NewSummarizer creates a summarizer with sensible defaults.
func NewSummarizer(client client.Client) *Summarizer {
	return &Summarizer{
		Client:                client,
		Policy:                NewDefaultPolicy(),
		BatchSize:             6, // 3 turns
		SystemPrompt:          defaultSummarizationPrompt,
		MaxSummaryTokens:      500,
		PreserveTurnStructure: false,
	}
}

// defaultSummarizationPrompt is the system prompt for summarization.
const defaultSummarizationPrompt = `You are a conversation history summarizer. Your task is to create concise, accurate summaries of conversation segments while preserving key information.

Guidelines:
- Preserve factual information, decisions, and important context
- Remove greetings, acknowledgments, and filler content
- Maintain chronological order of events
- Focus on user goals and assistant actions
- Use third person: "User asked...", "Assistant explained..."
- Be concise but comprehensive
- Preserve technical details, code snippets, and specific data

Format: Single paragraph summary, 2-4 sentences maximum.`

// Summarize creates a summary of the given messages.
// Returns a string summary or error if summarization fails.
func (s *Summarizer) Summarize(ctx context.Context, messages []client.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Filter messages that can be summarized
	summarizable := make([]client.Message, 0, len(messages))
	for _, msg := range messages {
		if s.Policy.CanSummarize(msg) {
			summarizable = append(summarizable, msg)
		}
	}

	if len(summarizable) == 0 {
		return "", fmt.Errorf("no summarizable messages")
	}

	// Batch summarization if enabled
	if s.BatchSize > 0 && len(summarizable) > s.BatchSize {
		return s.summarizeBatched(ctx, summarizable)
	}

	// Single summarization
	return s.summarizeSingle(ctx, summarizable)
}

// SummarizeToMessage creates a summary message that can replace the originals.
func (s *Summarizer) SummarizeToMessage(ctx context.Context, messages []client.Message) (client.Message, error) {
	summary, err := s.Summarize(ctx, messages)
	if err != nil {
		return client.Message{}, err
	}

	// Create a user message with the summary
	return client.Message{
		Role:    "user",
		Content: fmt.Sprintf("[Previous conversation summary: %s]", summary),
	}, nil
}

// SummarizeWithMetadata includes metadata about what was summarized.
type SummaryResult struct {
	// Summary is the generated text
	Summary string

	// OriginalMessages is the input
	OriginalMessages []client.Message

	// TokensSaved is the estimated token reduction
	TokensSaved int

	// SummaryTokens is the token count of the summary
	SummaryTokens int
}

// SummarizeWithResult creates a summary with detailed results.
func (s *Summarizer) SummarizeWithResult(ctx context.Context, messages []client.Message, counter TokenCounter) (*SummaryResult, error) {
	summary, err := s.Summarize(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Calculate token savings
	originalTokens := 0
	for _, msg := range messages {
		originalTokens += countMessageTokens(msg, counter)
	}

	summaryTokens := counter.CountTokens(summary)

	return &SummaryResult{
		Summary:          summary,
		OriginalMessages: messages,
		TokensSaved:      originalTokens - summaryTokens,
		SummaryTokens:    summaryTokens,
	}, nil
}

// summarizeSingle performs a single summarization API call.
func (s *Summarizer) summarizeSingle(ctx context.Context, messages []client.Message) (string, error) {
	// Format messages for summarization
	formatted := s.formatMessagesForSummary(messages)

	// Create summarization request
	req := &client.ChatCompletionRequest{
		Model: s.getModel(),
		Messages: []client.Message{
			{Role: "system", Content: s.SystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Summarize this conversation:\n\n%s", formatted)},
		},
		MaxTokens: &s.MaxSummaryTokens,
	}

	// Call API
	resp, err := s.Client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}

	// Extract summary from response
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	summary, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("invalid summary format")
	}

	return strings.TrimSpace(summary), nil
}

// summarizeBatched performs multiple summarization calls for large histories.
func (s *Summarizer) summarizeBatched(ctx context.Context, messages []client.Message) (string, error) {
	batches := s.splitIntoBatches(messages)
	summaries := make([]string, 0, len(batches))

	for i, batch := range batches {
		summary, err := s.summarizeSingle(ctx, batch)
		if err != nil {
			return "", fmt.Errorf("batch %d summarization failed: %w", i, err)
		}
		summaries = append(summaries, summary)
	}

	// If we have multiple summaries, create a final summary of summaries
	if len(summaries) > 1 {
		return s.summarizeOfSummaries(ctx, summaries)
	}

	return summaries[0], nil
}

// summarizeOfSummaries creates a meta-summary from multiple summaries.
func (s *Summarizer) summarizeOfSummaries(ctx context.Context, summaries []string) (string, error) {
	combined := strings.Join(summaries, "\n\n")

	req := &client.ChatCompletionRequest{
		Model: s.getModel(),
		Messages: []client.Message{
			{Role: "system", Content: s.SystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Create a cohesive summary from these segment summaries:\n\n%s", combined)},
		},
		MaxTokens: &s.MaxSummaryTokens,
	}

	resp, err := s.Client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("meta-summarization failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no meta-summary generated")
	}

	summary, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("invalid meta-summary format")
	}

	return strings.TrimSpace(summary), nil
}

// formatMessagesForSummary converts messages to a readable format.
func (s *Summarizer) formatMessagesForSummary(messages []client.Message) string {
	var builder strings.Builder

	for i, msg := range messages {
		// Add role label
		switch msg.Role {
		case "user":
			builder.WriteString("User: ")
		case "assistant":
			builder.WriteString("Assistant: ")
		case "system":
			builder.WriteString("System: ")
		default:
			builder.WriteString(msg.Role)
			builder.WriteString(": ")
		}

		// Add content
		if content, ok := msg.Content.(string); ok {
			builder.WriteString(content)
		} else if contentItems, ok := msg.Content.([]client.ContentItem); ok {
			for _, item := range contentItems {
				builder.WriteString(item.Text)
				builder.WriteString(" ")
			}
		}

		// Add newline between messages
		if i < len(messages)-1 {
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
}

// splitIntoBatches divides messages into batches for processing.
func (s *Summarizer) splitIntoBatches(messages []client.Message) [][]client.Message {
	if s.BatchSize <= 0 {
		return [][]client.Message{messages}
	}

	batches := make([][]client.Message, 0)
	for i := 0; i < len(messages); i += s.BatchSize {
		end := i + s.BatchSize
		if end > len(messages) {
			end = len(messages)
		}
		batches = append(batches, messages[i:end])
	}

	return batches
}

// getModel returns the model to use for summarization.
func (s *Summarizer) getModel() string {
	if s.Model != "" {
		return s.Model
	}
	// Fall back to client's default model
	return ""
}

// SummarizeRange summarizes a specific range of messages.
func (s *Summarizer) SummarizeRange(ctx context.Context, messages []client.Message, start, end int) (string, error) {
	if start < 0 || end > len(messages) || start >= end {
		return "", fmt.Errorf("invalid range: [%d:%d] for %d messages", start, end, len(messages))
	}

	return s.Summarize(ctx, messages[start:end])
}

// SummarizeByTurns summarizes conversation preserving turn structure.
func (s *Summarizer) SummarizeByTurns(ctx context.Context, messages []client.Message, maxTurns int) ([]client.Message, error) {
	if !s.PreserveTurnStructure {
		// Simple summarization without turn preservation
		summary, err := s.Summarize(ctx, messages)
		if err != nil {
			return nil, err
		}
		return []client.Message{
			{Role: "user", Content: fmt.Sprintf("[Summary: %s]", summary)},
		}, nil
	}

	// Split into turns
	turns := s.splitIntoTurns(messages)

	if len(turns) <= maxTurns {
		// Already within limit
		return messages, nil
	}

	// Summarize old turns, keep recent ones
	oldTurns := turns[:len(turns)-maxTurns]
	recentTurns := turns[len(turns)-maxTurns:]

	// Flatten old turns for summarization
	oldMessages := make([]client.Message, 0)
	for _, turn := range oldTurns {
		oldMessages = append(oldMessages, turn...)
	}

	summary, err := s.Summarize(ctx, oldMessages)
	if err != nil {
		return nil, err
	}

	// Build result with summary + recent turns
	result := []client.Message{
		{Role: "user", Content: fmt.Sprintf("[Earlier conversation: %s]", summary)},
	}

	for _, turn := range recentTurns {
		result = append(result, turn...)
	}

	return result, nil
}

// splitIntoTurns groups messages into user-assistant pairs.
func (s *Summarizer) splitIntoTurns(messages []client.Message) [][]client.Message {
	turns := make([][]client.Message, 0)
	currentTurn := make([]client.Message, 0)

	for _, msg := range messages {
		// Skip system messages in turn grouping
		if msg.Role == "system" {
			continue
		}

		currentTurn = append(currentTurn, msg)

		// End turn after assistant message
		if msg.Role == "assistant" {
			turns = append(turns, currentTurn)
			currentTurn = make([]client.Message, 0)
		}
	}

	// Add remaining messages as incomplete turn
	if len(currentTurn) > 0 {
		turns = append(turns, currentTurn)
	}

	return turns
}

// EstimateSummaryLength estimates the token length of a summary.
func (s *Summarizer) EstimateSummaryLength(messages []client.Message, counter TokenCounter) int {
	// Rough estimate: summaries are typically 20-30% of original length
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += countMessageTokens(msg, counter)
	}

	estimatedSummary := int(float64(totalTokens) * 0.25)

	// Apply max limit
	if estimatedSummary > s.MaxSummaryTokens {
		return s.MaxSummaryTokens
	}

	return estimatedSummary
}

// CanSummarize checks if messages are suitable for summarization.
func (s *Summarizer) CanSummarize(messages []client.Message) bool {
	if len(messages) == 0 {
		return false
	}

	// Check if at least one message can be summarized
	for _, msg := range messages {
		if s.Policy.CanSummarize(msg) {
			return true
		}
	}

	return false
}

// SummaryQuality estimates the quality of a summary.
type SummaryQuality struct {
	// Score from 0.0 to 1.0 (higher is better)
	Score float64

	// ReductionRatio is the percentage of tokens saved (0.0 to 1.0)
	ReductionRatio float64

	// InformationLoss estimates how much detail was lost
	InformationLoss float64
}

// EvaluateSummary assesses the quality of a summarization.
func (s *Summarizer) EvaluateSummary(original []client.Message, summary string, counter TokenCounter) *SummaryQuality {
	originalTokens := 0
	for _, msg := range original {
		originalTokens += countMessageTokens(msg, counter)
	}

	summaryTokens := counter.CountTokens(summary)

	if originalTokens == 0 {
		return &SummaryQuality{
			Score:           0.0,
			ReductionRatio:  0.0,
			InformationLoss: 1.0,
		}
	}

	reductionRatio := 1.0 - (float64(summaryTokens) / float64(originalTokens))

	// Estimate information loss (inverse of reduction quality)
	// Better summaries have high reduction with low loss
	informationLoss := 0.3 // Baseline loss estimate

	// Longer originals can tolerate more compression
	if originalTokens > 1000 {
		informationLoss = 0.2
	}

	// Very aggressive compression likely loses more
	if reductionRatio > 0.8 {
		informationLoss = 0.5
	}

	// Quality score balances reduction and loss
	score := reductionRatio * (1.0 - informationLoss)

	return &SummaryQuality{
		Score:           score,
		ReductionRatio:  reductionRatio,
		InformationLoss: informationLoss,
	}
}
