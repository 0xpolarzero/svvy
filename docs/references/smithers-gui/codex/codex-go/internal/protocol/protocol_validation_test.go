package protocol

import (
	"strings"
	"testing"
)

// TestOpUserTurnValidation tests the validation of OpUserTurn
func TestOpUserTurnValidation(t *testing.T) {
	tests := []struct {
		name    string
		op      *OpUserTurn
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid turn",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
				Summary:        SummaryLow,
			},
			wantErr: false,
		},
		{
			name: "empty items",
			op: &OpUserTurn{
				Items:          []UserInput{},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "items cannot be empty",
		},
		{
			name: "too many items",
			op: &OpUserTurn{
				Items:          make([]UserInput, 101),
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "items exceeds maximum of 100",
		},
		{
			name: "invalid item",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: "invalid-type"},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "item 0",
		},
		{
			name: "empty cwd",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "cwd cannot be empty",
		},
		{
			name: "invalid approval policy",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: "invalid-policy",
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "invalid approval_policy",
		},
		{
			name: "invalid sandbox policy",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: "invalid-mode"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: true,
			errMsg:  "sandbox_policy",
		},
		{
			name: "empty model",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "",
			},
			wantErr: true,
			errMsg:  "model cannot be empty",
		},
		{
			name: "invalid summary",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
				Summary:        "invalid-summary",
			},
			wantErr: true,
			errMsg:  "invalid summary",
		},
		{
			name: "valid with all approval policies",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAlways,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: false,
		},
		{
			name: "valid with never approval policy",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyNever,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
			},
			wantErr: false,
		},
		{
			name: "valid with all summaries",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: UserInputTypeText, Text: strPtr("hello")},
				},
				Cwd:            "/test",
				ApprovalPolicy: ApprovalPolicyAuto,
				SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
				Model:          "claude-3-5-sonnet-20241022",
				Summary:        SummaryHigh,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestOpExecApprovalValidation tests the validation of OpExecApproval
