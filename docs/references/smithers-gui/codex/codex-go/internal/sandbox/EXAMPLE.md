# Sandbox Escalation Example

This document demonstrates how the Codex Go sandbox system works, including automatic escalation between sandbox types.

## Scenario: Command Execution with Escalation

Let's walk through a real-world scenario where a tool needs to execute a command, and the orchestrator automatically escalates through different sandbox types.

### Initial Request

A tool requests to execute `npm install` in a project directory:

```go
request := &runtime.ToolRequest{
    CallID:           "npm-install-123",
    ToolName:         "shell",
    Arguments:        `{"command": "npm install"}`,
    WorkingDirectory: "/workspace/my-project",
}
```

### Step 1: Sandbox Selection

The orchestrator's `SandboxSelector` determines the initial sandbox:

```go
selector := NewSandboxSelector()

// Detection results (example):
// - Bubblewrap: Not available (Linux-only)
// - Docker: Available
// - Kubernetes: Available
// - Native: Always available

// Selects Docker (first available in priority order)
attempt := selector.SelectSandbox(tool, request, policy, false)
// Result: SandboxAttempt{Type: SandboxDocker}
```

**Priority Order**: Bubblewrap → Docker → Kubernetes → None

### Step 2: First Execution (Docker)

The orchestrator executes the command in Docker:

```go
// Docker sandbox builds command:
// docker run --rm --network=none -v /workspace:/workspace -w /workspace/my-project ubuntu:22.04 npm install

stdout, stderr, exitCode, err := dockerSandbox.Execute(ctx, cmd)
```

**Result**: Docker execution fails with permission error:
```
Error: EACCES: permission denied, mkdir '/workspace/my-project/node_modules'
```

The tool returns `ErrorSandboxDenied` because the Docker container doesn't have write access to the volume.

### Step 3: Approval Request

The orchestrator detects sandbox denial and requests user approval to retry:

```go
approvalRequest := &runtime.ApprovalRequest{
    CallID:     "npm-install-123",
    ToolName:   "shell",
    Command:    []string{"npm", "install"},
    IsRetry:    true,
    RetryReason: "permission denied",
}

decision, err := approvalHandler(ctx, approvalRequest)
```

**User sees**:
```
The command 'npm install' failed in Docker sandbox due to: permission denied

Retry with escalated sandbox (Kubernetes)?
[Approve] [Deny] [Approve for Session]
```

User selects: **Approve for Session**

### Step 4: Sandbox Escalation

The orchestrator escalates to the next available sandbox:

```go
// Docker failed, try Kubernetes next
escalatedAttempt := selector.EscalateSandbox(currentAttempt)
// Result: SandboxAttempt{Type: SandboxKubernetes}
```

**Escalation Path**:
```
Docker (failed) → Kubernetes → None
         ↓
   Permission denied
```

### Step 5: Retry Execution (Kubernetes)

The orchestrator retries with Kubernetes sandbox:

```go
// Kubernetes sandbox creates pod:
// 1. Generate pod name: codex-sandbox-1698765432-1
// 2. Create pod with proper volume mounts and permissions
// 3. Wait for pod Ready status
// 4. Stream logs
// 5. Get exit code
// 6. Clean up pod

result, err := kubernetesSandbox.Execute(ctx, cmd)
```

**Kubernetes Pod Spec** (simplified):
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: codex-sandbox-1698765432-1
  namespace: default
spec:
  restartPolicy: Never
  containers:
  - name: sandbox
    image: ubuntu:22.04
    command: ["npm", "install"]
    workingDir: /workspace/my-project
    volumeMounts:
    - name: workspace
      mountPath: /workspace
      readOnly: false
    resources:
      limits:
        cpu: "1000m"
        memory: "512Mi"
  volumes:
  - name: workspace
    hostPath:
      path: /workspace
```

**Result**: Success!
```
✓ npm install completed successfully
   Packages installed: 142
   Audit: 0 vulnerabilities
```

### Step 6: Result Return

The orchestrator returns the successful result:

```go
executionResult := &runtime.ExecutionResult{
    Request:          request,
    Response:         response,
    SandboxUsed:      true,
    RetryCount:       1,
    ApprovalRequired: true,
    StartTime:        startTime,
    EndTime:          endTime,
}
```

### Complete Flow Diagram

```
┌─────────────────────────────────────────────────────┐
│ 1. Tool Request: npm install                        │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 2. SandboxSelector: Choose Docker (first available) │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 3. Docker Execution: FAIL (permission denied)       │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 4. Approval Request: Retry with escalated sandbox?  │
│    User: Approve for Session                        │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 5. Escalate: Docker → Kubernetes                    │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 6. Kubernetes Execution: SUCCESS                    │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│ 7. Return Result: Command output + metadata         │
└─────────────────────────────────────────────────────┘
```

## Code Example: Direct Sandbox Usage

### Using Docker Sandbox Directly

```go
package main

import (
    "context"
    "fmt"
    "os/exec"
    "time"

    "github.com/evmts/codex/codex-go/internal/sandbox"
    "github.com/evmts/codex/codex-go/internal/sandbox/docker"
)

// RealCommander implements sandbox.Commander using os/exec
type RealCommander struct{}

