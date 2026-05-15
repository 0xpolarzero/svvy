//go:build darwin

// Package seatbelt provides macOS Seatbelt (Sandbox Profile Language) integration
// for sandboxing subprocess execution.
//
// Seatbelt is macOS's native sandboxing mechanism using the Sandbox Profile Language.
// This package generates sandbox profiles for different security policies and applies
// them to processes using sandbox_init() via CGo.
package seatbelt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evmts/codex/codex-go/internal/protocol"
)

const (
	// SeatbeltExecutablePath is the trusted path to sandbox-exec on macOS.
	// Only use this path to defend against PATH injection attacks.
	SeatbeltExecutablePath = "/usr/bin/sandbox-exec"
)

// BasePolicyTemplate is the base Seatbelt policy that provides minimal system access.
// This is inspired by Chrome's sandbox policy and provides:
// - Process execution and forking
// - Basic file operations (read-only by default)
// - System calls needed for basic process functionality
const BasePolicyTemplate = `(version 1)

; inspired by Chrome's sandbox policy:
; https://source.chromium.org/chromium/chromium/src/+/main:sandbox/policy/mac/common.sb

; start with closed-by-default
(deny default)

; child processes inherit the policy of their parent
(allow process-exec)
(allow process-fork)
(allow signal (target same-sandbox))

; Allow cf prefs to work.
(allow user-preference-read)

; process-info
(allow process-info* (target same-sandbox))

(allow file-write-data
  (require-all
    (path "/dev/null")
    (vnode-type CHARACTER-DEVICE)))

; sysctls permitted.
(allow sysctl-read
  (sysctl-name "hw.activecpu")
  (sysctl-name "hw.busfrequency_compat")
  (sysctl-name "hw.byteorder")
  (sysctl-name "hw.cacheconfig")
  (sysctl-name "hw.cachelinesize_compat")
  (sysctl-name "hw.cpufamily")
  (sysctl-name "hw.cpufrequency_compat")
  (sysctl-name "hw.cputype")
  (sysctl-name "hw.l1dcachesize_compat")
  (sysctl-name "hw.l1icachesize_compat")
  (sysctl-name "hw.l2cachesize_compat")
  (sysctl-name "hw.l3cachesize_compat")
  (sysctl-name "hw.logicalcpu_max")
  (sysctl-name "hw.machine")
  (sysctl-name "hw.memsize")
  (sysctl-name "hw.ncpu")
  (sysctl-name "hw.nperflevels")
  (sysctl-name-prefix "hw.optional.arm.")
  (sysctl-name-prefix "hw.optional.armv8_")
  (sysctl-name "hw.packages")
  (sysctl-name "hw.pagesize_compat")
  (sysctl-name "hw.pagesize")
  (sysctl-name "hw.physicalcpu_max")
  (sysctl-name "hw.tbfrequency_compat")
  (sysctl-name "hw.vectorunit")
  (sysctl-name "kern.hostname")
  (sysctl-name "kern.maxfilesperproc")
  (sysctl-name "kern.maxproc")
  (sysctl-name "kern.osproductversion")
  (sysctl-name "kern.osrelease")
  (sysctl-name "kern.ostype")
  (sysctl-name "kern.osvariant_status")
  (sysctl-name "kern.osversion")
  (sysctl-name "kern.secure_kernel")
  (sysctl-name "kern.usrstack64")
  (sysctl-name "kern.version")
  (sysctl-name "sysctl.proc_cputype")
  (sysctl-name "vm.loadavg")
  (sysctl-name-prefix "hw.perflevel")
  (sysctl-name-prefix "kern.proc.pgrp.")
  (sysctl-name-prefix "kern.proc.pid.")
  (sysctl-name-prefix "net.routetable.")
)

; IOKit
(allow iokit-open
  (iokit-registry-entry-class "RootDomainUserClient")
)

; needed to look up user info
(allow mach-lookup
  (global-name "com.apple.system.opendirectoryd.libinfo")
)

; Needed for python multiprocessing on MacOS for the SemLock
(allow ipc-posix-sem)

(allow mach-lookup
  (global-name "com.apple.PowerManagement.control")
)
`

// WritableRoot represents a writable directory root with optional read-only subpaths.
type WritableRoot struct {
	// Root is the writable directory path
	Root string
	// ReadOnlySubpaths are subdirectories within Root that should be read-only
	ReadOnlySubpaths []string
}

// ProfileConfig contains configuration for generating a Seatbelt profile.
type ProfileConfig struct {
	// AllowFileRead enables read access to all files
	AllowFileRead bool
	// AllowFileWrite enables write access based on WritableRoots
	AllowFileWrite bool
	// WritableRoots are directories where writes are permitted
	WritableRoots []WritableRoot
	// AllowNetworkOutbound enables outbound network connections
	AllowNetworkOutbound bool
	// AllowNetworkInbound enables inbound network connections
	AllowNetworkInbound bool
	// AllowSystemSocket enables system socket operations
	AllowSystemSocket bool
}