func TestOpExecApprovalValidation(t *testing.T) {
	tests := []struct {
		name    string
		op      *OpExecApproval
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid approval",
			op: &OpExecApproval{
				ID:       "test-id",
				Decision: DecisionApproved,
			},
			wantErr: false,
		},
		{
			name: "valid approved for session",
			op: &OpExecApproval{
				ID:       "test-id",
				Decision: DecisionApprovedForSession,
			},
			wantErr: false,
		},
		{
			name: "valid denied",
			op: &OpExecApproval{
				ID:       "test-id",
				Decision: DecisionDenied,
			},
			wantErr: false,
		},
		{
			name: "valid abort",
			op: &OpExecApproval{
				ID:       "test-id",
				Decision: DecisionAbort,
			},
			wantErr: false,
		},
		{
			name: "empty id",
			op: &OpExecApproval{
				ID:       "",
				Decision: DecisionApproved,
			},
			wantErr: true,
			errMsg:  "id cannot be empty",
		},
		{
			name: "invalid decision",
			op: &OpExecApproval{
				ID:       "test-id",
				Decision: "invalid-decision",
			},
			wantErr: true,
			errMsg:  "invalid decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestOpPatchApprovalValidation tests the validation of OpPatchApproval
func TestOpPatchApprovalValidation(t *testing.T) {
	tests := []struct {
		name    string
		op      *OpPatchApproval
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid approval",
			op: &OpPatchApproval{
				ID:       "test-id",
				Decision: DecisionApproved,
			},
			wantErr: false,
		},
		{
			name: "valid approved for session",
			op: &OpPatchApproval{
				ID:       "test-id",
				Decision: DecisionApprovedForSession,
			},
			wantErr: false,
		},
		{
			name: "valid denied",
			op: &OpPatchApproval{
				ID:       "test-id",
				Decision: DecisionDenied,
			},
			wantErr: false,
		},
		{
			name: "valid abort",
			op: &OpPatchApproval{
				ID:       "test-id",
				Decision: DecisionAbort,
			},
			wantErr: false,
		},
		{
			name: "empty id",
			op: &OpPatchApproval{
				ID:       "",
				Decision: DecisionApproved,
			},
			wantErr: true,
			errMsg:  "id cannot be empty",
		},
		{
			name: "invalid decision",
			op: &OpPatchApproval{
				ID:       "test-id",
				Decision: "maybe-later",
			},
			wantErr: true,
			errMsg:  "invalid decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestUserInputValidation tests the validation of UserInput
func TestUserInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   UserInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid text",
			input: UserInput{
				Type: UserInputTypeText,
				Text: strPtr("hello world"),
			},
			wantErr: false,
		},
		{
			name: "valid image url",
			input: UserInput{
				Type:     UserInputTypeImageURL,
				ImageURL: strPtr("https://example.com/image.png"),
			},
			wantErr: false,
		},
		{
			name: "valid path",
			input: UserInput{
				Type: UserInputTypePath,
				Path: strPtr("/path/to/file"),
			},
			wantErr: false,
		},
		{
			name: "text type missing text field",
			input: UserInput{
				Type: UserInputTypeText,
			},
			wantErr: true,
			errMsg:  "text is required for type 'text'",
		},
		{
			name: "image_url type missing image_url field",
			input: UserInput{
				Type: UserInputTypeImageURL,
			},
			wantErr: true,
			errMsg:  "image_url is required for type 'image_url'",
		},
		{
			name: "path type missing path field",
			input: UserInput{
				Type: UserInputTypePath,
			},
			wantErr: true,
			errMsg:  "path is required for type 'path'",
		},
		{
			name: "invalid type",
			input: UserInput{
				Type: "invalid-type",
				Text: strPtr("hello"),
			},
			wantErr: true,
			errMsg:  "invalid type",
		},
		{
			name: "text exceeds max size",
			input: UserInput{
				Type: UserInputTypeText,
				Text: strPtr(strings.Repeat("a", MaxUserInputTextSize+1)),
			},
			wantErr: true,
			errMsg:  "text exceeds maximum size",
		},
		{
			name: "image_url exceeds max size",
			input: UserInput{
				Type:     UserInputTypeImageURL,
				ImageURL: strPtr(strings.Repeat("a", MaxUserInputPathSize+1)),
			},
			wantErr: true,
			errMsg:  "image_url exceeds maximum size",
		},
		{
			name: "path exceeds max size",
			input: UserInput{
				Type: UserInputTypePath,
				Path: strPtr(strings.Repeat("a", MaxUserInputPathSize+1)),
			},
			wantErr: true,
			errMsg:  "path exceeds maximum size",
		},
		{
			name: "text at max size",
			input: UserInput{
				Type: UserInputTypeText,
				Text: strPtr(strings.Repeat("a", MaxUserInputTextSize)),
			},
			wantErr: false,
		},
		{
			name: "path at max size",
			input: UserInput{
				Type: UserInputTypePath,
				Path: strPtr(strings.Repeat("a", MaxUserInputPathSize)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestSandboxPolicyValidation tests the validation of SandboxPolicy
func TestSandboxPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  SandboxPolicy
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid read-only",
			policy: SandboxPolicy{
				Mode: SandboxModeReadOnly,
			},
			wantErr: false,
		},
		{
			name: "valid workspace-write",
			policy: SandboxPolicy{
				Mode:          SandboxModeWorkspaceWrite,
				WritableRoots: []string{"/tmp", "/var/tmp"},
			},
			wantErr: false,
		},
		{
			name: "valid workspace-write with empty writable roots",
			policy: SandboxPolicy{
				Mode:          SandboxModeWorkspaceWrite,
				WritableRoots: []string{},
			},
			wantErr: false,
		},
		{
			name: "valid danger-full-access",
			policy: SandboxPolicy{
				Mode: SandboxModeDangerFullAccess,
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			policy: SandboxPolicy{
				Mode: "invalid-mode",
			},
			wantErr: true,
			errMsg:  "invalid mode",
		},
		{
			name: "workspace-write with empty string in writable roots",
			policy: SandboxPolicy{
				Mode:          SandboxModeWorkspaceWrite,
				WritableRoots: []string{"/tmp", "", "/var/tmp"},
			},
			wantErr: true,
			errMsg:  "writable_roots[1] cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestConstantsDefinedCorrectly verifies that all constants are correctly defined
func TestConstantsDefinedCorrectly(t *testing.T) {
	// Test Decision constants
	decisions := []string{DecisionApproved, DecisionApprovedForSession, DecisionDenied, DecisionAbort}
	expectedDecisions := []string{"approved", "approved_for_session", "denied", "abort"}
	for i, d := range decisions {
		if d != expectedDecisions[i] {
			t.Errorf("Decision constant mismatch: got %q, want %q", d, expectedDecisions[i])
		}
	}

	// Test ApprovalPolicy constants
	policies := []string{ApprovalPolicyAuto, ApprovalPolicyAlways, ApprovalPolicyNever}
	expectedPolicies := []string{"auto", "always", "never"}
	for i, p := range policies {
		if p != expectedPolicies[i] {
			t.Errorf("ApprovalPolicy constant mismatch: got %q, want %q", p, expectedPolicies[i])
		}
	}

	// Test SandboxMode constants
	modes := []string{SandboxModeReadOnly, SandboxModeWorkspaceWrite, SandboxModeDangerFullAccess}
	expectedModes := []string{"read-only", "workspace-write", "danger-full-access"}
	for i, m := range modes {
		if m != expectedModes[i] {
			t.Errorf("SandboxMode constant mismatch: got %q, want %q", m, expectedModes[i])
		}
	}

	// Test Summary constants
	summaries := []string{SummaryNone, SummaryLow, SummaryMedium, SummaryHigh}
	expectedSummaries := []string{"none", "low", "medium", "high"}
	for i, s := range summaries {
		if s != expectedSummaries[i] {
			t.Errorf("Summary constant mismatch: got %q, want %q", s, expectedSummaries[i])
		}
	}

	// Test UserInputType constants
	types := []string{UserInputTypeText, UserInputTypeImageURL, UserInputTypePath}
	expectedTypes := []string{"text", "image_url", "path"}
	for i, typ := range types {
		if typ != expectedTypes[i] {
			t.Errorf("UserInputType constant mismatch: got %q, want %q", typ, expectedTypes[i])
		}
	}

	// Test size constants
	if MaxUserInputTextSize != 1*1024*1024 {
		t.Errorf("MaxUserInputTextSize mismatch: got %d, want %d", MaxUserInputTextSize, 1*1024*1024)
	}
	if MaxUserInputPathSize != 256*1024 {
		t.Errorf("MaxUserInputPathSize mismatch: got %d, want %d", MaxUserInputPathSize, 256*1024)
	}
}

// TestValidationEdgeCases tests edge cases for validation
func TestValidationEdgeCases(t *testing.T) {
	t.Run("OpUserTurn with exactly 100 items", func(t *testing.T) {
		items := make([]UserInput, 100)
		for i := 0; i < 100; i++ {
			items[i] = UserInput{
				Type: UserInputTypeText,
				Text: strPtr("test"),
			}
		}
		op := &OpUserTurn{
			Items:          items,
			Cwd:            "/test",
			ApprovalPolicy: ApprovalPolicyAuto,
			SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
			Model:          "claude-3-5-sonnet-20241022",
		}
		if err := op.Validate(); err != nil {
			t.Errorf("expected no error for 100 items, got %v", err)
		}
	})

	t.Run("OpUserTurn with empty approval policy is valid", func(t *testing.T) {
		op := &OpUserTurn{
			Items: []UserInput{
				{Type: UserInputTypeText, Text: strPtr("hello")},
			},
			Cwd:            "/test",
			ApprovalPolicy: "",
			SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
			Model:          "claude-3-5-sonnet-20241022",
		}
		if err := op.Validate(); err != nil {
			t.Errorf("expected no error for empty approval policy, got %v", err)
		}
	})

	t.Run("OpUserTurn with empty summary is valid", func(t *testing.T) {
		op := &OpUserTurn{
			Items: []UserInput{
				{Type: UserInputTypeText, Text: strPtr("hello")},
			},
			Cwd:            "/test",
			ApprovalPolicy: ApprovalPolicyAuto,
			SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
			Model:          "claude-3-5-sonnet-20241022",
			Summary:        "",
		}
		if err := op.Validate(); err != nil {
			t.Errorf("expected no error for empty summary, got %v", err)
		}
	})

	t.Run("SandboxPolicy validates all modes", func(t *testing.T) {
		modes := []string{SandboxModeReadOnly, SandboxModeWorkspaceWrite, SandboxModeDangerFullAccess}
		for _, mode := range modes {
			policy := SandboxPolicy{Mode: mode}
			if err := policy.Validate(); err != nil {
				t.Errorf("expected no error for mode %q, got %v", mode, err)
			}
		}
	})
}

// TestDoSProtection tests that validation protects against DoS attacks
func TestDoSProtection(t *testing.T) {
	t.Run("prevents oversized text input", func(t *testing.T) {
		input := UserInput{
			Type: UserInputTypeText,
			Text: strPtr(strings.Repeat("a", 10*1024*1024)), // 10MB
		}
		err := input.Validate()
		if err == nil {
			t.Error("expected error for oversized text, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Errorf("expected size error, got %v", err)
		}
	})

	t.Run("prevents too many items", func(t *testing.T) {
		items := make([]UserInput, 1000)
		for i := 0; i < 1000; i++ {
			items[i] = UserInput{
				Type: UserInputTypeText,
				Text: strPtr("test"),
			}
		}
		op := &OpUserTurn{
			Items:          items,
			Cwd:            "/test",
			ApprovalPolicy: ApprovalPolicyAuto,
			SandboxPolicy:  SandboxPolicy{Mode: SandboxModeReadOnly},
			Model:          "claude-3-5-sonnet-20241022",
		}
		err := op.Validate()
		if err == nil {
			t.Error("expected error for too many items, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("expected maximum exceeded error, got %v", err)
		}
	})
}
