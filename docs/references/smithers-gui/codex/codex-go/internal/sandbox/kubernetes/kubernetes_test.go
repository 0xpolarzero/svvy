package kubernetes

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

func (m *MockCommander) GetCalls() []commandCall {
	return m.calls
}

func (m *MockCommander) LastCall() *commandCall {
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewKubernetesSandbox(t *testing.T) {
	tests := []struct {
		name string
		opts *Options
		want *Options
	}{
		{
			name: "default options",
			opts: nil,
			want: &Options{
				Namespace:      "default",
				Image:          "ubuntu:22.04",
				Timeout:        5 * time.Minute,
				CPULimit:       "1000m",
				MemoryLimit:    "512Mi",
				CleanupTimeout: 30 * time.Second,
			},
		},
		{
			name: "custom options",
			opts: &Options{
				Namespace:   "custom-ns",
				Image:       "alpine:3.18",
				Timeout:     10 * time.Minute,
				CPULimit:    "2000m",
				MemoryLimit: "1Gi",
			},
			want: &Options{
				Namespace:      "custom-ns",
				Image:          "alpine:3.18",
				Timeout:        10 * time.Minute,
				CPULimit:       "2000m",
				MemoryLimit:    "1Gi",
				CleanupTimeout: 30 * time.Second,
			},
		},
		{
			name: "partial custom options with defaults",
			opts: &Options{
				Namespace: "my-namespace",
				Image:     "custom-image:latest",
			},
			want: &Options{
				Namespace:      "my-namespace",
				Image:          "custom-image:latest",
				Timeout:        5 * time.Minute,
				CPULimit:       "1000m",
				MemoryLimit:    "512Mi",
				CleanupTimeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			sb := NewKubernetesSandbox(mock, tt.opts)

			require.NotNil(t, sb)
			assert.Equal(t, "kubernetes", sb.Type())
			assert.Equal(t, tt.want.Namespace, sb.opts.Namespace)
			assert.Equal(t, tt.want.Image, sb.opts.Image)
			assert.Equal(t, tt.want.Timeout, sb.opts.Timeout)
			assert.Equal(t, tt.want.CPULimit, sb.opts.CPULimit)
			assert.Equal(t, tt.want.MemoryLimit, sb.opts.MemoryLimit)
		})
	}
}

// ============================================================================
// IsAvailable Tests
// ============================================================================

func TestKubernetesSandbox_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*MockCommander)
		want      bool
	}{
		{
			name: "kubectl available",
			setupMock: func(m *MockCommander) {
				m.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
					if name == "kubectl" && args[0] == "version" {
						return "Client Version: v1.28.0\n", "", 0, nil
					}
					return "", "", 1, errors.New("command not found")
				}
			},
			want: true,
		},
		{
			name: "kubectl not available",
			setupMock: func(m *MockCommander) {
				m.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
					return "", "kubectl: command not found", 127, errors.New("command not found")
				}
			},
			want: false,
		},
		{
			name: "kubectl version fails",
			setupMock: func(m *MockCommander) {
				m.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
					return "", "connection refused", 1, errors.New("connection refused")
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setupMock(mock)

			sb := NewKubernetesSandbox(mock, nil)
			got := sb.IsAvailable()

			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// Command Construction Tests
// ============================================================================

func TestKubernetesSandbox_BuildPodName(t *testing.T) {
	sb := NewKubernetesSandbox(NewMockCommander(), nil)

	podName := sb.buildPodName()
	assert.True(t, strings.HasPrefix(podName, "codex-sandbox-"))
	assert.True(t, len(podName) > 15) // Has suffix

	// Should generate different names
	podName2 := sb.buildPodName()
	assert.NotEqual(t, podName, podName2)
}

func TestKubernetesSandbox_BuildPodSpec(t *testing.T) {
	mock := NewMockCommander()
	sb := NewKubernetesSandbox(mock, &Options{
		Namespace:   "test-ns",
		Image:       "alpine:3.18",
		CPULimit:    "500m",
		MemoryLimit: "256Mi",
	})

	cmd := &sandbox.Command{
		Program:          "sh",
		Args:             []string{"-c", "echo hello"},
		WorkingDirectory: "/workspace",
		Environment: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
		ReadOnlyPaths:  []string{"/usr", "/lib"},
		ReadWritePaths: []string{"/workspace"},
		NetworkEnabled: true,
	}

	spec := sb.buildPodSpec("test-pod", cmd)

	// Verify it's valid YAML/JSON-like structure
	assert.Contains(t, spec, "apiVersion: v1")
	assert.Contains(t, spec, "kind: Pod")
	assert.Contains(t, spec, "name: test-pod")
	assert.Contains(t, spec, "namespace: test-ns")
	assert.Contains(t, spec, "image: alpine:3.18")
	assert.Contains(t, spec, "cpu: \"500m\"")
	assert.Contains(t, spec, "memory: \"256Mi\"")
	assert.Contains(t, spec, "restartPolicy: Never")

	// Verify environment variables
	assert.Contains(t, spec, "- name: FOO")
	assert.Contains(t, spec, "  value: \"bar\"")
	assert.Contains(t, spec, "- name: BAZ")
	assert.Contains(t, spec, "  value: \"qux\"")

	// Verify command
	assert.Contains(t, spec, "command:")
	assert.Contains(t, spec, "- sh")
	assert.Contains(t, spec, "- -c")
	assert.Contains(t, spec, "- echo hello")
}

func TestKubernetesSandbox_BuildPodSpec_NoEnvironment(t *testing.T) {
	mock := NewMockCommander()
	sb := NewKubernetesSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "ls",
		Args:    []string{"-la"},
	}

	spec := sb.buildPodSpec("test-pod", cmd)

	// Should not have env section if no environment variables
	assert.NotContains(t, spec, "env:")
}

// ============================================================================
// Execute Tests
// ============================================================================

func TestKubernetesSandbox_Execute_Success(t *testing.T) {
	mock := NewMockCommander()
	callCount := 0

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		callCount++

		switch callCount {
		case 1: // kubectl apply
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, "apply", args[0])
			assert.Contains(t, strings.Join(args, " "), "-f -")
			return "pod/test-pod created\n", "", 0, nil

		case 2: // kubectl wait
			assert.Equal(t, "kubectl", name)
			assert.Contains(t, strings.Join(args, " "), "wait")
			assert.Contains(t, strings.Join(args, " "), "--for=condition=Ready")
			return "pod/test-pod condition met\n", "", 0, nil

		case 3: // kubectl logs
			assert.Equal(t, "kubectl", name)
			assert.Contains(t, strings.Join(args, " "), "logs")
			return "hello world\n", "", 0, nil

		case 4: // kubectl get pod (for exit code)
			assert.Equal(t, "kubectl", name)
			assert.Contains(t, strings.Join(args, " "), "get pod")
			return "Succeeded", "", 0, nil

		case 5: // kubectl delete
			assert.Equal(t, "kubectl", name)
			assert.Contains(t, strings.Join(args, " "), "delete")
			return "pod/test-pod deleted\n", "", 0, nil

		default:
			return "", "", 0, nil
		}
	}

	sb := NewKubernetesSandbox(mock, &Options{
		Namespace: "test-ns",
	})

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
	assert.Greater(t, callCount, 3) // At least apply, wait, logs, get
}