func (r *RealCommander) Run(ctx context.Context, program string, args ...string) (string, string, int, error) {
    cmd := exec.CommandContext(ctx, program, args...)
    stdout, err := cmd.Output()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return string(stdout), string(exitErr.Stderr), exitErr.ExitCode(), nil
        }
        return "", "", -1, err
    }
    return string(stdout), "", 0, nil
}

func main() {
    // Create Docker sandbox with custom configuration
    commander := &RealCommander{}
    dockerSandbox := docker.NewDockerSandbox(commander, &docker.Options{
        Image:       "node:18-alpine",
        Network:     "none",
        MemoryLimit: "512m",
        CPULimit:    "1.0",
    })

    // Check availability
    if !dockerSandbox.IsAvailable() {
        fmt.Println("Docker is not available")
        return
    }

    // Execute command
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    result, err := dockerSandbox.Execute(ctx, &sandbox.Command{
        Program:          "npm",
        Args:             []string{"install"},
        WorkingDirectory: "/workspace/my-project",
        ReadWritePaths:   []string{"/workspace"},
        NetworkEnabled:   true, // Allow npm to download packages
        Timeout:          5 * time.Minute,
    })

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Exit Code: %d\n", result.ExitCode)
    fmt.Printf("Execution Time: %v\n", result.ExecutionTime)
    fmt.Printf("Output:\n%s\n", result.Stdout)
}
```

### Using Kubernetes Sandbox Directly

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/evmts/codex/codex-go/internal/sandbox"
    "github.com/evmts/codex/codex-go/internal/sandbox/kubernetes"
)

func main() {
    // Create Kubernetes sandbox with custom configuration
    commander := &RealCommander{} // Same as above
    k8sSandbox := kubernetes.NewKubernetesSandbox(commander, &kubernetes.Options{
        Namespace:      "ci-builds",
        Image:          "golang:1.21",
        Timeout:        30 * time.Minute,
        CPULimit:       "4000m",
        MemoryLimit:    "8Gi",
        ServiceAccount: "ci-runner",
    })

    // Check availability
    if !k8sSandbox.IsAvailable() {
        fmt.Println("Kubernetes is not available")
        return
    }

    // Execute build command
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    result, err := k8sSandbox.Execute(ctx, &sandbox.Command{
        Program:          "go",
        Args:             []string{"build", "-o", "app", "."},
        WorkingDirectory: "/workspace",
        Environment: map[string]string{
            "GOOS":   "linux",
            "GOARCH": "amd64",
            "CGO_ENABLED": "0",
        },
        ReadWritePaths: []string{"/workspace", "/go"},
        NetworkEnabled: true, // Allow downloading dependencies
        Timeout:        30 * time.Minute,
    })

    // Cleanup happens automatically via defer in Execute

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Exit Code: %d\n", result.ExitCode)
    fmt.Printf("Execution Time: %v\n", result.ExecutionTime)
    fmt.Printf("Build Output:\n%s\n", result.Stdout)
}
```

## Testing with Mocks

All sandboxes support testing without actual command execution:

```go
func TestMyFeature(t *testing.T) {
    // Create mock commander
    mock := &MockCommander{
        RunFunc: func(ctx context.Context, name string, args ...string) (string, string, int, error) {
            // Simulate Docker execution
            if name == "docker" && args[0] == "run" {
                return "command output", "", 0, nil
            }
            return "", "", 1, fmt.Errorf("unexpected command")
        },
    }

    // Create sandbox with mock
    sandbox := docker.NewDockerSandbox(mock, nil)

    // Execute command (no actual Docker needed!)
    result, err := sandbox.Execute(ctx, &sandbox.Command{
        Program: "echo",
        Args:    []string{"hello"},
    })

    assert.NoError(t, err)
    assert.Equal(t, "command output", result.Stdout)
}
```

## Configuration Best Practices

### Docker Sandbox

**For Development**:
```go
&docker.Options{
    Image:       "ubuntu:22.04",
    Network:     "bridge",      // Allow network
    MemoryLimit: "1g",
    CPULimit:    "2.0",
}
```

**For CI/CD**:
```go
&docker.Options{
    Image:       "node:18-alpine",
    Network:     "none",        // Block network
    MemoryLimit: "512m",
    CPULimit:    "1.0",
}
```

**For Builds**:
```go
&docker.Options{
    Image:       "golang:1.21",
    Network:     "bridge",      // Download dependencies
    MemoryLimit: "4g",
    CPULimit:    "4.0",
}
```

### Kubernetes Sandbox

**For Short Tasks**:
```go
&kubernetes.Options{
    Namespace:   "default",
    Image:       "ubuntu:22.04",
    Timeout:     5 * time.Minute,
    CPULimit:    "1000m",
    MemoryLimit: "512Mi",
}
```

**For Long-Running Builds**:
```go
&kubernetes.Options{
    Namespace:      "ci-builds",
    Image:          "golang:1.21",
    Timeout:        1 * time.Hour,
    CPULimit:       "8000m",
    MemoryLimit:    "16Gi",
    ServiceAccount: "ci-runner",
}
```

## Summary

The Codex Go sandbox system provides:

1. **Automatic Selection**: Chooses best available sandbox
2. **Graceful Escalation**: Retries with different sandbox on failure
3. **User Approval**: Requests permission for escalation
4. **Comprehensive Testing**: All components fully tested with mocks
5. **Flexible Configuration**: Customizable per sandbox type
6. **Clean Abstractions**: Common interface for all sandbox types

This enables safe, isolated command execution with automatic fallback strategies.
