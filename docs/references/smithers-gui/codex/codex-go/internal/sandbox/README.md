# Sandbox Package

The `sandbox` package provides execution isolation for Codex Go through multiple sandbox technologies. It enables safe command execution with controlled access to filesystem, network, and system resources.

## Overview

Codex Go supports three sandbox types with automatic escalation:

1. **Native** - Direct execution without isolation (fastest, least secure)
2. **Docker** - Container-based isolation (balanced security and compatibility)
3. **Kubernetes** - Pod-based execution (distributed, cloud-native)

The orchestrator automatically selects the best available sandbox and can escalate through the chain if a sandbox fails.

## Sandbox Types

### 1. Native Sandbox (`internal/sandbox/native/`)

Executes commands directly on the host system using `os/exec`. Provides no isolation but has minimal overhead.

**Features**:
- Zero configuration
- Always available
- Full system access
- No resource limits

**Example**:
```go
import "github.com/evmts/codex/codex-go/internal/sandbox/native"

sandbox := native.New()
result, err := sandbox.Execute(ctx, &sandbox.Command{
    Program: "echo",
    Args: []string{"hello world"},
    WorkingDirectory: "/workspace",
})
```

### 2. Docker Sandbox (`internal/sandbox/docker/`)

Executes commands in isolated Docker containers with configurable resource limits, volume mounts, and network policies.

**Features**:
- Filesystem isolation via volume mounts
- Network isolation (none/bridge/host modes)
- Resource limits (CPU, memory)
- Automatic cleanup via `--rm` flag
- Security options (no-new-privileges)

**Example**:
```go
import "github.com/evmts/codex/codex-go/internal/sandbox/docker"

sandbox := docker.NewDockerSandbox(commander, &docker.Options{
    Image: "ubuntu:22.04",
    Network: "none",
    MemoryLimit: "512m",
    CPULimit: "1.0",
})

result, err := sandbox.Execute(ctx, &sandbox.Command{
    Program: "sh",
    Args: []string{"-c", "echo hello"},
    WorkingDirectory: "/workspace",
    ReadWritePaths: []string{"/workspace"},
    ReadOnlyPaths: []string{"/usr", "/lib"},
    NetworkEnabled: false,
})
```

### 3. Kubernetes Sandbox (`internal/sandbox/kubernetes/`)

Executes commands in isolated Kubernetes pods with full pod lifecycle management, volume mounts, and resource quotas.

**Features**:
- Pod-based isolation
- Resource limits via Kubernetes quotas
- Volume mounts (hostPath)
- Network policies (ClusterFirst/None)
- Service account integration
- Automatic pod cleanup after execution

**Example**:
```go
import "github.com/evmts/codex/codex-go/internal/sandbox/kubernetes"

sandbox := kubernetes.NewKubernetesSandbox(commander, &kubernetes.Options{
    Namespace: "codex-sandbox",
    Image: "ubuntu:22.04",
    Timeout: 5 * time.Minute,
    CPULimit: "1000m",
    MemoryLimit: "512Mi",
    ServiceAccount: "codex-runner",
})

result, err := sandbox.Execute(ctx, &sandbox.Command{
    Program: "sh",
    Args: []string{"-c", "echo hello"},
    WorkingDirectory: "/workspace",
    Environment: map[string]string{"FOO": "bar"},
    ReadWritePaths: []string{"/workspace"},
    NetworkEnabled: true,
})
```

## Orchestrator Integration

The `SandboxSelector` in the orchestrator automatically chooses and manages sandboxes.

### Sandbox Selection Priority

Bubblewrap → Docker → Kubernetes → None

### Escalation Strategy

When a sandbox fails (e.g., permission denied), the orchestrator can escalate:

```
Bubblewrap ──failed──▶ Docker ──failed──▶ Kubernetes ──failed──▶ None
```

**Example Escalation Flow**:
1. Tool requests execution with `SandboxAuto` preference
2. Selector chooses Docker (best available)
3. Docker execution fails with permission error
4. Orchestrator requests approval for retry
5. Selector escalates to Kubernetes
6. Kubernetes execution succeeds

## Testing

All sandbox implementations include comprehensive test suites with mocked commands.

### Running Tests

```bash
# Test all sandboxes
go test ./internal/sandbox/...

# Test specific sandbox
go test ./internal/sandbox/kubernetes/... -v

# Test orchestrator integration
go test ./internal/tools/orchestrator/... -v
```

## Configuration

### Docker Options
```go
docker.Options{
    Image:          "ubuntu:22.04",  // Container image
    Network:        "none",           // Network mode
    MemoryLimit:    "512m",          // Memory limit
    CPULimit:       "1.0",           // CPU limit
    CleanupTimeout: 30 * time.Second // Cleanup timeout
}
```

### Kubernetes Options
```go
kubernetes.Options{
    Namespace:      "default",       // K8s namespace
    Image:          "ubuntu:22.04",  // Container image
    Timeout:        5 * time.Minute, // Execution timeout
    CPULimit:       "1000m",         // CPU limit
    MemoryLimit:    "512Mi",         // Memory limit
    ServiceAccount: "default",       // Service account
    CleanupTimeout: 30 * time.Second // Pod deletion timeout
}
```

## Security Considerations

### Native Sandbox
- **No isolation**: Full system access
- **Use**: Only in trusted environments

### Docker Sandbox
- **Filesystem isolation**: Via volume mounts
- **Network isolation**: Configurable
- **Security options**: `--no-new-privileges`

### Kubernetes Sandbox
- **Pod isolation**: Namespace-based
- **Network policies**: Kubernetes NetworkPolicy
- **RBAC**: Service account permissions
- **Resource quotas**: Namespace-level limits

## Best Practices

1. **Always use context with timeout**
2. **Handle cleanup properly** with defer
3. **Limit resource usage** via options
4. **Use read-only mounts** when possible
5. **Disable network** for untrusted code

## License

Part of Codex Go - see main repository LICENSE file.
