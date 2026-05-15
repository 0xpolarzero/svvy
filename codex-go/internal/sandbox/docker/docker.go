// Package docker provides Docker container-based command execution sandbox.
//
// The Docker sandbox creates isolated containers for each command execution with:
//   - Filesystem isolation via volume mounts
//   - Network policy enforcement
//   - Resource limits (CPU, memory)
//   - Automatic container cleanup (--rm flag)
//
// Configuration options include image, network mode, and resource limits.
package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/evmts/codex/codex-go/internal/sandbox"
)

// Options configures the Docker sandbox behavior.
type Options struct {
	// Image is the Docker image to use (default: "ubuntu:22.04").
	Image string

	// Network is the Docker network mode (default: "none").
	// Can be "none", "bridge", "host", etc.
	Network string

	// MemoryLimit is the maximum memory allocation (e.g., "512m", "1g").
	MemoryLimit string

	// CPULimit is the maximum CPU allocation (e.g., "1.0", "0.5").
	CPULimit string

	// CleanupTimeout is how long to wait for container cleanup (default: 30s).
	CleanupTimeout time.Duration
}

// DockerSandbox executes commands in isolated Docker containers.
type DockerSandbox struct {
	commander sandbox.Commander
	opts      *Options
}

// NewDockerSandbox creates a new Docker sandbox with the given options.
// If opts is nil, default options are used.
func NewDockerSandbox(commander sandbox.Commander, opts *Options) *DockerSandbox {
	if opts == nil {
		opts = &Options{}
	}

	// Apply defaults
	if opts.Image == "" {
		opts.Image = "ubuntu:22.04"
	}
	if opts.Network == "" {
		opts.Network = "none"
	}
	if opts.CleanupTimeout == 0 {
		opts.CleanupTimeout = 30 * time.Second
	}

	return &DockerSandbox{
		commander: commander,
		opts:      opts,
	}
}

// Type returns the sandbox type identifier.
func (d *DockerSandbox) Type() string {
	return "docker"
}

// IsAvailable checks if Docker is available on the system.
func (d *DockerSandbox) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, exitCode, err := d.commander.Run(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	return err == nil && exitCode == 0
}

// Execute runs a command in a Docker container.
func (d *DockerSandbox) Execute(ctx context.Context, cmd *sandbox.Command) (*sandbox.Result, error) {
	startTime := time.Now()

	// Apply command timeout if specified
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	// Build docker run arguments
	args := d.buildDockerArgs(cmd)

	// Execute docker run
	stdout, stderr, exitCode, err := d.commander.Run(ctx, "docker", args...)

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Check for docker execution failure
	if err != nil && exitCode >= 125 {
		// Exit codes >= 125 indicate Docker daemon errors, not command failures
		return nil, fmt.Errorf("docker run failed (exit %d): %s %s", exitCode, stdout, stderr)
	}

	executionTime := time.Since(startTime)

	return &sandbox.Result{
		Stdout:        stdout,
		Stderr:        stderr,
		ExitCode:      exitCode,
		ExecutionTime: executionTime,
	}, nil
}

// Cleanup performs any necessary cleanup.
// Docker containers are automatically removed via --rm flag, so this is a no-op.
func (d *DockerSandbox) Cleanup(ctx context.Context) error {
	return nil
}

// buildDockerArgs constructs the docker run command arguments.
func (d *DockerSandbox) buildDockerArgs(cmd *sandbox.Command) []string {
	args := []string{"run"}

	// Always remove container after execution
	args = append(args, "--rm")

	// Set network mode
	network := d.opts.Network
	if cmd.NetworkEnabled {
		network = "bridge"
	}
	args = append(args, fmt.Sprintf("--network=%s", network))

	// Set working directory
	if cmd.WorkingDirectory != "" {
		args = append(args, "-w", cmd.WorkingDirectory)
	}

	// Add environment variables
	for key, value := range cmd.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volume mounts - read-write paths
	for _, path := range cmd.ReadWritePaths {
		args = append(args, "-v", fmt.Sprintf("%s:%s", path, path))
	}

	// Add volume mounts - read-only paths
	for _, path := range cmd.ReadOnlyPaths {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", path, path))
	}

	// Add resource limits if specified
	if d.opts.MemoryLimit != "" {
		args = append(args, fmt.Sprintf("--memory=%s", d.opts.MemoryLimit))
	}
	if d.opts.CPULimit != "" {
		args = append(args, fmt.Sprintf("--cpus=%s", d.opts.CPULimit))
	}

	// Add security options (run as non-root when possible)
	args = append(args, "--security-opt", "no-new-privileges")

	// Disable TTY allocation
	args = append(args, "-i")

	// Add image
	args = append(args, d.opts.Image)

	// Add command and arguments
	args = append(args, cmd.Program)
	args = append(args, cmd.Args...)

	return args
}
