//go:build linux

package seccomp

import (
	"syscall"
)

// SyscallNumbers contains architecture-specific syscall numbers.
// Different architectures (amd64, arm64) have different syscall numbers
// for the same operations, so we need to map them correctly.
type SyscallNumbers struct {
	// Network syscalls
	Socket      int
	Socketpair  int
	Connect     int
	Accept      int
	Accept4     int
	Bind        int
	Listen      int
	Shutdown    int
	Sendto      int
	Sendmsg     int
	Sendmmsg    int
	Recvfrom    int
	Recvmsg     int
	Recvmmsg    int
	Getsockopt  int
	Setsockopt  int
	Getpeername int
	Getsockname int

	// Process/security syscalls
	Ptrace          int
	Execve          int
	Execveat        int
	Clone           int
	Clone3          int
	Fork            int
	Vfork           int
	Unshare         int
	Setns           int
	Pivot_root      int
	Chroot          int
	Mount           int
	Umount2         int
	Swapon          int
	Swapoff         int
	Reboot          int
	Init_module     int
	Delete_module   int
	Kexec_load      int
	Kexec_file_load int

	// Filesystem syscalls (safe)
	Read       int
	Write      int
	Open       int
	Openat     int
	Openat2    int
	Close      int
	Stat       int
	Fstat      int
	Lstat      int
	Statx      int
	Access     int
	Faccessat  int
	Faccessat2 int
	Readlink   int
	Readlinkat int
	Getcwd     int
	Chdir      int
	Fchdir     int

	// Memory syscalls
	Mmap     int
	Munmap   int
	Mprotect int
	Madvise  int
	Brk      int
	Mremap   int
	Msync    int
	Mincore  int
	Mlock    int
	Munlock  int

	// Process management (safe)
	Getpid    int
	Getppid   int
	Getuid    int
	Geteuid   int
	Getgid    int
	Getegid   int
	Gettid    int
	Getpgid   int
	Setpgid   int
	Getpgrp   int
	Getsid    int
	Setsid    int
	Getrlimit int
	Setrlimit int
	Getrusage int
	Times     int
	Umask     int

	// Signal handling
	Rt_sigaction   int
	Rt_sigprocmask int
	Rt_sigreturn   int
	Sigaltstack    int
	Kill           int
	Tkill          int
	Tgkill         int

	// Time syscalls
	Nanosleep       int
	Clock_gettime   int
	Clock_getres    int
	Clock_nanosleep int
	Gettimeofday    int

	// File descriptor operations
	Dup           int
	Dup2          int
	Dup3          int
	Fcntl         int
	Ioctl         int
	Select        int
	Pselect6      int
	Poll          int
	Ppoll         int
	Epoll_create  int
	Epoll_create1 int
	Epoll_ctl     int
	Epoll_wait    int
	Epoll_pwait   int

	// IPC (safe: pipes, eventfd)
	Pipe            int
	Pipe2           int
	Eventfd         int
	Eventfd2        int
	Signalfd        int
	Signalfd4       int
	Timerfd_create  int
	Timerfd_settime int
	Timerfd_gettime int

	// Thread/futex
	Futex           int
	Set_robust_list int
	Get_robust_list int

	// Exit
	Exit       int
	Exit_group int
}

// GetSyscallNumbers returns the syscall numbers for the given architecture
func GetSyscallNumbers(arch string) *SyscallNumbers {
	switch arch {
	case "amd64":
		return getAmd64Syscalls()
	case "arm64":
		return getArm64Syscalls()
	default:
		// Default to current architecture
		return getAmd64Syscalls()
	}
}

