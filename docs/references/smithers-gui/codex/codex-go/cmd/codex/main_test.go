package main

import (
	"testing"
)

func TestDetermineApprovalPolicy(t *testing.T) {
	tests := []struct {
		name           string
		autoApprove    bool
		approvalPolicy string
		want           string
	}{
		{
			name:           "auto-approve flag sets auto policy",
			autoApprove:    true,
			approvalPolicy: "",
			want:           "auto",
		},
		{
			name:           "auto-approve flag overrides policy flag",
			autoApprove:    true,
			approvalPolicy: "manual",
			want:           "auto",
		},
		{
			name:           "manual policy",
			autoApprove:    false,
			approvalPolicy: "manual",
			want:           "manual",
		},
		{
			name:           "semi-auto policy",
			autoApprove:    false,
			approvalPolicy: "semi-auto",
			want:           "semi-auto",
		},
		{
			name:           "auto policy",
			autoApprove:    false,
			approvalPolicy: "auto",
			want:           "auto",
		},
		{
			name:           "never policy",
			autoApprove:    false,
			approvalPolicy: "never",
			want:           "never",
		},
		{
			name:           "default to manual when no flags set",
			autoApprove:    false,
			approvalPolicy: "",
			want:           "manual",
		},
		{
			name:           "invalid policy defaults to manual",
			autoApprove:    false,
			approvalPolicy: "invalid",
			want:           "manual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flag values for test
			*autoApproveFlag = tt.autoApprove
			*approvalPolicyFlag = tt.approvalPolicy

			got := determineApprovalPolicy()
			if got != tt.want {
				t.Errorf("determineApprovalPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApprovalPolicyValidation(t *testing.T) {
	validPolicies := []string{"manual", "semi-auto", "auto", "never"}

	for _, policy := range validPolicies {
		*autoApproveFlag = false
		*approvalPolicyFlag = policy
		got := determineApprovalPolicy()
		if got != policy {
			t.Errorf("Valid policy %q not accepted, got %q", policy, got)
		}
	}
}