func TestKubernetesSandbox_Execute_CommandFailure(t *testing.T) {
	mock := NewMockCommander()
	callCount := 0

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		callCount++

		switch callCount {
		case 1: // kubectl apply
			return "pod/test-pod created\n", "", 0, nil
		case 2: // kubectl wait
			return "pod/test-pod condition met\n", "", 0, nil
		case 3: // kubectl logs
			return "", "command not found\n", 0, nil
		case 4: // kubectl get pod (failed status)
			return "Failed", "", 0, nil
		case 5: // kubectl delete
			return "pod/test-pod deleted\n", "", 0, nil
		default:
			return "", "", 0, nil
		}
	}

	sb := NewKubernetesSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "nonexistent-command",
		Args:    []string{},
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.NoError(t, err) // Sandbox succeeded, but command failed
	require.NotNil(t, result)
	assert.NotEqual(t, 0, result.ExitCode) // Command failed
	assert.Contains(t, result.Stderr, "command not found")
}

func TestKubernetesSandbox_Execute_PodCreationFailure(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		if strings.Contains(strings.Join(args, " "), "apply") {
			return "", "error: pod quota exceeded", 1, errors.New("quota exceeded")
		}
		return "", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program: "echo",
		Args:    []string{"test"},
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create pod")
}

func TestKubernetesSandbox_Execute_PodWaitTimeout(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		if strings.Contains(strings.Join(args, " "), "apply") {
			return "pod/test-pod created\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "wait") {
			return "", "timed out waiting for condition", 1, errors.New("timeout")
		}
		if strings.Contains(strings.Join(args, " "), "delete") {
			return "pod/test-pod deleted\n", "", 0, nil
		}
		return "", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, &Options{
		Timeout: 1 * time.Second,
	})

	cmd := &sandbox.Command{
		Program: "sleep",
		Args:    []string{"infinity"},
	}

	ctx := context.Background()
	result, err := sb.Execute(ctx, cmd)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "pod failed to become ready")
}

func TestKubernetesSandbox_Execute_ContextCancellation(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		// Simulate slow operation
		if strings.Contains(strings.Join(args, " "), "wait") {
			<-ctx.Done()
			return "", "context canceled", 1, ctx.Err()
		}
		if strings.Contains(strings.Join(args, " "), "apply") {
			return "pod/test-pod created\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "delete") {
			return "pod/test-pod deleted\n", "", 0, nil
		}
		return "", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, nil)

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