// getAmd64Syscalls returns syscall numbers for x86_64 architecture
func getAmd64Syscalls() *SyscallNumbers {
	return &SyscallNumbers{
		// Network
		Socket:      syscall.SYS_SOCKET,      // 41
		Socketpair:  syscall.SYS_SOCKETPAIR,  // 53
		Connect:     syscall.SYS_CONNECT,     // 42
		Accept:      syscall.SYS_ACCEPT,      // 43
		Accept4:     syscall.SYS_ACCEPT4,     // 288
		Bind:        syscall.SYS_BIND,        // 49
		Listen:      syscall.SYS_LISTEN,      // 50
		Shutdown:    syscall.SYS_SHUTDOWN,    // 48
		Sendto:      syscall.SYS_SENDTO,      // 44
		Sendmsg:     syscall.SYS_SENDMSG,     // 46
		Sendmmsg:    syscall.SYS_SENDMMSG,    // 307
		Recvfrom:    syscall.SYS_RECVFROM,    // 45
		Recvmsg:     syscall.SYS_RECVMSG,     // 47
		Recvmmsg:    syscall.SYS_RECVMMSG,    // 299
		Getsockopt:  syscall.SYS_GETSOCKOPT,  // 55
		Setsockopt:  syscall.SYS_SETSOCKOPT,  // 54
		Getpeername: syscall.SYS_GETPEERNAME, // 52
		Getsockname: syscall.SYS_GETSOCKNAME, // 51

		// Process/security
		Ptrace:          syscall.SYS_PTRACE,     // 101
		Execve:          syscall.SYS_EXECVE,     // 59
		Execveat:        syscall.SYS_EXECVEAT,   // 322
		Clone:           syscall.SYS_CLONE,      // 56
		Clone3:          435,                    // 435 (SYS_CLONE3)
		Fork:            syscall.SYS_FORK,       // 57
		Vfork:           syscall.SYS_VFORK,      // 58
		Unshare:         syscall.SYS_UNSHARE,    // 272
		Setns:           syscall.SYS_SETNS,      // 308
		Pivot_root:      syscall.SYS_PIVOT_ROOT, // 155
		Chroot:          syscall.SYS_CHROOT,     // 161
		Mount:           syscall.SYS_MOUNT,      // 165
		Umount2:         syscall.SYS_UMOUNT2,    // 166
		Swapon:          syscall.SYS_SWAPON,     // 167
		Swapoff:         syscall.SYS_SWAPOFF,    // 168
		Reboot:          syscall.SYS_REBOOT,     // 169
		Init_module:     175,                    // 175 (SYS_INIT_MODULE)
		Delete_module:   176,                    // 176 (SYS_DELETE_MODULE)
		Kexec_load:      syscall.SYS_KEXEC_LOAD, // 246
		Kexec_file_load: 320,                    // 320 (SYS_KEXEC_FILE_LOAD)

		// Filesystem (safe)
		Read:       syscall.SYS_READ,       // 0
		Write:      syscall.SYS_WRITE,      // 1
		Open:       syscall.SYS_OPEN,       // 2
		Openat:     syscall.SYS_OPENAT,     // 257
		Openat2:    437,                    // 437 (SYS_OPENAT2)
		Close:      syscall.SYS_CLOSE,      // 3
		Stat:       syscall.SYS_STAT,       // 4
		Fstat:      syscall.SYS_FSTAT,      // 5
		Lstat:      syscall.SYS_LSTAT,      // 6
		Statx:      332,                    // 332 (SYS_STATX)
		Access:     syscall.SYS_ACCESS,     // 21
		Faccessat:  syscall.SYS_FACCESSAT,  // 269
		Faccessat2: 439,                    // 439 (SYS_FACCESSAT2)
		Readlink:   syscall.SYS_READLINK,   // 89
		Readlinkat: syscall.SYS_READLINKAT, // 267
		Getcwd:     syscall.SYS_GETCWD,     // 79
		Chdir:      syscall.SYS_CHDIR,      // 80
		Fchdir:     syscall.SYS_FCHDIR,     // 81

		// Memory
		Mmap:     syscall.SYS_MMAP,     // 9
		Munmap:   syscall.SYS_MUNMAP,   // 11
		Mprotect: syscall.SYS_MPROTECT, // 10
		Madvise:  syscall.SYS_MADVISE,  // 28
		Brk:      syscall.SYS_BRK,      // 12
		Mremap:   syscall.SYS_MREMAP,   // 25
		Msync:    syscall.SYS_MSYNC,    // 26
		Mincore:  syscall.SYS_MINCORE,  // 27
		Mlock:    syscall.SYS_MLOCK,    // 149
		Munlock:  syscall.SYS_MUNLOCK,  // 150

		// Process management
		Getpid:    syscall.SYS_GETPID,    // 39
		Getppid:   syscall.SYS_GETPPID,   // 110
		Getuid:    syscall.SYS_GETUID,    // 102
		Geteuid:   syscall.SYS_GETEUID,   // 107
		Getgid:    syscall.SYS_GETGID,    // 104
		Getegid:   syscall.SYS_GETEGID,   // 108
		Gettid:    syscall.SYS_GETTID,    // 186
		Getpgid:   syscall.SYS_GETPGID,   // 121
		Setpgid:   syscall.SYS_SETPGID,   // 109
		Getpgrp:   syscall.SYS_GETPGRP,   // 111
		Getsid:    syscall.SYS_GETSID,    // 124
		Setsid:    syscall.SYS_SETSID,    // 112
		Getrlimit: syscall.SYS_GETRLIMIT, // 97
		Setrlimit: syscall.SYS_SETRLIMIT, // 160
		Getrusage: syscall.SYS_GETRUSAGE, // 98
		Times:     syscall.SYS_TIMES,     // 100
		Umask:     syscall.SYS_UMASK,     // 95

		// Signals
		Rt_sigaction:   syscall.SYS_RT_SIGACTION,   // 13
		Rt_sigprocmask: syscall.SYS_RT_SIGPROCMASK, // 14
		Rt_sigreturn:   syscall.SYS_RT_SIGRETURN,   // 15
		Sigaltstack:    syscall.SYS_SIGALTSTACK,    // 131
		Kill:           syscall.SYS_KILL,           // 62
		Tkill:          syscall.SYS_TKILL,          // 200
		Tgkill:         syscall.SYS_TGKILL,         // 234

		// Time
		Nanosleep:       syscall.SYS_NANOSLEEP,       // 35
		Clock_gettime:   syscall.SYS_CLOCK_GETTIME,   // 228
		Clock_getres:    syscall.SYS_CLOCK_GETRES,    // 229
		Clock_nanosleep: syscall.SYS_CLOCK_NANOSLEEP, // 230
		Gettimeofday:    syscall.SYS_GETTIMEOFDAY,    // 96

		// File descriptors
		Dup:           syscall.SYS_DUP,           // 32
		Dup2:          syscall.SYS_DUP2,          // 33
		Dup3:          syscall.SYS_DUP3,          // 292
		Fcntl:         syscall.SYS_FCNTL,         // 72
		Ioctl:         syscall.SYS_IOCTL,         // 16
		Select:        syscall.SYS_SELECT,        // 23
		Pselect6:      syscall.SYS_PSELECT6,      // 270
		Poll:          syscall.SYS_POLL,          // 7
		Ppoll:         syscall.SYS_PPOLL,         // 271
		Epoll_create:  syscall.SYS_EPOLL_CREATE,  // 213
		Epoll_create1: syscall.SYS_EPOLL_CREATE1, // 291
		Epoll_ctl:     syscall.SYS_EPOLL_CTL,     // 233
		Epoll_wait:    syscall.SYS_EPOLL_WAIT,    // 232
		Epoll_pwait:   syscall.SYS_EPOLL_PWAIT,   // 281

		// IPC
		Pipe:            syscall.SYS_PIPE,            // 22
		Pipe2:           syscall.SYS_PIPE2,           // 293
		Eventfd:         syscall.SYS_EVENTFD,         // 284
		Eventfd2:        syscall.SYS_EVENTFD2,        // 290
		Signalfd:        syscall.SYS_SIGNALFD,        // 282
		Signalfd4:       syscall.SYS_SIGNALFD4,       // 289
		Timerfd_create:  syscall.SYS_TIMERFD_CREATE,  // 283
		Timerfd_settime: syscall.SYS_TIMERFD_SETTIME, // 286
		Timerfd_gettime: syscall.SYS_TIMERFD_GETTIME, // 287

		// Futex
		Futex:           syscall.SYS_FUTEX,           // 202
		Set_robust_list: syscall.SYS_SET_ROBUST_LIST, // 273
		Get_robust_list: syscall.SYS_GET_ROBUST_LIST, // 274

		// Exit
		Exit:       syscall.SYS_EXIT,       // 60
		Exit_group: syscall.SYS_EXIT_GROUP, // 231
	}
}

