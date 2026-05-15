//go:build linux

package seccomp

import (
	"fmt"
)

// BPF instruction opcodes
const (
	// Load operations
	BPF_LD  = 0x00 // Load word
	BPF_LDX = 0x01 // Load word from X
	BPF_LDI = 0x40 // Load immediate (unused in seccomp)
	BPF_LDB = 0x10 // Load byte
	BPF_LDH = 0x08 // Load half-word

	// Store operations
	BPF_ST  = 0x02 // Store
	BPF_STX = 0x03 // Store X

	// ALU operations
	BPF_ALU = 0x04 // ALU operations
	BPF_ADD = 0x00 // Add
	BPF_SUB = 0x10 // Subtract
	BPF_MUL = 0x20 // Multiply
	BPF_DIV = 0x30 // Divide
	BPF_OR  = 0x40 // Bitwise OR
	BPF_AND = 0x50 // Bitwise AND
	BPF_LSH = 0x60 // Left shift
	BPF_RSH = 0x70 // Right shift
	BPF_NEG = 0x80 // Negate
	BPF_MOD = 0x90 // Modulo
	BPF_XOR = 0xa0 // Bitwise XOR

	// Jump operations
	BPF_JMP  = 0x05 // Jump
	BPF_JA   = 0x00 // Jump always
	BPF_JEQ  = 0x10 // Jump if equal
	BPF_JGT  = 0x20 // Jump if greater than
	BPF_JGE  = 0x30 // Jump if greater or equal
	BPF_JSET = 0x40 // Jump if set (bitwise AND)

	// Return operations
	BPF_RET = 0x06 // Return
	BPF_K   = 0x00 // Use constant
	BPF_X   = 0x08 // Use X register
	BPF_A   = 0x10 // Use accumulator

	// Miscellaneous
	BPF_MISC = 0x07
	BPF_TAX  = 0x00 // Transfer A to X
	BPF_TXA  = 0x80 // Transfer X to A

	// Addressing modes
	BPF_IMM = 0x00 // Immediate value
	BPF_ABS = 0x20 // Absolute offset
	BPF_IND = 0x40 // Indirect offset
	BPF_MEM = 0x60 // Memory location
	BPF_LEN = 0x80 // Length of packet (unused in seccomp)
	BPF_MSH = 0xa0 // 4*(P[k:1]&0xf) (unused in seccomp)
)

// Seccomp data structure offsets (struct seccomp_data)
const (
	// offsetNr is the offset of the syscall number (int)
	offsetNr = 0
	// offsetArch is the offset of the architecture (__u32)
	offsetArch = 4
	// offsetInstructionPointer is the offset of the instruction pointer (__u64)
	offsetInstructionPointer = 8
	// offsetArgs is the offset of the syscall arguments array (__u64[6])
	offsetArgs = 16
)

// Architecture audit constants (AUDIT_ARCH_*)
const (
	// AUDIT_ARCH_X86_64 - x86_64 (amd64) architecture
	AUDIT_ARCH_X86_64 = 0xc000003e
	// AUDIT_ARCH_AARCH64 - ARM64 architecture
	AUDIT_ARCH_AARCH64 = 0xc00000b7
	// AUDIT_ARCH_I386 - x86 (32-bit) architecture
	AUDIT_ARCH_I386 = 0x40000003
	// AUDIT_ARCH_ARM - ARM (32-bit) architecture
	AUDIT_ARCH_ARM = 0x40000028
)

// FilterAction represents the action to take when a syscall matches
type FilterAction uint32

const (
	// ActionAllow permits the syscall
	ActionAllow FilterAction = SECCOMP_RET_ALLOW
	// ActionErrno returns an errno value (default: EPERM)
	ActionErrno FilterAction = SECCOMP_RET_ERRNO
	// ActionTrap sends a SIGSYS signal
	ActionTrap FilterAction = SECCOMP_RET_TRAP
	// ActionKillThread kills the calling thread
	ActionKillThread FilterAction = SECCOMP_RET_KILL_THREAD
	// ActionKillProcess kills the entire process
	ActionKillProcess FilterAction = SECCOMP_RET_KILL_PROCESS
	// ActionLog allows but logs the syscall
	ActionLog FilterAction = SECCOMP_RET_LOG
)

