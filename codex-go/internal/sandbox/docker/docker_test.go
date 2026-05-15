package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/sandbox"
)

// ============================================================================
// Mock Commander for Testing
// ============================================================================

type MockCommander struct {
	RunFunc func(ctx context.Context, name string, args ...string) (string, string, int, error)
	calls   []commandCall
}

type commandCall struct {
	name string
	args []string
}

func NewMockCommander() *MockCommander {
	return &MockCommander{
		calls: make([]commandCall, 0),
		RunFunc: func(ctx context.Context, name string, args ...string) (string, string, int, error) {
			return "", "", 0, nil
		},
	}
}

func (m *MockCommander) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	m.calls = append(m.calls, commandCall{name: name, args: args})
	return m.RunFunc(ctx, name, args...)
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewDockerSandbox(t *testing.T) {
	tests := []struct {
		name string
		opts *Options
		want *Options
	}{
		{
			name: "default options",
			opts: nil,
			want: &Options{
				Image:          "ubuntu:22.04",
				Network:        "none",
				CleanupTimeout: 30 * time.Second,
			},
		},
		{
			name: "custom options",
			opts: &Options{
				Image:   "alpine:3.18",
				Network: "bridge",
			},
			want: &Options{
				Image:          "alpine:3.18",
				Network:        "bridge",
				CleanupTimeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			sb := NewDockerSandbox(mock, tt.opts)

			require.NotNil(t, sb)
			assert.Equal(t, "docker", sb.Type())
			assert.Equal(t, tt.want.Image, sb.opts.Image)
			assert.Equal(t, tt.want.Network, sb.opts.Network)
		})
	}
}

// ============================================================================
// IsAvailable Tests
// ============================================================================

func TestDockerSandbox_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*MockCommander)
		want      bool
	}{
		{
			name: "docker available",
			setupMock: func(m *MockCommander) {
				m.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
					if name == "docker" && args[0] == "version" {
						return "Docker version 24.0.0\n", "", 0, nil
					}
					return "", "", 1, errors.New("command not found")
				}
			},
			want: true,
		},
		{
			name: "docker not available",
			setupMock: func(m *MockCommander) {
				m.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
					return "", "docker: command not found", 127, errors.New("command not found")
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setupMock(mock)

			sb := NewDockerSandbox(mock, nil)
			got := sb.IsAvailable()

			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// Execute Tests
// ============================================================================

func TestDockerSandbox_Execute_Success(t *testing.T) {
	mock := NewMockCommander()
	callCount := 0

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		callCount++

		switch callCount {
		case 1: // docker run
			assert.Equal(t, "docker", name)
			assert.Equal(t, "run", args[0])
			assert.Contains(t, strings.Join(args, " "), "--rm")
			assert.Contains(t, strings.Join(args, " "), "ubuntu:22.04")
			return "hello world\n", "", 0, nil

		default:
			return "", "", 0, nil
		}
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program:          "echo",
		Args:             []string{"hello world"},
		WorkingDirectory: "/workspace",
		Timeout:          30 * time.Second,
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hello world\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestDockerSandbox_Execute_WithEnvironment(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		argsStr := strings.Join(args, " ")
		// Check environment variables are passed
		assert.Contains(t, argsStr, "-e FOO=bar")
		assert.Contains(t, argsStr, "-e BAZ=qux")
		return "output\n", "", 0, nil
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "env",
		Environment: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}

	ctx := context.Background()
	_, err := sb.Execute(ctx, cmd)

	require.NoError(t, err)
}

func TestDockerSandbox_Execute_WithVolumes(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		argsStr := strings.Join(args, " ")
		// Check read-write volume mount
		assert.Contains(t, argsStr, "-v /workspace:/workspace")
		// Check read-only volume mount
		assert.Contains(t, argsStr, "-v /usr:/usr:ro")
		return "output\n", "", 0, nil
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program:          "ls",
		WorkingDirectory: "/workspace",
		ReadOnlyPaths:    []string{"/usr"},
		ReadWritePaths:   []string{"/workspace"},
	}

	ctx := context.Background()
	_, err := sb.Execute(ctx, cmd)

	require.NoError(t, err)
}

func TestDockerSandbox_Execute_NetworkPolicy(t *testing.T) {
	tests := []struct {
		name           string
		networkEnabled bool
		wantNetwork    string
	}{
		{
			name:           "network disabled",
			networkEnabled: false,
			wantNetwork:    "--network=none",
		},
		{
			name:           "network enabled",
			networkEnabled: true,
			wantNetwork:    "--network=bridge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()

			mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
				argsStr := strings.Join(args, " ")
				assert.Contains(t, argsStr, tt.wantNetwork)
				return "output\n", "", 0, nil
			}

			sb := NewDockerSandbox(mock, nil)

			cmd := &sandbox.Command{
				Program:        "echo",
				Args:           []string{"test"},
				NetworkEnabled: tt.networkEnabled,
			}

			ctx := context.Background()
			_, err := sb.Execute(ctx, cmd)

			require.NoError(t, err)
		})
	}
}

func TestDockerSandbox_Execute_CommandFailure(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		return "", "command not found\n", 127, nil
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "nonexistent-command",
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.NoError(t, err) // Sandbox succeeded, command failed
	require.NotNil(t, result)
	assert.Equal(t, 127, result.ExitCode)
	assert.Contains(t, result.Stderr, "command not found")
}

func TestDockerSandbox_Execute_DockerFailure(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		return "", "docker: error pulling image", 125, errors.New("image pull failed")
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "echo",
		Args:    []string{"test"},
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "docker run failed")
}

func TestDockerSandbox_Execute_ContextCancellation(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		<-ctx.Done()
		return "", "context canceled", 1, ctx.Err()
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "sleep",
		Args:    []string{"100"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := sb.Execute(ctx, cmd)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
}

// ============================================================================
// Cleanup Tests
// ============================================================================

func TestDockerSandbox_Cleanup(t *testing.T) {
	mock := NewMockCommander()

	sb := NewDockerSandbox(mock, nil)

	ctx := context.Background()
	err := sb.Cleanup(ctx)

	// Docker sandbox doesn't maintain state, so cleanup is a no-op
	require.NoError(t, err)
}

// ============================================================================
// Concurrent Execution Tests
// ============================================================================

func TestDockerSandbox_ConcurrentExecution(t *testing.T) {
	mock := NewMockCommander()
	runCount := 0

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		runCount++
		return "output\n", "", 0, nil
	}

	sb := NewDockerSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "echo",
		Args:    []string{"test"},
	}

	ctx := context.Background()

	// Execute multiple commands concurrently
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_, err := sb.Execute(ctx, cmd)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	assert.Equal(t, 3, runCount)
}

// ============================================================================
// Resource Limits Tests
// ============================================================================

func TestDockerSandbox_Execute_WithResourceLimits(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		argsStr := strings.Join(args, " ")
		assert.Contains(t, argsStr, "--memory=512m")
		assert.Contains(t, argsStr, "--cpus=1.0")
		return "output\n", "", 0, nil
	}

	sb := NewDockerSandbox(mock, &Options{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
	})

	cmd := &sandbox.Command{
		Program: "echo",
		Args:    []string{"test"},
	}

	ctx := context.Background()
	_, err := sb.Execute(ctx, cmd)

	require.NoError(t, err)
}
