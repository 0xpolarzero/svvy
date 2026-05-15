package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseShellCommand tests the shell command parser
func TestParseShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		expected []string
	}{
		{
			name:     "simple direct command",
			command:  []string{"ls"},
			expected: []string{"ls"},
		},
		{
			name:     "direct command with args",
			command:  []string{"ls", "-la"},
			expected: []string{"ls"},
		},
		{
			name:     "shell wrapped single command",
			command:  []string{"sh", "-c", "ls"},
			expected: []string{"ls"},
		},
		{
			name:     "shell wrapped with args",
			command:  []string{"sh", "-c", "ls -la"},
			expected: []string{"ls"},
		},
		{
			name:     "shell wrapped with && operator",
			command:  []string{"sh", "-c", "ls && echo hi"},
			expected: []string{"ls", "echo"},
		},
		{
			name:     "shell wrapped with || operator",
			command:  []string{"sh", "-c", "ls || echo failed"},
			expected: []string{"ls", "echo"},
		},
		{
			name:     "shell wrapped with semicolon",
			command:  []string{"sh", "-c", "ls ; echo done"},
			expected: []string{"ls", "echo"},
		},
		{
			name:     "shell wrapped with pipe",
			command:  []string{"sh", "-c", "ls | grep test"},
			expected: []string{"ls", "grep"},
		},
		{
			name:     "complex multi-command",
			command:  []string{"sh", "-c", "ls -la && cat file.txt | grep error || echo failed"},
			expected: []string{"ls", "cat", "grep", "echo"},
		},
		{
			name:     "bash instead of sh",
			command:  []string{"bash", "-c", "ls && echo hi"},
			expected: []string{"ls", "echo"},
		},
		{
			name:     "command with quoted string",
			command:  []string{"sh", "-c", "echo 'hello world'"},
			expected: []string{"echo"},
		},
		{
			name:     "command with double quoted string",
			command:  []string{"sh", "-c", `echo "hello world"`},
			expected: []string{"echo"},
		},
		{
			name:     "command with backticks",
			command:  []string{"sh", "-c", "echo `date`"},
			expected: []string{"echo"},
		},
		{
			name:     "command with redirects",
			command:  []string{"sh", "-c", "ls > output.txt"},
			expected: []string{"ls"},
		},
		{
			name:     "command with stderr redirect",
			command:  []string{"sh", "-c", "ls 2> error.txt"},
			expected: []string{"ls"},
		},
		{
			name:     "command with append redirect",
			command:  []string{"sh", "-c", "echo hi >> output.txt"},
			expected: []string{"echo"},
		},
		{
			name:     "empty command array",
			command:  []string{},
			expected: nil,
		},
		{
			name:     "shell with empty command",
			command:  []string{"sh", "-c", ""},
			expected: nil,
		},
		{
			name:     "command with escaped characters",
			command:  []string{"sh", "-c", `echo \"test\"`},
			expected: []string{"echo"},
		},
		{
			name:     "command with single quote containing spaces",
			command:  []string{"sh", "-c", "echo 'hello && world'"},
			expected: []string{"echo"},
		},
		{
			name:     "dangerous command in chain",
			command:  []string{"sh", "-c", "ls && rm -rf /tmp/test"},
			expected: []string{"ls", "rm"},
		},
		{
			name:     "command with background operator",
			command:  []string{"sh", "-c", "sleep 10 & echo done"},
			expected: []string{"sleep", "echo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseShellCommand(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractCommandsFromShellString tests the shell string parser
func TestExtractCommandsFromShellString(t *testing.T) {
	tests := []struct {
		name     string
		shellCmd string
		expected []string
	}{
		{
			name:     "single command",
			shellCmd: "ls",
			expected: []string{"ls"},
		},
		{
			name:     "command with arguments",
			shellCmd: "ls -la /tmp",
			expected: []string{"ls"},
		},
		{
			name:     "two commands with &&",
			shellCmd: "ls && echo hi",
			expected: []string{"ls", "echo"},
		},
		{
			name:     "two commands with ||",
			shellCmd: "ls || echo failed",
			expected: []string{"ls", "echo"},
		},
		{
			name:     "two commands with semicolon",
			shellCmd: "ls ; echo done",
			expected: []string{"ls", "echo"},
		},
		{
			name:     "pipe command",
			shellCmd: "ls | grep test",
			expected: []string{"ls", "grep"},
		},
		{
			name:     "complex chain",
			shellCmd: "cd /tmp && ls -la | grep test || echo not found ; pwd",
			expected: []string{"cd", "ls", "grep", "echo", "pwd"},
		},
		{
			name:     "quoted string with operator",
			shellCmd: `echo "hello && world"`,
			expected: []string{"echo"},
		},
		{
			name:     "single quoted string with operator",
			shellCmd: `echo 'hello || world'`,
			expected: []string{"echo"},
		},
		{
			name:     "multiple arguments",
			shellCmd: "grep -r pattern /path/to/dir",
			expected: []string{"grep"},
		},
		{
			name:     "redirects",
			shellCmd: "ls > output.txt 2>&1",
			expected: []string{"ls"},
		},
		{
			name:     "empty string",
			shellCmd: "",
			expected: nil,
		},
		{
			name:     "only whitespace",
			shellCmd: "   \t  \n  ",
			expected: nil,
		},
		{
			name:     "escaped quote",
			shellCmd: `echo \"test\"`,
			expected: []string{"echo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCommandsFromShellString(tt.shellCmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsShellOperator tests the operator detection
func TestIsShellOperator(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"&&", true},
		{"||", true},
		{";", true},
		{"|", true},
		{"&", true},
		{">", true},
		{">>", true},
		{"<", true},
		{"2>", true},
		{"2>>", true},
		{"2>&1", true},
		{"ls", false},
		{"echo", false},
		{"", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := isShellOperator(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsKnownSafeCommand tests safe command detection with shell wrapping
func TestIsKnownSafeCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		expected bool
	}{
		{
			name:     "direct safe command",
			command:  []string{"ls"},
			expected: true,
		},
		{
			name:     "direct safe command with args",
			command:  []string{"ls", "-la"},
			expected: true,
		},
		{
			name:     "shell wrapped safe command",
			command:  []string{"sh", "-c", "ls"},
			expected: true,
		},
		{
			name:     "shell wrapped safe command with args",
			command:  []string{"sh", "-c", "ls -la"},
			expected: true,
		},
		{
			name:     "multiple safe commands with &&",
			command:  []string{"sh", "-c", "ls && echo hi"},
			expected: true,
		},
		{
			name:     "multiple safe commands with pipe",
			command:  []string{"sh", "-c", "ls | grep test"},
			expected: true,
		},
		{
			name:     "complex safe command chain",
			command:  []string{"sh", "-c", "ls && cat file.txt | grep pattern || echo not found"},
			expected: true,
		},
		{
			name:     "direct unsafe command",
			command:  []string{"rm"},
			expected: false,
		},
		{
			name:     "shell wrapped unsafe command",
			command:  []string{"sh", "-c", "rm -rf /"},
			expected: false,
		},
		{
			name:     "safe command followed by unsafe",
			command:  []string{"sh", "-c", "ls && rm file.txt"},
			expected: false,
		},
		{
			name:     "unsafe command followed by safe",
			command:  []string{"sh", "-c", "rm file.txt && ls"},
			expected: false,
		},
		{
			name:     "safe commands with one unsafe in middle",
			command:  []string{"sh", "-c", "ls && rm file.txt && echo done"},
			expected: false,
		},
		{
			name:     "unknown command",
			command:  []string{"unknown-command"},
			expected: false,
		},
		{
			name:     "shell wrapped unknown command",
			command:  []string{"sh", "-c", "unknown-command"},
			expected: false,
		},
		{
			name:     "empty command array",
			command:  []string{},
			expected: true, // Empty command is safe (no-op)
		},
		{
			name:     "shell with empty command string",
			command:  []string{"sh", "-c", ""},
			expected: true, // Empty command is safe (no-op)
		},
		{
			name:     "bash wrapped safe command",
			command:  []string{"bash", "-c", "ls && pwd"},
			expected: true,
		},
		{
			name:     "safe command with quoted args",
			command:  []string{"sh", "-c", `echo "hello world"`},
			expected: true,
		},
		{
			name:     "cat command",
			command:  []string{"sh", "-c", "cat file.txt"},
			expected: true,
		},
		{
			name:     "grep command",
			command:  []string{"sh", "-c", "grep pattern file.txt"},
			expected: true,
		},
		{
			name:     "multiple read-only commands",
			command:  []string{"sh", "-c", "pwd && whoami && date"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKnownSafeCommand(tt.command)
			assert.Equal(t, tt.expected, result, "Command: %v", tt.command)
		})
	}
}

// TestIsDangerousCommand tests dangerous command detection with shell wrapping
func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		expected bool
	}{
		{
			name:     "direct dangerous command rm",
			command:  []string{"rm"},
			expected: true,
		},
		{
			name:     "direct dangerous command sudo",
			command:  []string{"sudo"},
			expected: true,
		},
		{
			name:     "shell wrapped dangerous command",
			command:  []string{"sh", "-c", "rm -rf /tmp/test"},
			expected: true,
		},
		{
			name:     "shell wrapped sudo",
			command:  []string{"sh", "-c", "sudo apt install package"},
			expected: true,
		},
		{
			name:     "safe command followed by dangerous",
			command:  []string{"sh", "-c", "ls && rm file.txt"},
			expected: true,
		},
		{
			name:     "dangerous command followed by safe",
			command:  []string{"sh", "-c", "rm file.txt && ls"},
			expected: true,
		},
		{
			name:     "dangerous in middle of chain",
			command:  []string{"sh", "-c", "ls && rm file.txt && echo done"},
			expected: true,
		},
		{
			name:     "chmod command",
			command:  []string{"sh", "-c", "chmod 777 file.txt"},
			expected: true,
		},
		{
			name:     "chown command",
			command:  []string{"sh", "-c", "chown user:group file.txt"},
			expected: true,
		},
		{
			name:     "dd command",
			command:  []string{"sh", "-c", "dd if=/dev/zero of=file"},
			expected: true,
		},
		{
			name:     "kill command",
			command:  []string{"sh", "-c", "kill -9 1234"},
			expected: true,
		},
		{
			name:     "systemctl command",
			command:  []string{"sh", "-c", "systemctl restart service"},
			expected: true,
		},
		{
			name:     "direct safe command",
			command:  []string{"ls"},
			expected: false,
		},
		{
			name:     "shell wrapped safe command",
			command:  []string{"sh", "-c", "ls"},
			expected: false,
		},
		{
			name:     "multiple safe commands",
			command:  []string{"sh", "-c", "ls && echo hi && pwd"},
			expected: false,
		},
		{
			name:     "safe commands with pipe",
			command:  []string{"sh", "-c", "ls | grep test"},
			expected: false,
		},
		{
			name:     "empty command array",
			command:  []string{},
			expected: false, // IsDangerousCommand: empty is not dangerous
		},
		{
			name:     "shell with empty command string",
			command:  []string{"sh", "-c", ""},
			expected: false, // IsDangerousCommand: empty is not dangerous
		},
		{
			name:     "bash wrapped dangerous command",
			command:  []string{"bash", "-c", "rm -rf /"},
			expected: true,
		},
		{
			name:     "reboot command",
			command:  []string{"sh", "-c", "reboot"},
			expected: true,
		},
		{
			name:     "shutdown command",
			command:  []string{"sh", "-c", "shutdown now"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDangerousCommand(tt.command)
			assert.Equal(t, tt.expected, result, "Command: %v", tt.command)
		})
	}
}

// TestShellCommandParsingEdgeCases tests edge cases in shell command parsing
func TestShellCommandParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		testFunc func([]string) bool
		expected bool
	}{
		{
			name:     "quoted command with && inside",
			command:  []string{"sh", "-c", `echo "ls && rm"`},
			testFunc: IsKnownSafeCommand,
			expected: true, // echo is safe, the && is inside quotes
		},
		{
			name:     "single quoted command with || inside",
			command:  []string{"sh", "-c", `echo 'ls || rm'`},
			testFunc: IsKnownSafeCommand,
			expected: true, // echo is safe, the || is inside quotes
		},
		{
			name:     "command with tabs and newlines",
			command:  []string{"sh", "-c", "ls\t&&\necho\thi"},
			testFunc: IsKnownSafeCommand,
			expected: true,
		},
		{
			name:     "command with multiple spaces",
			command:  []string{"sh", "-c", "ls    &&    echo    hi"},
			testFunc: IsKnownSafeCommand,
			expected: true,
		},
		{
			name:     "command with backticks",
			command:  []string{"sh", "-c", "echo `pwd`"},
			testFunc: IsKnownSafeCommand,
			expected: true,
		},
		{
			name:     "dangerous command with backticks",
			command:  []string{"sh", "-c", "echo `rm file.txt`"},
			testFunc: IsDangerousCommand,
			expected: false, // rm is inside backticks, but we only extract echo
		},
		{
			name:     "complex redirect chain",
			command:  []string{"sh", "-c", "ls 2>&1 | grep error > output.txt"},
			testFunc: IsKnownSafeCommand,
			expected: true,
		},
		{
			name:     "background process",
			command:  []string{"sh", "-c", "sleep 10 & echo done"},
			testFunc: IsKnownSafeCommand,
			expected: false, // sleep is not in safe list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc(tt.command)
			assert.Equal(t, tt.expected, result, "Command: %v", tt.command)
		})
	}
}

// TestMemoryApprovalCache tests the in-memory approval cache
func TestMemoryApprovalCache(t *testing.T) {
	cache := NewMemoryApprovalCache()

	t.Run("get non-existent key", func(t *testing.T) {
		result := cache.Get("non-existent")
		assert.Nil(t, result)
	})

	t.Run("put and get session approval", func(t *testing.T) {
		cache.Put("key1", ApprovalApprovedForSession)
		result := cache.Get("key1")
		require.NotNil(t, result)
		assert.Equal(t, ApprovalApprovedForSession, *result)
	})

	t.Run("put approved once not cached", func(t *testing.T) {
		cache.Put("key2", ApprovalApproved)
		result := cache.Get("key2")
		assert.Nil(t, result, "ApprovalApproved should not be cached")
	})

	t.Run("put denied not cached", func(t *testing.T) {
		cache.Put("key3", ApprovalDenied)
		result := cache.Get("key3")
		assert.Nil(t, result, "ApprovalDenied should not be cached")
	})

	t.Run("clear cache", func(t *testing.T) {
		cache.Put("key4", ApprovalApprovedForSession)
		cache.Clear()
		result := cache.Get("key4")
		assert.Nil(t, result)
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		cache.Put("key5", ApprovalApprovedForSession)
		cache.Put("key5", ApprovalApprovedForSession)
		result := cache.Get("key5")
		require.NotNil(t, result)
		assert.Equal(t, ApprovalApprovedForSession, *result)
	})
}