// FilterBuilder helps construct BPF filter programs
type FilterBuilder struct {
	instructions  []sockFilter
	arch          uint32
	defaultAction FilterAction
}

// NewFilterBuilder creates a new BPF filter builder
//
// Parameters:
//   - arch: target architecture ("amd64", "arm64")
//   - defaultAction: action to take when no rules match (typically ActionAllow or ActionErrno)
func NewFilterBuilder(arch string, defaultAction FilterAction) (*FilterBuilder, error) {
	var archConst uint32
	switch arch {
	case "amd64":
		archConst = AUDIT_ARCH_X86_64
	case "arm64":
		archConst = AUDIT_ARCH_AARCH64
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}

	fb := &FilterBuilder{
		instructions:  make([]sockFilter, 0, 128),
		arch:          archConst,
		defaultAction: defaultAction,
	}

	// Start by validating the architecture
	fb.validateArchitecture()

	return fb, nil
}

// validateArchitecture adds BPF instructions to validate the syscall architecture
// matches the expected architecture. This prevents cross-architecture syscall confusion.
func (fb *FilterBuilder) validateArchitecture() {
	// Load architecture from seccomp_data (offset 4)
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, offsetArch)

	// Compare with expected architecture
	// If not equal, jump to the "kill" instruction (last instruction)
	fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 1, fb.arch)

	// Architecture mismatch - kill the process
	fb.addInstruction(BPF_RET|BPF_K, 0, 0, uint32(SECCOMP_RET_KILL_PROCESS))
}

// DenySyscall adds a rule to deny a specific syscall with EPERM
func (fb *FilterBuilder) DenySyscall(syscallNr int) {
	// Load syscall number from seccomp_data (offset 0)
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, offsetNr)

	// If syscall matches, return EPERM; otherwise skip to next rule
	fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 1, uint32(syscallNr))
	fb.addInstruction(BPF_RET|BPF_K, 0, 0, uint32(SECCOMP_RET_ERRNO|EPERM))
}

// AllowSyscall adds a rule to explicitly allow a specific syscall
func (fb *FilterBuilder) AllowSyscall(syscallNr int) {
	// Load syscall number from seccomp_data (offset 0)
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, offsetNr)

	// If syscall matches, allow it; otherwise skip to next rule
	fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 1, uint32(syscallNr))
	fb.addInstruction(BPF_RET|BPF_K, 0, 0, uint32(SECCOMP_RET_ALLOW))
}

// DenySyscallConditional denies a syscall if a specific argument condition is met
//
// Parameters:
//   - syscallNr: the syscall number to check
//   - argIndex: which argument to check (0-5)
//   - argValue: the value to compare against
//
// Example: To deny socket() calls for non-AF_UNIX domains:
//
//	DenySyscallConditional(SYS_SOCKET, 0, AF_UNIX, false)
func (fb *FilterBuilder) DenySyscallConditional(syscallNr int, argIndex int, argValue uint64, invertMatch bool) error {
	if argIndex < 0 || argIndex > 5 {
		return fmt.Errorf("invalid argument index: %d (must be 0-5)", argIndex)
	}

	// Load syscall number
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, offsetNr)

	// Check if it's the target syscall
	// If not, skip this entire rule (jump forward by the number of instructions in this rule)
	fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 5, uint32(syscallNr))

	// Load the specified argument (64-bit value, load high 32 bits)
	argOffset := offsetArgs + (argIndex * 8)
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, uint32(argOffset+4))

	// Check high 32 bits == 0 (assuming argument fits in 32 bits)
	fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 1, uint32(argValue>>32))

	// Load low 32 bits
	fb.addInstruction(BPF_LD|BPF_W|BPF_ABS, 0, 0, uint32(argOffset))

	// Compare with expected value
	if invertMatch {
		// If NOT equal to argValue, deny
		fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 1, 0, uint32(argValue&0xFFFFFFFF))
	} else {
		// If equal to argValue, deny
		fb.addInstruction(BPF_JMP|BPF_JEQ|BPF_K, 0, 1, uint32(argValue&0xFFFFFFFF))
	}
	fb.addInstruction(BPF_RET|BPF_K, 0, 0, uint32(SECCOMP_RET_ERRNO|EPERM))

	return nil
}

