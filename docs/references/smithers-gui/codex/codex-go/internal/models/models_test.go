package models

import (
	"testing"
)

func TestModelRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	testModel := &Model{
		ID:          "test-model",
		ModelSlug:   "test-model-v1",
		DisplayName: "Test Model",
		Description: "A test model",
		Capabilities: ModelCapabilities{
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsReasoning: false,
		},
		ContextWindow:   100000,
		MaxOutputTokens: 4096,
		IsDefault:       false,
	}

	registry.Register(testModel)

	retrieved := registry.Get("test-model")
	if retrieved == nil {
		t.Fatal("expected model to be registered")
	}

	if retrieved.ID != testModel.ID {
		t.Errorf("expected ID %s, got %s", testModel.ID, retrieved.ID)
	}
}

func TestModelRegistry_GetBySlug(t *testing.T) {
	registry := NewRegistry()

	model := registry.GetBySlug("gpt-5-codex")
	if model == nil {
		t.Fatal("expected to find gpt-5-codex by slug")
	}

	if model.ID != "gpt-5-codex" {
		t.Errorf("expected ID gpt-5-codex, got %s", model.ID)
	}
}

func TestModelRegistry_GetDefault(t *testing.T) {
	registry := NewRegistry()

	defaultModel := registry.GetDefault()
	if defaultModel == nil {
		t.Fatal("expected a default model")
	}

	if !defaultModel.IsDefault {
		t.Error("expected IsDefault to be true")
	}

	if defaultModel.ID != "gpt-5-codex" {
		t.Errorf("expected default model to be gpt-5-codex, got %s", defaultModel.ID)
	}
}

func TestModelRegistry_List(t *testing.T) {
	registry := NewRegistry()

	models := registry.List()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}

	// Should have at least gpt-5-codex and gpt-5
	if len(models) < 2 {
		t.Errorf("expected at least 2 models, got %d", len(models))
	}
}

func TestModelRegistry_Validate(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name      string
		modelID   string
		expectErr bool
	}{
		{
			name:      "valid model",
			modelID:   "gpt-5-codex",
			expectErr: false,
		},
		{
			name:      "another valid model",
			modelID:   "gpt-5",
			expectErr: false,
		},
		{
			name:      "invalid model",
			modelID:   "nonexistent-model",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := registry.Validate(tt.modelID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				if model != nil {
					t.Error("expected nil model for invalid ID")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if model == nil {
					t.Error("expected model but got nil")
				}
			}
		})
	}
}