func TestKubernetesSandbox_Cleanup_Success(t *testing.T) {
	mock := NewMockCommander()
	deleteCalled := false

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		if strings.Contains(strings.Join(args, " "), "delete") {
			deleteCalled = true
			return "pod/test-pod deleted\n", "", 0, nil
		}
		return "", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, nil)
	sb.currentPodName = "test-pod" // Simulate active pod

	ctx := context.Background()
	err := sb.Cleanup(ctx)

	require.NoError(t, err)
	assert.True(t, deleteCalled)
	assert.Equal(t, "", sb.currentPodName) // Should clear pod name
}

func TestKubernetesSandbox_Cleanup_NoPod(t *testing.T) {
	mock := NewMockCommander()

	sb := NewKubernetesSandbox(mock, nil)
	// No currentPodName set

	ctx := context.Background()
	err := sb.Cleanup(ctx)

	require.NoError(t, err)
	assert.Len(t, mock.GetCalls(), 0) // Should not call kubectl
}

func TestKubernetesSandbox_Cleanup_DeleteFailure(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		return "", "pod not found", 1, errors.New("not found")
	}

	sb := NewKubernetesSandbox(mock, nil)
	sb.currentPodName = "test-pod"

	ctx := context.Background()
	err := sb.Cleanup(ctx)

	// Should not error if pod is already gone
	require.NoError(t, err)
}

// ============================================================================
// Volume Mount Tests
// ============================================================================

func TestKubernetesSandbox_BuildPodSpec_WithVolumes(t *testing.T) {
	mock := NewMockCommander()
	sb := NewKubernetesSandbox(mock, nil)

	cmd := &sandbox.Command{
		Program:          "ls",
		WorkingDirectory: "/workspace",
		ReadOnlyPaths:    []string{"/usr", "/lib"},
		ReadWritePaths:   []string{"/workspace", "/tmp"},
	}

	spec := sb.buildPodSpec("test-pod", cmd)

	// Should contain volume mounts
	assert.Contains(t, spec, "volumeMounts:")
	assert.Contains(t, spec, "mountPath: /workspace")
	assert.Contains(t, spec, "readOnly: false")

	// Read-only paths should be mounted as readOnly
	assert.Contains(t, spec, "mountPath: /usr")
	assert.Contains(t, spec, "readOnly: true")
}

// ============================================================================
// Network Policy Tests
// ============================================================================

func TestKubernetesSandbox_BuildPodSpec_NetworkPolicy(t *testing.T) {
	tests := []struct {
		name           string
		networkEnabled bool
		wantContains   string
	}{
		{
			name:           "network enabled",
			networkEnabled: true,
			wantContains:   "dnsPolicy: ClusterFirst",
		},
		{
			name:           "network disabled",
			networkEnabled: false,
			wantContains:   "dnsPolicy: None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			sb := NewKubernetesSandbox(mock, nil)

			cmd := &sandbox.Command{
				Program:        "echo",
				NetworkEnabled: tt.networkEnabled,
			}

			spec := sb.buildPodSpec("test-pod", cmd)
			assert.Contains(t, spec, tt.wantContains)
		})
	}
}

// ============================================================================
// Concurrent Execution Tests
// ============================================================================

func TestKubernetesSandbox_ConcurrentExecution(t *testing.T) {
	mock := NewMockCommander()
	podCount := 0

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		if strings.Contains(strings.Join(args, " "), "apply") {
			podCount++
			return "pod created\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "wait") {
			return "ready\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "logs") {
			return "output\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "get pod") {
			return "Succeeded", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "delete") {
			return "deleted\n", "", 0, nil
		}
		return "", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, nil)

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

	// Should have created 3 different pods
	assert.Equal(t, 3, podCount)
}

// ============================================================================
// Timeout Tests
// ============================================================================

func TestKubernetesSandbox_Execute_WithCommandTimeout(t *testing.T) {
	mock := NewMockCommander()

	mock.RunFunc = func(ctx context.Context, name string, args ...string) (string, string, int, error) {
		if strings.Contains(strings.Join(args, " "), "apply") {
			return "pod/test-pod created\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "wait") {
			// Check that timeout is propagated to kubectl
			assert.Contains(t, strings.Join(args, " "), "--timeout=")
			return "ready\n", "", 0, nil
		}
		if strings.Contains(strings.Join(args, " "), "delete") {
			return "deleted\n", "", 0, nil
		}
		return "output\n", "", 0, nil
	}

	sb := NewKubernetesSandbox(mock, &Options{
		Timeout: 2 * time.Minute,
	})

	cmd := &sandbox.Command{
		Program: "echo",
		Args:    []string{"test"},
		Timeout: 30 * time.Second, // Command-specific timeout
	}

	ctx := context.Background()
	_, err := sb.Execute(ctx, cmd)

	require.NoError(t, err)
}