// Build finalizes the filter and returns a SeccompFilter ready to apply.
// It adds the default action as the final fallback instruction.
func (fb *FilterBuilder) Build() *SeccompFilter {
	// Add default action as the last instruction
	fb.addInstruction(BPF_RET|BPF_K, 0, 0, uint32(fb.defaultAction))

	return &SeccompFilter{
		program: fb.instructions,
		arch:    fmt.Sprintf("0x%x", fb.arch),
	}
}

// addInstruction is a helper to add a BPF instruction to the program
func (fb *FilterBuilder) addInstruction(code uint16, jt, jf uint8, k uint32) {
	fb.instructions = append(fb.instructions, sockFilter{
		Code: code,
		Jt:   jt,
		Jf:   jf,
		K:    k,
	})
}

// BPF_W is a shorthand for word (32-bit) operations
const BPF_W = 0x00

// CreateNetworkFilter creates a Seccomp filter that blocks network syscalls
// except for AF_UNIX domain sockets. This is used to implement network isolation
// while allowing local IPC.
//
// Denied syscalls:
//   - socket (except AF_UNIX)
//   - connect, accept, accept4
//   - bind, listen, shutdown
//   - sendto, sendmsg, sendmmsg
//   - recvmsg, recvmmsg
//   - getsockopt, setsockopt
//   - getpeername, getsockname
//
// This matches the Rust implementation's network seccomp filter.
func CreateNetworkFilter(arch string) (*SeccompFilter, error) {
	fb, err := NewFilterBuilder(arch, ActionAllow)
	if err != nil {
		return nil, err
	}

	// Get syscall numbers for the current architecture
	syscalls := GetSyscallNumbers(arch)

	// Deny network-related syscalls
	fb.DenySyscall(syscalls.Connect)
	fb.DenySyscall(syscalls.Accept)
	fb.DenySyscall(syscalls.Accept4)
	fb.DenySyscall(syscalls.Bind)
	fb.DenySyscall(syscalls.Listen)
	fb.DenySyscall(syscalls.Shutdown)
	fb.DenySyscall(syscalls.Sendto)
	fb.DenySyscall(syscalls.Sendmsg)
	fb.DenySyscall(syscalls.Sendmmsg)
	fb.DenySyscall(syscalls.Recvmsg)
	fb.DenySyscall(syscalls.Recvmmsg)
	fb.DenySyscall(syscalls.Getsockopt)
	fb.DenySyscall(syscalls.Setsockopt)
	fb.DenySyscall(syscalls.Getpeername)
	fb.DenySyscall(syscalls.Getsockname)
	fb.DenySyscall(syscalls.Ptrace)

	// Socket: allow AF_UNIX (1), deny others
	// Deny socket() if arg0 (domain) != AF_UNIX
	if err := fb.DenySyscallConditional(syscalls.Socket, 0, 1 /* AF_UNIX */, true); err != nil {
		return nil, fmt.Errorf("failed to add socket filter: %w", err)
	}

	// Socketpair: allow AF_UNIX, deny others
	if err := fb.DenySyscallConditional(syscalls.Socketpair, 0, 1 /* AF_UNIX */, true); err != nil {
		return nil, fmt.Errorf("failed to add socketpair filter: %w", err)
	}

	return fb.Build(), nil
}

// CreateRestrictiveFilter creates a highly restrictive Seccomp filter that
// only allows safe, read-only syscalls. This is useful for maximum isolation.
func CreateRestrictiveFilter(arch string, allowedSyscalls []int) (*SeccompFilter, error) {
	fb, err := NewFilterBuilder(arch, ActionErrno)
	if err != nil {
		return nil, err
	}

	// Allow only explicitly listed syscalls
	for _, syscallNr := range allowedSyscalls {
		fb.AllowSyscall(syscallNr)
	}

	return fb.Build(), nil
}
