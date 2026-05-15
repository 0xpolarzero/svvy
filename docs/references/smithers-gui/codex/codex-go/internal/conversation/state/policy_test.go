package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPolicy(t *testing.T) {
	t.Run("creates policy with defaults", func(t *testing.T) {
		policy := NewPolicy()

		require.NotNil(t, policy)
		assert.True(t, policy.RequireToolApproval)
		assert.Equal(t, int64(100000), policy.MaxTokensPerTurn)
		assert.Equal(t, 100, policy.MaxMessagesInHistory)
	})
}

func TestNewPolicyWithOptions(t *testing.T) {
	t.Run("creates policy with custom options", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			RequireToolApproval:  false,
			MaxTokensPerTurn:     50000,
			MaxMessagesInHistory: 200,
			AllowedTools:         []string{"read_file", "write_file"},
			BlockedTools:         []string{"exec"},
		})

		assert.False(t, policy.RequireToolApproval)
		assert.Equal(t, int64(50000), policy.MaxTokensPerTurn)
		assert.Equal(t, 200, policy.MaxMessagesInHistory)
		assert.Len(t, policy.AllowedTools, 2)
		assert.Len(t, policy.BlockedTools, 1)
	})
}

func TestPolicy_ValidateToolCall(t *testing.T) {
	t.Run("allows tool when not blocked", func(t *testing.T) {
		policy := NewPolicy()

		err := policy.ValidateToolCall("read_file", map[string]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("blocks tool in blocked list", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			BlockedTools: []string{"exec", "delete"},
		})

		err := policy.ValidateToolCall("exec", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})

	t.Run("allows tool in allowed list", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			AllowedTools: []string{"read_file", "write_file"},
		})

		err := policy.ValidateToolCall("read_file", map[string]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("blocks tool not in allowed list", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			AllowedTools: []string{"read_file", "write_file"},
		})

		err := policy.ValidateToolCall("exec", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed list")
	})

	t.Run("blocked list takes precedence over allowed list", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			AllowedTools: []string{"read_file", "write_file", "exec"},
			BlockedTools: []string{"exec"},
		})

		err := policy.ValidateToolCall("exec", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})
}

func TestPolicy_ValidateTokenUsage(t *testing.T) {
	t.Run("allows usage within limit", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 100000,
		})

		usage := TokenUsage{
			InputTokens:  50000,
			OutputTokens: 30000,
			TotalTokens:  80000,
		}

		err := policy.ValidateTokenUsage(usage)
		assert.NoError(t, err)
	})

	t.Run("rejects usage exceeding limit", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 100000,
		})

		usage := TokenUsage{
			InputTokens:  80000,
			OutputTokens: 50000,
			TotalTokens:  130000,
		}

		err := policy.ValidateTokenUsage(usage)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("allows any usage when limit is 0", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 0, // No limit
		})

		usage := TokenUsage{
			TotalTokens: 1000000,
		}

		err := policy.ValidateTokenUsage(usage)
		assert.NoError(t, err)
	})
}

func TestPolicy_ValidateMessageHistory(t *testing.T) {
	t.Run("allows history within limit", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxMessagesInHistory: 100,
		})

		history := NewMessageHistory()
		for i := 0; i < 50; i++ {
			history.Append(Message{
				Role:    "user",
				Content: "test",
			})
		}

		err := policy.ValidateMessageHistory(history)
		assert.NoError(t, err)
	})

	t.Run("rejects history exceeding limit", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxMessagesInHistory: 10,
		})

		history := NewMessageHistory()
		for i := 0; i < 15; i++ {
			history.Append(Message{
				Role:    "user",
				Content: "test",
			})
		}

		err := policy.ValidateMessageHistory(history)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("allows any size when limit is 0", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxMessagesInHistory: 0, // No limit
		})

		history := NewMessageHistory()
		for i := 0; i < 1000; i++ {
			history.Append(Message{
				Role:    "user",
				Content: "test",
			})
		}

		err := policy.ValidateMessageHistory(history)
		assert.NoError(t, err)
	})
}

func TestPolicy_ShouldApproveToolCall(t *testing.T) {
	t.Run("requires approval when policy enabled", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			RequireToolApproval: true,
		})

		assert.True(t, policy.ShouldApproveToolCall("read_file"))
	})

	t.Run("does not require approval when policy disabled", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			RequireToolApproval: false,
		})

		assert.False(t, policy.ShouldApproveToolCall("read_file"))
	})

	t.Run("requires approval for dangerous tools even when disabled", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			RequireToolApproval: false,
			DangerousTools:      []string{"exec", "delete"},
		})

		assert.True(t, policy.ShouldApproveToolCall("exec"))
		assert.True(t, policy.ShouldApproveToolCall("delete"))
		assert.False(t, policy.ShouldApproveToolCall("read_file"))
	})
}