// getArm64Syscalls returns syscall numbers for ARM64 architecture
func getArm64Syscalls() *SyscallNumbers {
	return &SyscallNumbers{
		// Network
		Socket:      198, // SYS_SOCKET
		Socketpair:  199, // SYS_SOCKETPAIR
		Connect:     203, // SYS_CONNECT
		Accept:      202, // SYS_ACCEPT
		Accept4:     242, // SYS_ACCEPT4
		Bind:        200, // SYS_BIND
		Listen:      201, // SYS_LISTEN
		Shutdown:    210, // SYS_SHUTDOWN
		Sendto:      206, // SYS_SENDTO
		Sendmsg:     211, // SYS_SENDMSG
		Sendmmsg:    269, // SYS_SENDMMSG
		Recvfrom:    207, // SYS_RECVFROM
		Recvmsg:     212, // SYS_RECVMSG
		Recvmmsg:    243, // SYS_RECVMMSG
		Getsockopt:  209, // SYS_GETSOCKOPT
		Setsockopt:  208, // SYS_SETSOCKOPT
		Getpeername: 205, // SYS_GETPEERNAME
		Getsockname: 204, // SYS_GETSOCKNAME

		// Process/security
		Ptrace:          117, // SYS_PTRACE
		Execve:          221, // SYS_EXECVE
		Execveat:        281, // SYS_EXECVEAT
		Clone:           220, // SYS_CLONE
		Clone3:          435, // SYS_CLONE3
		Fork:            -1,  // Not available on ARM64
		Vfork:           -1,  // Not available on ARM64
		Unshare:         97,  // SYS_UNSHARE
		Setns:           268, // SYS_SETNS
		Pivot_root:      41,  // SYS_PIVOT_ROOT
		Chroot:          51,  // SYS_CHROOT
		Mount:           40,  // SYS_MOUNT
		Umount2:         39,  // SYS_UMOUNT2
		Swapon:          224, // SYS_SWAPON
		Swapoff:         225, // SYS_SWAPOFF
		Reboot:          142, // SYS_REBOOT
		Init_module:     105, // SYS_INIT_MODULE
		Delete_module:   106, // SYS_DELETE_MODULE
		Kexec_load:      104, // SYS_KEXEC_LOAD
		Kexec_file_load: 294, // SYS_KEXEC_FILE_LOAD

		// Filesystem (safe)
		Read:       63,  // SYS_READ
		Write:      64,  // SYS_WRITE
		Open:       -1,  // Not available on ARM64 (use openat)
		Openat:     56,  // SYS_OPENAT
		Openat2:    437, // SYS_OPENAT2
		Close:      57,  // SYS_CLOSE
		Stat:       -1,  // Not available on ARM64 (use fstatat)
		Fstat:      80,  // SYS_FSTAT
		Lstat:      -1,  // Not available on ARM64 (use fstatat)
		Statx:      291, // SYS_STATX
		Access:     -1,  // Not available on ARM64 (use faccessat)
		Faccessat:  48,  // SYS_FACCESSAT
		Faccessat2: 439, // SYS_FACCESSAT2
		Readlink:   -1,  // Not available on ARM64 (use readlinkat)
		Readlinkat: 78,  // SYS_READLINKAT
		Getcwd:     17,  // SYS_GETCWD
		Chdir:      49,  // SYS_CHDIR
		Fchdir:     50,  // SYS_FCHDIR

		// Memory
		Mmap:     222, // SYS_MMAP
		Munmap:   215, // SYS_MUNMAP
		Mprotect: 226, // SYS_MPROTECT
		Madvise:  233, // SYS_MADVISE
		Brk:      214, // SYS_BRK
		Mremap:   216, // SYS_MREMAP
		Msync:    227, // SYS_MSYNC
		Mincore:  232, // SYS_MINCORE
		Mlock:    228, // SYS_MLOCK
		Munlock:  229, // SYS_MUNLOCK

		// Process management
		Getpid:    172, // SYS_GETPID
		Getppid:   173, // SYS_GETPPID
		Getuid:    174, // SYS_GETUID
		Geteuid:   175, // SYS_GETEUID
		Getgid:    176, // SYS_GETGID
		Getegid:   177, // SYS_GETEGID
		Gettid:    178, // SYS_GETTID
		Getpgid:   155, // SYS_GETPGID
		Setpgid:   154, // SYS_SETPGID
		Getpgrp:   156, // SYS_GETPGRP
		Getsid:    157, // SYS_GETSID
		Setsid:    157, // SYS_SETSID
		Getrlimit: 163, // SYS_GETRLIMIT
		Setrlimit: 164, // SYS_SETRLIMIT
		Getrusage: 165, // SYS_GETRUSAGE
		Times:     153, // SYS_TIMES
		Umask:     166, // SYS_UMASK

		// Signals
		Rt_sigaction:   134, // SYS_RT_SIGACTION
		Rt_sigprocmask: 135, // SYS_RT_SIGPROCMASK
		Rt_sigreturn:   139, // SYS_RT_SIGRETURN
		Sigaltstack:    132, // SYS_SIGALTSTACK
		Kill:           129, // SYS_KILL
		Tkill:          130, // SYS_TKILL
		Tgkill:         131, // SYS_TGKILL

		// Time
		Nanosleep:       101, // SYS_NANOSLEEP
		Clock_gettime:   113, // SYS_CLOCK_GETTIME
		Clock_getres:    114, // SYS_CLOCK_GETRES
		Clock_nanosleep: 115, // SYS_CLOCK_NANOSLEEP
		Gettimeofday:    169, // SYS_GETTIMEOFDAY

		// File descriptors
		Dup:           23, // SYS_DUP
		Dup2:          -1, // Not available on ARM64 (use dup3)
		Dup3:          24, // SYS_DUP3
		Fcntl:         25, // SYS_FCNTL
		Ioctl:         29, // SYS_IOCTL
		Select:        -1, // Not available on ARM64 (use pselect6)
		Pselect6:      72, // SYS_PSELECT6
		Poll:          -1, // Not available on ARM64 (use ppoll)
		Ppoll:         73, // SYS_PPOLL
		Epoll_create:  -1, // Not available on ARM64 (use epoll_create1)
		Epoll_create1: 20, // SYS_EPOLL_CREATE1
		Epoll_ctl:     21, // SYS_EPOLL_CTL
		Epoll_wait:    -1, // Not available on ARM64 (use epoll_pwait)
		Epoll_pwait:   22, // SYS_EPOLL_PWAIT

		// IPC
		Pipe:            -1, // Not available on ARM64 (use pipe2)
		Pipe2:           59, // SYS_PIPE2
		Eventfd:         -1, // Not available on ARM64 (use eventfd2)
		Eventfd2:        19, // SYS_EVENTFD2
		Signalfd:        -1, // Not available on ARM64 (use signalfd4)
		Signalfd4:       74, // SYS_SIGNALFD4
		Timerfd_create:  85, // SYS_TIMERFD_CREATE
		Timerfd_settime: 86, // SYS_TIMERFD_SETTIME
		Timerfd_gettime: 87, // SYS_TIMERFD_GETTIME

		// Futex
		Futex:           98,  // SYS_FUTEX
		Set_robust_list: 99,  // SYS_SET_ROBUST_LIST
		Get_robust_list: 100, // SYS_GET_ROBUST_LIST

		// Exit
		Exit:       93, // SYS_EXIT
		Exit_group: 94, // SYS_EXIT_GROUP
	}
}

