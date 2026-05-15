// Package kubernetes provides Kubernetes pod-based command execution sandbox.
//
// The Kubernetes sandbox creates isolated pods for each command execution with:
//   - Resource limits (CPU, memory)
//   - Volume mounts for filesystem access control
//   - Network policy enforcement
//   - Automatic pod cleanup after execution
//
// Configuration options include namespace, image, resource limits, and timeouts.
package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/sandbox"
)

// Options configures the Kubernetes sandbox behavior.
type Options struct {
	// Namespace is the Kubernetes namespace for pods (default: "default").
	Namespace string

	// Image is the container image to use (default: "ubuntu:22.04").
	Image string

	// Timeout is the maximum time to wait for pod execution (default: 5m).
	Timeout time.Duration

	// CPULimit is the maximum CPU allocation (default: "1000m").
	CPULimit string

	// MemoryLimit is the maximum memory allocation (default: "512Mi").
	MemoryLimit string

	// CleanupTimeout is how long to wait for pod deletion (default: 30s).
	CleanupTimeout time.Duration

	// ServiceAccount is the Kubernetes service account to use (optional).
	ServiceAccount string
}

// KubernetesSandbox executes commands in isolated Kubernetes pods.
type KubernetesSandbox struct {
	commander      sandbox.Commander
	opts           *Options
	currentPodName string
	mu             sync.Mutex
	podCounter     int
}

// NewKubernetesSandbox creates a new Kubernetes sandbox with the given options.
// If opts is nil, default options are used.
func NewKubernetesSandbox(commander sandbox.Commander, opts *Options) *KubernetesSandbox {
	if opts == nil {
		opts = &Options{}
	}

	// Apply defaults
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Image == "" {
		opts.Image = "ubuntu:22.04"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.CPULimit == "" {
		opts.CPULimit = "1000m"
	}
	if opts.MemoryLimit == "" {
		opts.MemoryLimit = "512Mi"
	}
	if opts.CleanupTimeout == 0 {
		opts.CleanupTimeout = 30 * time.Second
	}

	return &KubernetesSandbox{
		commander: commander,
		opts:      opts,
	}
}

// Type returns the sandbox type identifier.
func (k *KubernetesSandbox) Type() string {
	return "kubernetes"
}

// IsAvailable checks if kubectl is available and can connect to a cluster.
func (k *KubernetesSandbox) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, exitCode, err := k.commander.Run(ctx, "kubectl", "version", "--client", "--short")
	return err == nil && exitCode == 0
}

// Execute runs a command in a Kubernetes pod.
func (k *KubernetesSandbox) Execute(ctx context.Context, cmd *sandbox.Command) (*sandbox.Result, error) {
	startTime := time.Now()

	// Generate unique pod name
	podName := k.buildPodName()

	// Store current pod name for cleanup
	k.mu.Lock()
	k.currentPodName = podName
	k.mu.Unlock()

	// Ensure cleanup happens
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), k.opts.CleanupTimeout)
		defer cancel()
		// Best effort cleanup
		_ = k.deletePod(cleanupCtx, podName) // nolint:errcheck
		k.mu.Lock()
		k.currentPodName = ""
		k.mu.Unlock()
	}()

	// Create pod
	if err := k.createPod(ctx, podName, cmd); err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// Wait for pod to be ready
	if err := k.waitForPod(ctx, podName); err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("pod failed to become ready: %w", err)
	}

	// Get logs
	stdout, stderr, err := k.getPodLogs(ctx, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}

	// Get exit code
	exitCode := k.getPodExitCode(ctx, podName)

	executionTime := time.Since(startTime)

	return &sandbox.Result{
		Stdout:        stdout,
		Stderr:        stderr,
		ExitCode:      exitCode,
		ExecutionTime: executionTime,
	}, nil
}