func TestModel_ValidateReasoningEffort(t *testing.T) {
	registry := NewRegistry()

	// Test with reasoning-capable model
	reasoningModel := registry.Get("gpt-5-codex")
	if reasoningModel == nil {
		t.Fatal("expected to find gpt-5-codex")
	}

	tests := []struct {
		name      string
		effort    ReasoningEffort
		expectErr bool
	}{
		{
			name:      "empty effort (use default)",
			effort:    "",
			expectErr: false,
		},
		{
			name:      "valid low effort",
			effort:    ReasoningEffortLow,
			expectErr: false,
		},
		{
			name:      "valid medium effort",
			effort:    ReasoningEffortMedium,
			expectErr: false,
		},
		{
			name:      "valid high effort",
			effort:    ReasoningEffortHigh,
			expectErr: false,
		},
		{
			name:      "invalid effort",
			effort:    "ultra-high",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reasoningModel.ValidateReasoningEffort(tt.effort)

			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestModel_GetEffectiveReasoningEffort(t *testing.T) {
	registry := NewRegistry()
	model := registry.Get("gpt-5-codex")
	if model == nil {
		t.Fatal("expected to find gpt-5-codex")
	}

	tests := []struct {
		name     string
		input    ReasoningEffort
		expected ReasoningEffort
	}{
		{
			name:     "empty defaults to medium",
			input:    "",
			expected: ReasoningEffortMedium,
		},
		{
			name:     "explicit low",
			input:    ReasoningEffortLow,
			expected: ReasoningEffortLow,
		},
		{
			name:     "explicit high",
			input:    ReasoningEffortHigh,
			expected: ReasoningEffortHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.GetEffectiveReasoningEffort(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuiltinModels(t *testing.T) {
	// Test that built-in models have required fields
	for _, model := range builtinModels {
		t.Run(model.ID, func(t *testing.T) {
			if model.ID == "" {
				t.Error("model ID cannot be empty")
			}
			if model.ModelSlug == "" {
				t.Error("model slug cannot be empty")
			}
			if model.DisplayName == "" {
				t.Error("display name cannot be empty")
			}
			if model.Description == "" {
				t.Error("description cannot be empty")
			}
			if model.ContextWindow <= 0 {
				t.Error("context window must be positive")
			}
			if model.MaxOutputTokens <= 0 {
				t.Error("max output tokens must be positive")
			}
		})
	}
}

func TestOnlyOneDefaultModel(t *testing.T) {
	registry := NewRegistry()
	models := registry.List()

	defaultCount := 0
	for _, model := range models {
		if model.IsDefault {
			defaultCount++
		}
	}

	if defaultCount != 1 {
		t.Errorf("expected exactly 1 default model, got %d", defaultCount)
	}
}

func TestModelCapabilities(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		modelID           string
		expectVision      bool
		expectTools       bool
		expectReasoning   bool
	}{
		{
			modelID:         "gpt-5-codex",
			expectVision:    true,
			expectTools:     true,
			expectReasoning: true,
		},
		{
			modelID:         "gpt-5",
			expectVision:    true,
			expectTools:     true,
			expectReasoning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			model := registry.Get(tt.modelID)
			if model == nil {
				t.Fatalf("expected to find model %s", tt.modelID)
			}

			if model.Capabilities.SupportsVision != tt.expectVision {
				t.Errorf("expected SupportsVision=%v, got %v", tt.expectVision, model.Capabilities.SupportsVision)
			}
			if model.Capabilities.SupportsTools != tt.expectTools {
				t.Errorf("expected SupportsTools=%v, got %v", tt.expectTools, model.Capabilities.SupportsTools)
			}
			if model.Capabilities.SupportsReasoning != tt.expectReasoning {
				t.Errorf("expected SupportsReasoning=%v, got %v", tt.expectReasoning, model.Capabilities.SupportsReasoning)
			}
		})
	}
}

func TestSupportedModels(t *testing.T) {
	models := SupportedModels()
	if len(models) == 0 {
		t.Fatal("expected at least one supported model")
	}
}

func TestGetModel(t *testing.T) {
	model, err := GetModel("gpt-5-codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model == nil {
		t.Fatal("expected model but got nil")
	}
	if model.ID != "gpt-5-codex" {
		t.Errorf("expected gpt-5-codex, got %s", model.ID)
	}

	_, err = GetModel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestGetDefaultModel(t *testing.T) {
	model := GetDefaultModel()
	if model == nil {
		t.Fatal("expected default model")
	}
	if !model.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name      string
		modelID   string
		expectDefault bool
		expectErr bool
	}{
		{
			name:          "empty resolves to default",
			modelID:       "",
			expectDefault: true,
			expectErr:     false,
		},
		{
			name:          "valid model ID",
			modelID:       "gpt-5",
			expectDefault: false,
			expectErr:     false,
		},
		{
			name:      "invalid model ID",
			modelID:   "nonexistent",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := ResolveModel(tt.modelID)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectDefault {
				if !model.IsDefault {
					t.Error("expected default model")
				}
			} else {
				if model.ID != tt.modelID {
					t.Errorf("expected %s, got %s", tt.modelID, model.ID)
				}
			}
		})
	}
}

func TestToModelInfo(t *testing.T) {
	model := &Model{
		ID:          "test",
		ModelSlug:   "test-v1",
		DisplayName: "Test",
		Description: "Test model",
		Capabilities: ModelCapabilities{
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsReasoning: true,
			SupportedReasoningEfforts: []ReasoningEffortOption{
				{Effort: ReasoningEffortLow, Description: "Low"},
				{Effort: ReasoningEffortHigh, Description: "High"},
			},
			DefaultReasoningEffort: ReasoningEffortLow,
		},
		ContextWindow:   100000,
		MaxOutputTokens: 4096,
		IsDefault:       false,
	}

	info := ToModelInfo(model)

	if info.ID != model.ID {
		t.Errorf("expected ID %s, got %s", model.ID, info.ID)
	}
	if info.Model != model.ModelSlug {
		t.Errorf("expected Model %s, got %s", model.ModelSlug, info.Model)
	}
	if !info.SupportsReasoning {
		t.Error("expected SupportsReasoning to be true")
	}
	if len(info.ReasoningEfforts) != 2 {
		t.Errorf("expected 2 reasoning efforts, got %d", len(info.ReasoningEfforts))
	}
	if info.DefaultReasoningEffort != ReasoningEffortLow {
		t.Errorf("expected default effort low, got %s", info.DefaultReasoningEffort)
	}
}

func TestListModelsInfo(t *testing.T) {
	infos := ListModelsInfo()
	if len(infos) == 0 {
		t.Fatal("expected at least one model info")
	}

	// Check that all required fields are populated
	for _, info := range infos {
		if info.ID == "" {
			t.Error("model info ID cannot be empty")
		}
		if info.Model == "" {
			t.Error("model info Model cannot be empty")
		}
		if info.DisplayName == "" {
			t.Error("model info DisplayName cannot be empty")
		}
	}
}
