package models

import (
	"encoding/json"
	"fmt"
)

// ModelList represents a list of available models suitable for API responses.
type ModelList struct {
	Models []*Model `json:"models"`
}

// ToJSON serializes the model list to JSON.
func (ml *ModelList) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ml, "", "  ")
}

// SupportedModels returns all models available in the default registry.
func SupportedModels() []*Model {
	return DefaultRegistry.List()
}

// GetModel retrieves a model by ID from the default registry.
func GetModel(id string) (*Model, error) {
	model := DefaultRegistry.Get(id)
	if model == nil {
		return nil, fmt.Errorf("unknown model: %s", id)
	}
	return model, nil
}

// GetModelBySlug retrieves a model by its slug from the default registry.
func GetModelBySlug(slug string) (*Model, error) {
	model := DefaultRegistry.GetBySlug(slug)
	if model == nil {
		return nil, fmt.Errorf("unknown model slug: %s", slug)
	}
	return model, nil
}

// GetDefaultModel returns the default model.
func GetDefaultModel() *Model {
	return DefaultRegistry.GetDefault()
}

// ValidateModel checks if a model ID is valid.
func ValidateModel(id string) error {
	if id == "" {
		return nil // Empty is valid, will use default
	}
	_, err := GetModel(id)
	return err
}

// ResolveModel resolves a model ID, returning the default if empty.
func ResolveModel(id string) (*Model, error) {
	if id == "" {
		return GetDefaultModel(), nil
	}
	return GetModel(id)
}

// ModelInfo contains detailed information about a model for API responses.
type ModelInfo struct {
	ID                   string                  `json:"id"`
	Model                string                  `json:"model"`
	DisplayName          string                  `json:"display_name"`
	Description          string                  `json:"description"`
	ContextWindow        int64                   `json:"context_window"`
	MaxOutputTokens      int64                   `json:"max_output_tokens"`
	SupportsVision       bool                    `json:"supports_vision"`
	SupportsTools        bool                    `json:"supports_tools"`
	SupportsReasoning    bool                    `json:"supports_reasoning"`
	ReasoningEfforts     []ReasoningEffortOption `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort ReasoningEffort       `json:"default_reasoning_effort,omitempty"`
	IsDefault            bool                    `json:"is_default"`
}

// ToModelInfo converts a Model to ModelInfo for API responses.
func ToModelInfo(m *Model) *ModelInfo {
	info := &ModelInfo{
		ID:              m.ID,
		Model:           m.ModelSlug,
		DisplayName:     m.DisplayName,
		Description:     m.Description,
		ContextWindow:   m.ContextWindow,
		MaxOutputTokens: m.MaxOutputTokens,
		SupportsVision:  m.Capabilities.SupportsVision,
		SupportsTools:   m.Capabilities.SupportsTools,
		SupportsReasoning: m.Capabilities.SupportsReasoning,
		IsDefault:       m.IsDefault,
	}

	if m.Capabilities.SupportsReasoning {
		info.ReasoningEfforts = m.Capabilities.SupportedReasoningEfforts
		info.DefaultReasoningEffort = m.Capabilities.DefaultReasoningEffort
	}

	return info
}

// ListModelsInfo returns all models as ModelInfo for API responses.
func ListModelsInfo() []*ModelInfo {
	models := SupportedModels()
	result := make([]*ModelInfo, len(models))
	for i, m := range models {
		result[i] = ToModelInfo(m)
	}
	return result
}
