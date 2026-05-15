package runtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeCommandSafety tests the main safety analysis function
func TestAnalyzeCommandSafety(t *testing.T) {
	tests := []struct {
		name             string
		command          []string
		workingDir       string
		expectedLevel    SafetyLevel
		expectedApproval bool
		expectedRisk     RiskLevel
	}{
		{
			name:             "empty command",
			command:          []string{},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "always safe command - ls",
			command:          []string{"ls", "-la"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "always safe command - cat",
			command:          []string{"cat", "file.txt"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "shell wrapped safe command",
			command:          []string{"sh", "-c", "ls -la"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "multiple safe commands",
			command:          []string{"sh", "-c", "ls && pwd && echo hi"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "rm without flags",
			command:          []string{"rm", "file.txt"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskMedium,
		},
		{
			name:             "rm -rf (critical)",
			command:          []string{"rm", "-rf", "/tmp/test"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "rm -r (high risk)",
			command:          []string{"rm", "-r", "dir"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskHigh,
		},
		{
			name:             "rm -f (high risk)",
			command:          []string{"rm", "-f", "file.txt"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskHigh,
		},
		{
			name:             "chmod 777 (high risk)",
			command:          []string{"chmod", "777", "file.txt"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskHigh,
		},
		{
			name:             "chmod 755",
			command:          []string{"chmod", "755", "script.sh"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskMedium,
		},
		{
			name:             "find without dangerous flags",
			command:          []string{"find", ".", "-name", "*.txt"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "find with -delete",
			command:          []string{"find", ".", "-name", "*.txt", "-delete"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskHigh,
		},
		{
			name:             "find with -exec",
			command:          []string{"find", ".", "-name", "*.txt", "-exec", "rm", "{}", ";"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "git status (safe)",
			command:          []string{"git", "status"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyAlwaysSafe,
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "git reset (dangerous)",
			command:          []string{"git", "reset", "--hard"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskHigh,
		},
		{
			name:             "always unsafe - sudo",
			command:          []string{"sudo", "apt", "install", "package"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "always unsafe - curl",
			command:          []string{"curl", "https://example.com"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "shell wrapped dangerous command",
			command:          []string{"sh", "-c", "rm -rf /"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "safe command followed by dangerous",
			command:          []string{"sh", "-c", "ls && rm -rf dir"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "unknown command",
			command:          []string{"unknown-command", "arg"},
			workingDir:       "/tmp",
			expectedLevel:    SafetyUnsafe,
			expectedApproval: true,
			expectedRisk:     RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := AnalyzeCommandSafety(tt.command, tt.workingDir)
			require.NotNil(t, analysis)
			assert.Equal(t, tt.expectedLevel, analysis.Level, "Safety level mismatch")
			assert.Equal(t, tt.expectedApproval, analysis.RequiresApproval, "Approval requirement mismatch")
			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel, "Risk level mismatch")
		})
	}
}

// TestAnalyzeRmCommand tests rm command analysis
func TestAnalyzeRmCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedRisk RiskLevel
		reasonContains string
	}{
		{
			name:           "rm without flags",
			command:        []string{"rm", "file.txt"},
			expectedRisk:   RiskMedium,
			reasonContains: "file deletion",
		},
		{
			name:           "rm -rf",
			command:        []string{"rm", "-rf", "/tmp/test"},
			expectedRisk:   RiskCritical,
			reasonContains: "rm -rf",
		},
		{
			name:           "rm -fr (reversed flags)",
			command:        []string{"rm", "-fr", "/tmp/test"},
			expectedRisk:   RiskCritical,
			reasonContains: "rm -rf",
		},
		{
			name:           "rm -r",
			command:        []string{"rm", "-r", "dir"},
			expectedRisk:   RiskHigh,
			reasonContains: "recursive deletion",
		},
		{
			name:           "rm -f",
			command:        []string{"rm", "-f", "file.txt"},
			expectedRisk:   RiskHigh,
			reasonContains: "forced deletion",
		},
		{
			name:           "rm --recursive --force",
			command:        []string{"rm", "--recursive", "--force", "dir"},
			expectedRisk:   RiskCritical,
			reasonContains: "rm -rf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeRmCommand(tt.command)
			require.NotNil(t, analysis)
			assert.Equal(t, SafetyUnsafe, analysis.Level)
			assert.True(t, analysis.RequiresApproval)
			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)

			found := false
			for _, reason := range analysis.Reasons {
				if containsIgnoreCase(reason, tt.reasonContains) {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected reason to contain: %s, got: %v", tt.reasonContains, analysis.Reasons)
		})
	}
}

// TestAnalyzeChmodCommand tests chmod command analysis
func TestAnalyzeChmodCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedRisk RiskLevel
		reasonContains string
	}{
		{
			name:           "chmod 777",
			command:        []string{"chmod", "777", "file.txt"},
			expectedRisk:   RiskHigh,
			reasonContains: "chmod 777",
		},
		{
			name:           "chmod 755",
			command:        []string{"chmod", "755", "script.sh"},
			expectedRisk:   RiskMedium,
			reasonContains: "execute permissions",
		},
		{
			name:           "chmod 644",
			command:        []string{"chmod", "644", "file.txt"},
			expectedRisk:   RiskMedium,
			reasonContains: "permission modification",
		},
		{
			name:           "chmod a+rwx",
			command:        []string{"chmod", "a+rwx", "file.txt"},
			expectedRisk:   RiskHigh,
			reasonContains: "world-writable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeChmodCommand(tt.command)
			require.NotNil(t, analysis)
			assert.Equal(t, SafetyUnsafe, analysis.Level)
			assert.True(t, analysis.RequiresApproval)
			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)

			found := false
			for _, reason := range analysis.Reasons {
				if containsIgnoreCase(reason, tt.reasonContains) {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected reason to contain: %s, got: %v", tt.reasonContains, analysis.Reasons)
		})
	}
}

// TestAnalyzeFindCommand tests find command analysis
func TestAnalyzeFindCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedSafe bool
		expectedRisk RiskLevel
	}{
		{
			name:         "find without dangerous flags",
			command:      []string{"find", ".", "-name", "*.txt"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "find with -type",
			command:      []string{"find", "/tmp", "-type", "f"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "find with -delete",
			command:      []string{"find", ".", "-name", "*.tmp", "-delete"},
			expectedSafe: false,
			expectedRisk: RiskHigh,
		},
		{
			name:         "find with -exec",
			command:      []string{"find", ".", "-exec", "rm", "{}", ";"},
			expectedSafe: false,
			expectedRisk: RiskCritical,
		},
		{
			name:         "find with -execdir",
			command:      []string{"find", ".", "-execdir", "cat", "{}", ";"},
			expectedSafe: false,
			expectedRisk: RiskCritical,
		},
		{
			name:         "find with -ok",
			command:      []string{"find", ".", "-ok", "rm", "{}", ";"},
			expectedSafe: false,
			expectedRisk: RiskCritical,
		},
		{
			name:         "find with -fprint",
			command:      []string{"find", ".", "-fprint", "/etc/passwd"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeFindCommand(tt.command)
			require.NotNil(t, analysis)

			if tt.expectedSafe {
				assert.Equal(t, SafetyAlwaysSafe, analysis.Level)
				assert.False(t, analysis.RequiresApproval)
			} else {
				assert.Equal(t, SafetyUnsafe, analysis.Level)
				assert.True(t, analysis.RequiresApproval)
			}

			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)
		})
	}
}

// TestAnalyzeGitCommand tests git command analysis
func TestAnalyzeGitCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedSafe bool
		expectedRisk RiskLevel
	}{
		{
			name:         "git status",
			command:      []string{"git", "status"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "git log",
			command:      []string{"git", "log", "--oneline"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "git diff",
			command:      []string{"git", "diff"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "git show",
			command:      []string{"git", "show", "HEAD"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "git branch",
			command:      []string{"git", "branch", "-a"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "git reset",
			command:      []string{"git", "reset", "--hard"},
			expectedSafe: false,
			expectedRisk: RiskHigh,
		},
		{
			name:         "git clean",
			command:      []string{"git", "clean", "-fd"},
			expectedSafe: false,
			expectedRisk: RiskHigh,
		},
		{
			name:         "git rm",
			command:      []string{"git", "rm", "file.txt"},
			expectedSafe: false,
			expectedRisk: RiskHigh,
		},
		{
			name:         "git push",
			command:      []string{"git", "push", "origin", "main"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
		{
			name:         "git pull",
			command:      []string{"git", "pull"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
		{
			name:         "git rebase",
			command:      []string{"git", "rebase", "main"},
			expectedSafe: false,
			expectedRisk: RiskHigh,
		},
		{
			name:         "git commit",
			command:      []string{"git", "commit", "-m", "message"},
			expectedSafe: false,
			expectedRisk: RiskLow,
		},
		{
			name:         "git without subcommand",
			command:      []string{"git"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeGitCommand(tt.command)
			require.NotNil(t, analysis)

			if tt.expectedSafe {
				assert.Equal(t, SafetyAlwaysSafe, analysis.Level)
				assert.False(t, analysis.RequiresApproval)
			} else {
				assert.NotEqual(t, SafetyAlwaysSafe, analysis.Level)
				assert.True(t, analysis.RequiresApproval)
			}

			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)
		})
	}
}

// TestAnalyzeSedCommand tests sed command analysis
func TestAnalyzeSedCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedSafe bool
		expectedRisk RiskLevel
	}{
		{
			name:         "sed -n Np (safe read-only)",
			command:      []string{"sed", "-n", "10p", "file.txt"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "sed -n M,Np (safe read-only)",
			command:      []string{"sed", "-n", "1,5p", "file.txt"},
			expectedSafe: true,
			expectedRisk: RiskLow,
		},
		{
			name:         "sed with -i (in-place edit)",
			command:      []string{"sed", "-i", "s/old/new/g", "file.txt"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
		{
			name:         "sed with -i.bak",
			command:      []string{"sed", "-i.bak", "s/old/new/g", "file.txt"},
			expectedSafe: false,
			expectedRisk: RiskMedium,
		},
		{
			name:         "sed substitution without -i",
			command:      []string{"sed", "s/old/new/g", "file.txt"},
			expectedSafe: false,
			expectedRisk: RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeSedCommand(tt.command)
			require.NotNil(t, analysis)

			if tt.expectedSafe {
				assert.Equal(t, SafetyAlwaysSafe, analysis.Level)
				assert.False(t, analysis.RequiresApproval)
			} else {
				assert.NotEqual(t, SafetyAlwaysSafe, analysis.Level)
			}

			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)
		})
	}
}

// TestAnalyzeRipgrepCommand tests ripgrep command analysis
func TestAnalyzeRipgrepCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		expectedSafe bool
	}{
		{
			name:         "rg basic search",
			command:      []string{"rg", "pattern", "file.txt"},
			expectedSafe: true,
		},
		{
			name:         "rg with flags",
			command:      []string{"rg", "-n", "-i", "pattern"},
			expectedSafe: true,
		},
		{
			name:         "rg with --pre",
			command:      []string{"rg", "--pre", "command", "pattern"},
			expectedSafe: false,
		},
		{
			name:         "rg with --pre=command",
			command:      []string{"rg", "--pre=command", "pattern"},
			expectedSafe: false,
		},
		{
			name:         "rg with --hostname-bin",
			command:      []string{"rg", "--hostname-bin", "command", "pattern"},
			expectedSafe: false,
		},
		{
			name:         "rg with --hostname-bin=command",
			command:      []string{"rg", "--hostname-bin=command", "pattern"},
			expectedSafe: false,
		},
		{
			name:         "rg with --search-zip",
			command:      []string{"rg", "--search-zip", "pattern"},
			expectedSafe: false,
		},
		{
			name:         "rg with -z",
			command:      []string{"rg", "-z", "pattern"},
			expectedSafe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeRipgrepCommand(tt.command)
			require.NotNil(t, analysis)

			if tt.expectedSafe {
				assert.Equal(t, SafetyAlwaysSafe, analysis.Level)
				assert.False(t, analysis.RequiresApproval)
			} else {
				assert.Equal(t, SafetyUnsafe, analysis.Level)
				assert.True(t, analysis.RequiresApproval)
			}
		})
	}
}

// TestIsValidSedNArg tests sed -n argument validation
func TestIsValidSedNArg(t *testing.T) {
	tests := []struct {
		arg      string
		expected bool
	}{
		{"10p", true},
		{"1,5p", true},
		{"100,200p", true},
		{"1p", true},
		{"xp", false},
		{"10", false},
		{"p", false},
		{"1,2,3p", false},
		{"", false},
		{",5p", false},
		{"1,p", false},
		{"a,bp", false},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			result := isValidSedNArg(tt.arg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidatePathSafety tests path validation
func TestValidatePathSafety(t *testing.T) {
	workingDir := "/workspace/project"

	tests := []struct {
		name       string
		path       string
		workingDir string
		expected   bool
	}{
		{
			name:       "relative path within workspace",
			path:       "src/main.go",
			workingDir: workingDir,
			expected:   true,
		},
		{
			name:       "absolute path within workspace",
			path:       "/workspace/project/src/main.go",
			workingDir: workingDir,
			expected:   true,
		},
		{
			name:       "absolute path outside workspace",
			path:       "/etc/passwd",
			workingDir: workingDir,
			expected:   false,
		},
		{
			name:       "relative path with ..",
			path:       "../project/file.txt",
			workingDir: workingDir,
			expected:   true,
		},
		{
			name:       "relative path escaping workspace",
			path:       "../../etc/passwd",
			workingDir: workingDir,
			expected:   false,
		},
		{
			name:       "current directory",
			path:       ".",
			workingDir: workingDir,
			expected:   true,
		},
		{
			name:       "parent directory",
			path:       "..",
			workingDir: workingDir,
			expected:   false, // Parent escapes workspace
		},
		{
			name:       "complex path that stays in workspace",
			path:       "src/../file.txt",
			workingDir: workingDir,
			expected:   true, // Resolves to /workspace/project/file.txt
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePathSafety(tt.path, tt.workingDir)
			assert.Equal(t, tt.expected, result, "Path: %s, WorkingDir: %s", tt.path, tt.workingDir)
		})
	}
}

// TestCreateRiskAssessment tests risk assessment creation
func TestCreateRiskAssessment(t *testing.T) {
	tests := []struct {
		name             string
		analysis         *CommandSafetyAnalysis
		expectedRiskLevel RiskLevel
		mitigationContains string
	}{
		{
			name:               "nil analysis",
			analysis:           nil,
			expectedRiskLevel:  RiskMedium,
			mitigationContains: "Sandbox restrictions",
		},
		{
			name: "low risk",
			analysis: &CommandSafetyAnalysis{
				Level:            SafetyAlwaysSafe,
				RequiresApproval: false,
				Reasons:          []string{"read-only operation"},
				RiskLevel:        RiskLow,
			},
			expectedRiskLevel:  RiskLow,
			mitigationContains: "read-only",
		},
		{
			name: "medium risk",
			analysis: &CommandSafetyAnalysis{
				Level:            SafetyConditional,
				RequiresApproval: true,
				Reasons:          []string{"file modification"},
				RiskLevel:        RiskMedium,
			},
			expectedRiskLevel:  RiskMedium,
			mitigationContains: "prevent access outside",
		},
		{
			name: "high risk",
			analysis: &CommandSafetyAnalysis{
				Level:            SafetyUnsafe,
				RequiresApproval: true,
				Reasons:          []string{"recursive deletion"},
				RiskLevel:        RiskHigh,
			},
			expectedRiskLevel:  RiskHigh,
			mitigationContains: "data loss may still occur",
		},
		{
			name: "critical risk",
			analysis: &CommandSafetyAnalysis{
				Level:            SafetyUnsafe,
				RequiresApproval: true,
				Reasons:          []string{"destructive operation"},
				RiskLevel:        RiskCritical,
			},
			expectedRiskLevel:  RiskCritical,
			mitigationContains: "cannot be safely sandboxed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assessment := CreateRiskAssessment(tt.analysis)
			require.NotNil(t, assessment)
			assert.Equal(t, tt.expectedRiskLevel, assessment.Level)
			assert.Contains(t, assessment.Mitigation, tt.mitigationContains)
		})
	}
}

// TestHasCommandChaining tests command chaining detection
func TestHasCommandChaining(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		expected bool
	}{
		{
			name:     "no chaining - simple command",
			command:  []string{"ls", "-la"},
			expected: false,
		},
		{
			name:     "chaining with &&",
			command:  []string{"sh", "-c", "ls && echo hi"},
			expected: true,
		},
		{
			name:     "chaining with ||",
			command:  []string{"sh", "-c", "ls || echo failed"},
			expected: true,
		},
		{
			name:     "chaining with semicolon",
			command:  []string{"sh", "-c", "ls ; echo done"},
			expected: true,
		},
		{
			name:     "chaining with pipe",
			command:  []string{"sh", "-c", "ls | grep test"},
			expected: true,
		},
		{
			name:     "chaining inside quotes (not chaining)",
			command:  []string{"sh", "-c", `echo "ls && rm"`},
			expected: false,
		},
		{
			name:     "chaining inside single quotes (not chaining)",
			command:  []string{"sh", "-c", `echo 'ls || rm'`},
			expected: false,
		},
		{
			name:     "no shell wrapper",
			command:  []string{"ls"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasCommandChaining(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSafetyLevelClassification tests safety level classification
func TestSafetyLevelClassification(t *testing.T) {
	tests := []struct {
		name          string
		command       []string
		expectedLevel SafetyLevel
	}{
		// Always safe commands
		{
			name:          "ls",
			command:       []string{"ls"},
			expectedLevel: SafetyAlwaysSafe,
		},
		{
			name:          "cat",
			command:       []string{"cat", "file.txt"},
			expectedLevel: SafetyAlwaysSafe,
		},
		{
			name:          "grep",
			command:       []string{"grep", "pattern", "file.txt"},
			expectedLevel: SafetyAlwaysSafe,
		},
		{
			name:          "pwd",
			command:       []string{"pwd"},
			expectedLevel: SafetyAlwaysSafe,
		},

		// Always unsafe commands
		{
			name:          "sudo",
			command:       []string{"sudo", "command"},
			expectedLevel: SafetyUnsafe,
		},
		{
			name:          "curl",
			command:       []string{"curl", "https://example.com"},
			expectedLevel: SafetyUnsafe,
		},
		{
			name:          "dd",
			command:       []string{"dd", "if=/dev/zero", "of=file"},
			expectedLevel: SafetyUnsafe,
		},

		// Conditional commands
		{
			name:          "rm (unsafe)",
			command:       []string{"rm", "file.txt"},
			expectedLevel: SafetyUnsafe,
		},
		{
			name:          "chmod (unsafe)",
			command:       []string{"chmod", "755", "file.txt"},
			expectedLevel: SafetyUnsafe,
		},
		{
			name:          "find safe",
			command:       []string{"find", ".", "-name", "*.txt"},
			expectedLevel: SafetyAlwaysSafe,
		},
		{
			name:          "find unsafe",
			command:       []string{"find", ".", "-delete"},
			expectedLevel: SafetyUnsafe,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := AnalyzeCommandSafety(tt.command, "/tmp")
			require.NotNil(t, analysis)
			assert.Equal(t, tt.expectedLevel, analysis.Level)
		})
	}
}

// TestComplexCommandChains tests complex command chain analysis
func TestComplexCommandChains(t *testing.T) {
	tests := []struct {
		name             string
		command          []string
		expectedApproval bool
		expectedRisk     RiskLevel
	}{
		{
			name:             "all safe commands",
			command:          []string{"sh", "-c", "ls && pwd && echo hi"},
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
		{
			name:             "safe then unsafe",
			command:          []string{"sh", "-c", "ls && rm file.txt"},
			expectedApproval: true,
			expectedRisk:     RiskMedium,
		},
		{
			name:             "unsafe then safe",
			command:          []string{"sh", "-c", "rm file.txt && ls"},
			expectedApproval: true,
			expectedRisk:     RiskMedium,
		},
		{
			name:             "safe with critical in chain",
			command:          []string{"sh", "-c", "ls && rm -rf / && pwd"},
			expectedApproval: true,
			expectedRisk:     RiskCritical,
		},
		{
			name:             "piped safe commands",
			command:          []string{"sh", "-c", "ls | grep test | wc -l"},
			expectedApproval: false,
			expectedRisk:     RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := AnalyzeCommandSafety(tt.command, "/tmp")
			require.NotNil(t, analysis)
			assert.Equal(t, tt.expectedApproval, analysis.RequiresApproval)
			assert.Equal(t, tt.expectedRisk, analysis.RiskLevel)
		})
	}
}

// Helper function for case-insensitive string contains check
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TestExtractCommandTokens tests the command token extractor
func TestExtractCommandTokens(t *testing.T) {
	tests := []struct {
		name          string
		shellCmd      string
		targetProgram string
		expected      []string
	}{
		{
			name:          "extract rm with flags",
			shellCmd:      "ls && rm -rf dir",
			targetProgram: "rm",
			expected:      []string{"rm", "-rf", "dir"},
		},
		{
			name:          "extract ls from chain",
			shellCmd:      "ls && rm -rf dir",
			targetProgram: "ls",
			expected:      []string{"ls"},
		},
		{
			name:          "extract chmod with args",
			shellCmd:      "ls && chmod 777 file.txt",
			targetProgram: "chmod",
			expected:      []string{"chmod", "777", "file.txt"},
		},
		{
			name:          "extract from pipe",
			shellCmd:      "ls | grep test",
			targetProgram: "grep",
			expected:      []string{"grep", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCommandTokens(tt.shellCmd, tt.targetProgram)
			assert.Equal(t, tt.expected, result)
		})
	}
}