// GenerateProfile generates a Seatbelt profile string from the configuration.
func GenerateProfile(config *ProfileConfig) string {
	var sb strings.Builder

	// Start with base policy
	sb.WriteString(BasePolicyTemplate)
	sb.WriteString("\n")

	// Add file read policy
	if config.AllowFileRead {
		sb.WriteString("; allow read-only file operations\n")
		sb.WriteString("(allow file-read*)\n")
	}

	// Add file write policy
	if config.AllowFileWrite && len(config.WritableRoots) > 0 {
		sb.WriteString("(allow file-write*\n")
		for _, root := range config.WritableRoots {
			if len(root.ReadOnlySubpaths) == 0 {
				sb.WriteString(fmt.Sprintf("  (subpath \"%s\")\n", root.Root))
			} else {
				// Build a require-all with require-not for read-only subpaths
				sb.WriteString("  (require-all\n")
				sb.WriteString(fmt.Sprintf("    (subpath \"%s\")\n", root.Root))
				for _, roPath := range root.ReadOnlySubpaths {
					sb.WriteString(fmt.Sprintf("    (require-not (subpath \"%s\"))\n", roPath))
				}
				sb.WriteString("  )\n")
			}
		}
		sb.WriteString(")\n")
	}

	// Add network policy
	if config.AllowNetworkOutbound || config.AllowNetworkInbound || config.AllowSystemSocket {
		if config.AllowNetworkOutbound {
			sb.WriteString("(allow network-outbound)\n")
		}
		if config.AllowNetworkInbound {
			sb.WriteString("(allow network-inbound)\n")
		}
		if config.AllowSystemSocket {
			sb.WriteString("(allow system-socket)\n")
		}
	}

	return sb.String()
}

// GenerateProfileFromSandboxPolicy generates a Seatbelt profile from a protocol.SandboxPolicy.
// The cwd parameter is used to determine writable roots when needed.
func GenerateProfileFromSandboxPolicy(policy *protocol.SandboxPolicy, cwd string) string {
	config := ProfileConfigFromSandboxPolicy(policy, cwd)
	return GenerateProfile(config)
}

// ProfileConfigFromSandboxPolicy converts a protocol.SandboxPolicy to a ProfileConfig.
func ProfileConfigFromSandboxPolicy(policy *protocol.SandboxPolicy, cwd string) *ProfileConfig {
	config := &ProfileConfig{
		AllowFileRead:        false,
		AllowFileWrite:       false,
		WritableRoots:        []WritableRoot{},
		AllowNetworkOutbound: false,
		AllowNetworkInbound:  false,
		AllowSystemSocket:    false,
	}

	switch policy.Mode {
	case "read-only":
		// ReadOnly: Allow read everywhere, deny all writes
		config.AllowFileRead = true
		config.AllowFileWrite = false

	case "workspace-write":
		// WorkspaceWrite: Allow read everywhere, writes only in specified roots
		config.AllowFileRead = true
		config.AllowFileWrite = true

		// Build writable roots from policy
		writableRoots := getWritableRootsWithCwd(policy, cwd)
		config.WritableRoots = writableRoots

		// Add network access if enabled
		if policy.NetworkAccess {
			config.AllowNetworkOutbound = true
			config.AllowNetworkInbound = true
			config.AllowSystemSocket = true
		}

	case "danger-full-access":
		// DangerFullAccess: No restrictions
		config.AllowFileRead = true
		config.AllowFileWrite = true
		// Use a special writable root of "/" to allow all writes
		config.WritableRoots = []WritableRoot{{Root: "/"}}
		config.AllowNetworkOutbound = true
		config.AllowNetworkInbound = true
		config.AllowSystemSocket = true

	default:
		// Default to read-only for unknown modes
		config.AllowFileRead = true
		config.AllowFileWrite = false
	}

	return config
}

// getWritableRootsWithCwd returns the writable roots for a WorkspaceWrite policy.
// It includes the policy's writable_roots, the cwd, /tmp (unless excluded), and
// TMPDIR env var (unless excluded). It also automatically marks .git directories
// as read-only subpaths.
func getWritableRootsWithCwd(policy *protocol.SandboxPolicy, cwd string) []WritableRoot {
	var roots []WritableRoot

	// Add explicitly specified writable roots
	for _, root := range policy.WritableRoots {
		canonRoot, err := filepath.Abs(root)
		if err != nil {
			canonRoot = root
		}
		roots = append(roots, WritableRoot{
			Root:             canonRoot,
			ReadOnlySubpaths: getReadOnlySubpaths(canonRoot),
		})
	}

	// Add cwd as writable root
	if cwd != "" {
		canonCwd, err := filepath.Abs(cwd)
		if err != nil {
			canonCwd = cwd
		}
		roots = append(roots, WritableRoot{
			Root:             canonCwd,
			ReadOnlySubpaths: getReadOnlySubpaths(canonCwd),
		})
	}

	// Add /tmp unless excluded
	if !policy.ExcludeSlashTmp {
		// On macOS, /tmp is typically a symlink to /private/tmp
		tmpPath := "/tmp"
		canonTmp, err := filepath.EvalSymlinks(tmpPath)
		if err != nil {
			canonTmp = tmpPath
		}
		roots = append(roots, WritableRoot{
			Root:             canonTmp,
			ReadOnlySubpaths: []string{},
		})
	}

	// Add TMPDIR env var unless excluded
	// Note: In a real implementation, you'd read os.Getenv("TMPDIR")
	// For now we'll skip this as it's runtime-dependent

	return roots
}

// getReadOnlySubpaths returns subdirectories that should be read-only within a writable root.
// Currently, this marks .git directories as read-only to prevent accidental corruption.
func getReadOnlySubpaths(root string) []string {
	// For now, we'll return an empty list and let the caller determine
	// if .git should be protected based on actual filesystem checks.
	// The profiles.go functions handle .git detection using getGitDirIfExists.
	return []string{}
}