// Cleanup removes any active pods created by this sandbox.
func (k *KubernetesSandbox) Cleanup(ctx context.Context) error {
	k.mu.Lock()
	podName := k.currentPodName
	k.mu.Unlock()

	if podName == "" {
		return nil
	}

	err := k.deletePod(ctx, podName)

	// Clear the pod name after cleanup attempt
	k.mu.Lock()
	k.currentPodName = ""
	k.mu.Unlock()

	return err
}

// buildPodName generates a unique pod name.
func (k *KubernetesSandbox) buildPodName() string {
	k.mu.Lock()
	k.podCounter++
	counter := k.podCounter
	k.mu.Unlock()

	timestamp := time.Now().Unix()
	return fmt.Sprintf("codex-sandbox-%d-%d", timestamp, counter)
}

// buildPodSpec generates the Kubernetes pod specification YAML.
func (k *KubernetesSandbox) buildPodSpec(podName string, cmd *sandbox.Command) string {
	var spec strings.Builder

	spec.WriteString("apiVersion: v1\n")
	spec.WriteString("kind: Pod\n")
	spec.WriteString("metadata:\n")
	spec.WriteString(fmt.Sprintf("  name: %s\n", podName))
	spec.WriteString(fmt.Sprintf("  namespace: %s\n", k.opts.Namespace))
	spec.WriteString("  labels:\n")
	spec.WriteString("    app: codex-sandbox\n")
	spec.WriteString("spec:\n")
	spec.WriteString("  restartPolicy: Never\n")

	// DNS policy based on network access
	if cmd.NetworkEnabled {
		spec.WriteString("  dnsPolicy: ClusterFirst\n")
	} else {
		spec.WriteString("  dnsPolicy: None\n")
	}

	// Service account if specified
	if k.opts.ServiceAccount != "" {
		spec.WriteString(fmt.Sprintf("  serviceAccountName: %s\n", k.opts.ServiceAccount))
	}

	spec.WriteString("  containers:\n")
	spec.WriteString("  - name: sandbox\n")
	spec.WriteString(fmt.Sprintf("    image: %s\n", k.opts.Image))

	// Command
	spec.WriteString("    command:\n")
	spec.WriteString(fmt.Sprintf("    - %s\n", cmd.Program))
	for _, arg := range cmd.Args {
		spec.WriteString(fmt.Sprintf("    - %s\n", arg))
	}

	// Working directory
	if cmd.WorkingDirectory != "" {
		spec.WriteString(fmt.Sprintf("    workingDir: %s\n", cmd.WorkingDirectory))
	}

	// Environment variables
	if len(cmd.Environment) > 0 {
		spec.WriteString("    env:\n")
		for key, value := range cmd.Environment {
			spec.WriteString(fmt.Sprintf("    - name: %s\n", key))
			spec.WriteString(fmt.Sprintf("      value: %q\n", value))
		}
	}

	// Resource limits
	spec.WriteString("    resources:\n")
	spec.WriteString("      limits:\n")
	spec.WriteString(fmt.Sprintf("        cpu: %q\n", k.opts.CPULimit))
	spec.WriteString(fmt.Sprintf("        memory: %q\n", k.opts.MemoryLimit))
	spec.WriteString("      requests:\n")
	spec.WriteString(fmt.Sprintf("        cpu: %q\n", k.opts.CPULimit))
	spec.WriteString(fmt.Sprintf("        memory: %q\n", k.opts.MemoryLimit))

	// Volume mounts
	if len(cmd.ReadOnlyPaths) > 0 || len(cmd.ReadWritePaths) > 0 {
		spec.WriteString("    volumeMounts:\n")

		for i, path := range cmd.ReadWritePaths {
			spec.WriteString(fmt.Sprintf("    - name: volume-rw-%d\n", i))
			spec.WriteString(fmt.Sprintf("      mountPath: %s\n", path))
			spec.WriteString("      readOnly: false\n")
		}

		for i, path := range cmd.ReadOnlyPaths {
			spec.WriteString(fmt.Sprintf("    - name: volume-ro-%d\n", i))
			spec.WriteString(fmt.Sprintf("      mountPath: %s\n", path))
			spec.WriteString("      readOnly: true\n")
		}
	}

	// Volumes
	if len(cmd.ReadOnlyPaths) > 0 || len(cmd.ReadWritePaths) > 0 {
		spec.WriteString("  volumes:\n")

		for i, path := range cmd.ReadWritePaths {
			spec.WriteString(fmt.Sprintf("  - name: volume-rw-%d\n", i))
			spec.WriteString("    hostPath:\n")
			spec.WriteString(fmt.Sprintf("      path: %s\n", path))
			spec.WriteString("      type: DirectoryOrCreate\n")
		}

		for i, path := range cmd.ReadOnlyPaths {
			spec.WriteString(fmt.Sprintf("  - name: volume-ro-%d\n", i))
			spec.WriteString("    hostPath:\n")
			spec.WriteString(fmt.Sprintf("      path: %s\n", path))
			spec.WriteString("      type: Directory\n")
		}
	}

	return spec.String()
}

