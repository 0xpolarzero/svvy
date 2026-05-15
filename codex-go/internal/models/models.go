// Package models provides model definitions, registry, and validation for supported AI models.
// It maintains a registry of available models with their capabilities, token limits, and metadata.
package models

import (
	"fmt"
)

// ReasoningEffort represents the level of reasoning effort for reasoning-capable models.
type ReasoningEffort string

const (
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
)

// ReasoningEffortOption describes a reasoning effort level available for a model.
type ReasoningEffortOption struct {
	Effort      ReasoningEffort `json:"reasoning_effort"`
	Description string          `json:"description"`
}

// ModelCapabilities describes what features a model supports.
type ModelCapabilities struct {
	// SupportsVision indicates if the model can process images
	SupportsVision bool

	// SupportsTools indicates if the model can use function/tool calling
	SupportsTools bool

	// SupportsReasoning indicates if the model has explicit reasoning capabilities
	SupportsReasoning bool

	// SupportedReasoningEfforts lists available reasoning effort levels
	SupportedReasoningEfforts []ReasoningEffortOption

	// DefaultReasoningEffort is the default reasoning effort for this model
	DefaultReasoningEffort ReasoningEffort
}

// Model represents a supported AI model with its metadata and capabilities.
type Model struct {
	// ID is the stable identifier for this model preset
	ID string `json:"id"`

	// ModelSlug is the actual model identifier used in API calls
	ModelSlug string `json:"model"`

	// DisplayName is the human-friendly name shown in UIs
	DisplayName string `json:"display_name"`

	// Description is a short explanation of the model's purpose
	Description string `json:"description"`

	// Capabilities describes what features this model supports
	Capabilities ModelCapabilities `json:"capabilities"`

	// ContextWindow is the maximum number of tokens the model can process
	ContextWindow int64 `json:"context_window"`

	// MaxOutputTokens is the maximum number of tokens the model can generate
	MaxOutputTokens int64 `json:"max_output_tokens"`

	// IsDefault indicates if this is the default model for new sessions
	IsDefault bool `json:"is_default"`
}

// ValidateReasoningEffort checks if the given effort level is supported by this model.
func (m *Model) ValidateReasoningEffort(effort ReasoningEffort) error {
	if !m.Capabilities.SupportsReasoning {
		return fmt.Errorf("model %s does not support reasoning", m.ID)
	}

	if effort == "" {
		return nil // Empty means use default
	}

	for _, supported := range m.Capabilities.SupportedReasoningEfforts {
		if supported.Effort == effort {
			return nil
		}
	}

	return fmt.Errorf("model %s does not support reasoning effort %s", m.ID, effort)
}

// GetEffectiveReasoningEffort returns the reasoning effort to use, falling back to default if empty.
func (m *Model) GetEffectiveReasoningEffort(effort ReasoningEffort) ReasoningEffort {
	if effort == "" {
		return m.Capabilities.DefaultReasoningEffort
	}
	return effort
}

// ModelRegistry maintains the catalog of available models and provides lookup functionality.
type ModelRegistry struct {
	models       map[string]*Model
	defaultModel *Model
}

// NewRegistry creates a new model registry with the default set of supported models.
func NewRegistry() *ModelRegistry {
	registry := &ModelRegistry{
		models: make(map[string]*Model),
	}

	// Register all built-in models
	for _, model := range builtinModels {
		registry.Register(model)
	}

	return registry
}

// Register adds a model to the registry.
func (r *ModelRegistry) Register(model *Model) {
	r.models[model.ID] = model
	if model.IsDefault {
		r.defaultModel = model
	}
}

// Get retrieves a model by ID. Returns nil if not found.
func (r *ModelRegistry) Get(id string) *Model {
	return r.models[id]
}

// GetBySlug retrieves a model by its slug. Returns nil if not found.
func (r *ModelRegistry) GetBySlug(slug string) *Model {
	for _, model := range r.models {
		if model.ModelSlug == slug {
			return model
		}
	}
	return nil
}

// GetDefault returns the default model.
func (r *ModelRegistry) GetDefault() *Model {
	return r.defaultModel
}

// List returns all registered models.
func (r *ModelRegistry) List() []*Model {
	result := make([]*Model, 0, len(r.models))
	for _, model := range r.models {
		result = append(result, model)
	}
	return result
}

// Exists checks if a model with the given ID exists in the registry.
func (r *ModelRegistry) Exists(id string) bool {
	_, exists := r.models[id]
	return exists
}

// Validate checks if a model ID is valid and returns the model if so.
func (r *ModelRegistry) Validate(id string) (*Model, error) {
	model := r.Get(id)
	if model == nil {
		return nil, fmt.Errorf("unknown model: %s", id)
	}
	return model, nil
}

// DefaultRegistry is the global model registry instance.
var DefaultRegistry = NewRegistry()

// Built-in model definitions matching Rust implementation
var builtinModels = []*Model{
	{
		ID:          "gpt-5-codex",
		ModelSlug:   "gpt-5-codex",
		DisplayName: "gpt-5-codex",
		Description: "Optimized for coding tasks with many tools.",
		Capabilities: ModelCapabilities{
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsReasoning: true,
			SupportedReasoningEfforts: []ReasoningEffortOption{
				{
					Effort:      ReasoningEffortLow,
					Description: "Fastest responses with limited reasoning",
				},
				{
					Effort:      ReasoningEffortMedium,
					Description: "Dynamically adjusts reasoning based on the task",
				},
				{
					Effort:      ReasoningEffortHigh,
					Description: "Maximizes reasoning depth for complex or ambiguous problems",
				},
			},
			DefaultReasoningEffort: ReasoningEffortMedium,
		},
		ContextWindow:   200000, // 200k tokens
		MaxOutputTokens: 16384,  // 16k tokens
		IsDefault:       true,
	},
	{
		ID:          "gpt-5",
		ModelSlug:   "gpt-5",
		DisplayName: "gpt-5",
		Description: "Broad world knowledge with strong general reasoning.",
		Capabilities: ModelCapabilities{
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsReasoning: true,
			SupportedReasoningEfforts: []ReasoningEffortOption{
				{
					Effort:      ReasoningEffortMinimal,
					Description: "Fastest responses with little reasoning",
				},
				{
					Effort:      ReasoningEffortLow,
					Description: "Balances speed with some reasoning; useful for straightforward queries and short explanations",
				},
				{
					Effort:      ReasoningEffortMedium,
					Description: "Provides a solid balance of reasoning depth and latency for general-purpose tasks",
				},
				{
					Effort:      ReasoningEffortHigh,
					Description: "Maximizes reasoning depth for complex or ambiguous problems",
				},
			},
			DefaultReasoningEffort: ReasoningEffortMedium,
		},
		ContextWindow:   128000, // 128k tokens
		MaxOutputTokens: 8192,   // 8k tokens
		IsDefault:       false,
	},
}