// GetSafeSyscallList returns a list of safe syscalls that should be allowed
// in a restrictive sandbox. These are primarily read-only operations.
func GetSafeSyscallList(arch string) []int {
	syscalls := GetSyscallNumbers(arch)

	safeSyscalls := []int{
		// Essential for process operation
		syscalls.Read,
		syscalls.Write,
		syscalls.Exit,
		syscalls.Exit_group,
		syscalls.Rt_sigreturn,

		// Memory management (needed by runtime)
		syscalls.Mmap,
		syscalls.Munmap,
		syscalls.Mprotect,
		syscalls.Madvise,
		syscalls.Brk,

		// File operations (read-only context)
		syscalls.Openat,
		syscalls.Close,
		syscalls.Fstat,
		syscalls.Statx,
		syscalls.Readlinkat,
		syscalls.Faccessat,
		syscalls.Getcwd,

		// Process info
		syscalls.Getpid,
		syscalls.Getppid,
		syscalls.Getuid,
		syscalls.Geteuid,
		syscalls.Getgid,
		syscalls.Getegid,
		syscalls.Gettid,

		// Time
		syscalls.Clock_gettime,
		syscalls.Clock_getres,
		syscalls.Gettimeofday,
		syscalls.Nanosleep,

		// File descriptors
		syscalls.Fcntl,
		syscalls.Ioctl,
		syscalls.Dup,
		syscalls.Dup3,
		syscalls.Pipe2,

		// Signals (needed by Go runtime)
		syscalls.Rt_sigaction,
		syscalls.Rt_sigprocmask,
		syscalls.Sigaltstack,

		// Futex (needed by Go runtime for goroutine synchronization)
		syscalls.Futex,
		syscalls.Set_robust_list,
		syscalls.Get_robust_list,

		// Epoll (needed by Go runtime)
		syscalls.Epoll_create1,
		syscalls.Epoll_ctl,
		syscalls.Epoll_pwait,
	}

	// Filter out -1 values (syscalls not available on this arch)
	filtered := make([]int, 0, len(safeSyscalls))
	for _, nr := range safeSyscalls {
		if nr != -1 {
			filtered = append(filtered, nr)
		}
	}

	return filtered
}