func TestPolicy_Clone(t *testing.T) {
	t.Run("creates independent copy", func(t *testing.T) {
		original := NewPolicyWithOptions(PolicyOptions{
			RequireToolApproval:  true,
			MaxTokensPerTurn:     100000,
			MaxMessagesInHistory: 100,
			AllowedTools:         []string{"read_file"},
			BlockedTools:         []string{"exec"},
		})

		clone := original.Clone()

		// Modify clone
		clone.RequireToolApproval = false
		clone.MaxTokensPerTurn = 50000
		clone.AllowedTools = append(clone.AllowedTools, "write_file")

		// Original should be unchanged
		assert.True(t, original.RequireToolApproval)
		assert.Equal(t, int64(100000), original.MaxTokensPerTurn)
		assert.Len(t, original.AllowedTools, 1)
	})
}

func TestPolicyEnforcer(t *testing.T) {
	t.Run("creates enforcer with policy", func(t *testing.T) {
		policy := NewPolicy()
		enforcer := NewPolicyEnforcer(policy)

		require.NotNil(t, enforcer)
		assert.NotNil(t, enforcer.Policy())
	})

	t.Run("tracks violations", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 100000,
		})
		enforcer := NewPolicyEnforcer(policy)

		usage := TokenUsage{
			TotalTokens: 150000,
		}

		err := enforcer.EnforceTokenUsage(usage)
		assert.Error(t, err)

		violations := enforcer.Violations()
		assert.Len(t, violations, 1)
		assert.Contains(t, violations[0].Message, "token")
	})

	t.Run("tracks multiple violations", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn:     100000,
			MaxMessagesInHistory: 10,
			BlockedTools:         []string{"exec"},
		})
		enforcer := NewPolicyEnforcer(policy)

		// Token violation
		enforcer.EnforceTokenUsage(TokenUsage{TotalTokens: 150000})

		// Tool violation
		enforcer.EnforceToolCall("exec", map[string]interface{}{})

		// Message history violation
		history := NewMessageHistory()
		for i := 0; i < 15; i++ {
			history.Append(Message{Role: "user", Content: "test"})
		}
		enforcer.EnforceMessageHistory(history)

		violations := enforcer.Violations()
		assert.Len(t, violations, 3)
	})

	t.Run("clears violations", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 100000,
		})
		enforcer := NewPolicyEnforcer(policy)

		enforcer.EnforceTokenUsage(TokenUsage{TotalTokens: 150000})
		assert.Len(t, enforcer.Violations(), 1)

		enforcer.ClearViolations()
		assert.Empty(t, enforcer.Violations())
	})
}

func TestPolicyViolation(t *testing.T) {
	t.Run("creates violation with details", func(t *testing.T) {
		violation := PolicyViolation{
			Type:     "token_limit",
			Message:  "Token usage exceeds limit",
			Severity: "error",
			Details: map[string]interface{}{
				"used":  150000,
				"limit": 100000,
			},
		}

		assert.Equal(t, "token_limit", violation.Type)
		assert.Equal(t, "error", violation.Severity)
		assert.NotNil(t, violation.Details)
	})
}

func TestPolicyEnforcer_ThreadSafety(t *testing.T) {
	t.Run("concurrent enforcement", func(t *testing.T) {
		policy := NewPolicyWithOptions(PolicyOptions{
			MaxTokensPerTurn: 100000,
		})
		enforcer := NewPolicyEnforcer(policy)

		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func(n int) {
				usage := TokenUsage{
					TotalTokens: int64(50000 + n*1000),
				}
				enforcer.EnforceTokenUsage(usage)
				done <- true
			}(i)
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		// Some should have violated
		violations := enforcer.Violations()
		assert.NotEmpty(t, violations)
	})
}

func TestPolicy_CustomValidator(t *testing.T) {
	t.Run("adds custom validator", func(t *testing.T) {
		policy := NewPolicy()

		validator := func(toolName string, args map[string]interface{}) error {
			if toolName == "custom_tool" {
				return nil
			}
			return nil
		}

		policy.AddCustomValidator(validator)

		err := policy.ValidateToolCall("custom_tool", map[string]interface{}{})
		assert.NoError(t, err)
	})
}