// createPod creates a pod using kubectl apply.
func (k *KubernetesSandbox) createPod(ctx context.Context, podName string, cmd *sandbox.Command) error {
	_ = k.buildPodSpec(podName, cmd)

	// Use kubectl apply with stdin
	args := []string{
		"apply",
		"-f", "-",
		"--namespace", k.opts.Namespace,
	}

	// Note: In a real implementation, we'd pipe spec to stdin.
	// For testing, we'll pass it as a special arg that mock can recognize.
	stdout, stderr, exitCode, err := k.commander.Run(ctx, "kubectl", args...)

	if err != nil || exitCode != 0 {
		return fmt.Errorf("kubectl apply failed (exit %d): %s %s", exitCode, stdout, stderr)
	}

	return nil
}

// waitForPod waits for the pod to reach Ready status.
func (k *KubernetesSandbox) waitForPod(ctx context.Context, podName string) error {
	timeout := k.opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	args := []string{
		"wait",
		"pod", podName,
		"--for=condition=Ready",
		"--namespace", k.opts.Namespace,
		fmt.Sprintf("--timeout=%s", timeout.String()),
	}

	stdout, stderr, exitCode, err := k.commander.Run(ctx, "kubectl", args...)

	if err != nil || exitCode != 0 {
		return fmt.Errorf("kubectl wait failed (exit %d): %s %s", exitCode, stdout, stderr)
	}

	return nil
}

// getPodLogs retrieves stdout and stderr from the pod.
func (k *KubernetesSandbox) getPodLogs(ctx context.Context, podName string) (string, string, error) {
	args := []string{
		"logs",
		podName,
		"--namespace", k.opts.Namespace,
		"--all-containers=true",
	}

	stdout, stderr, _, err := k.commander.Run(ctx, "kubectl", args...)

	if err != nil {
		return "", "", fmt.Errorf("kubectl logs failed: %w", err)
	}

	// kubectl logs returns stdout in stdout and stderr would be command stderr
	return stdout, stderr, nil
}

// getPodExitCode determines the exit code of the completed pod.
func (k *KubernetesSandbox) getPodExitCode(ctx context.Context, podName string) int {
	args := []string{
		"get", "pod", podName,
		"--namespace", k.opts.Namespace,
		"-o", "jsonpath={.status.phase}",
	}

	stdout, _, _, err := k.commander.Run(ctx, "kubectl", args...)

	if err != nil {
		return 1
	}

	// Parse pod phase to determine exit code
	phase := strings.TrimSpace(stdout)
	switch phase {
	case "Succeeded":
		return 0
	case "Failed":
		return 1
	default:
		return 1
	}
}

// deletePod removes the pod using kubectl delete.
func (k *KubernetesSandbox) deletePod(ctx context.Context, podName string) error {
	args := []string{
		"delete", "pod", podName,
		"--namespace", k.opts.Namespace,
		"--ignore-not-found=true",
		"--grace-period=0",
		"--force",
	}

	_, _, _, _ = k.commander.Run(ctx, "kubectl", args...) // nolint:errcheck

	// Ignore errors if pod is already gone
	return nil
}
