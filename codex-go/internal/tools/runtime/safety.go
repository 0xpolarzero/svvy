package runtime

import (
	"path/filepath"
	"strings"
)

// SafetyLevel categorizes the security risk of a command
type SafetyLevel int

const (
	// SafetyAlwaysSafe indicates commands that are read-only and always allowed
	SafetyAlwaysSafe SafetyLevel = iota

	// SafetyConditional indicates commands that are safe with certain arguments
	SafetyConditional

	// SafetyUnsafe indicates commands that always require approval
	SafetyUnsafe
)

// CommandSafetyAnalysis provides detailed safety assessment for a command
type CommandSafetyAnalysis struct {
	// Level indicates the overall safety classification
	Level SafetyLevel

	// RequiresApproval indicates whether user approval is needed
	RequiresApproval bool

	// Reasons lists specific safety concerns
	Reasons []string

	// RiskLevel provides risk assessment for approval events
	RiskLevel RiskLevel
}

// AnalyzeCommandSafety performs comprehensive safety analysis on a command
// with argument-aware validation and dangerous flag detection.
func AnalyzeCommandSafety(command []string, workingDir string) *CommandSafetyAnalysis {
	if len(command) == 0 {
		// Empty command array is a no-op, treat as safe
		return &CommandSafetyAnalysis{
			Level:            SafetyAlwaysSafe,
			RequiresApproval: false,
			Reasons:          []string{},
			RiskLevel:        RiskLow,
		}
	}

	// Parse shell-wrapped commands to extract actual commands
	programs := parseShellCommand(command)
	if len(programs) == 0 {
		// Empty command is essentially a no-op, treat as safe
		return &CommandSafetyAnalysis{
			Level:            SafetyAlwaysSafe,
			RequiresApproval: false,
			Reasons:          []string{},
			RiskLevel:        RiskLow,
		}
	}

	// Check for command chaining with dangerous operators
	// Note: We don't early-return here anymore because we need to analyze
	// each command to determine the actual risk level (e.g., rm vs rm -rf)

	// Analyze each command in the chain
	var analysis *CommandSafetyAnalysis
	var maxRiskLevel RiskLevel = RiskLow
	var allReasons []string

	// Extract the shell command string if it exists (for passing to individual analyzers)
	shellCmd := ""
	if len(command) >= 3 && (command[0] == "sh" || command[0] == "bash") && command[1] == "-c" {
		shellCmd = command[2]
	}

	for _, program := range programs {
		// For shell-wrapped commands, we need to analyze with the full shell context
		var cmdAnalysis *CommandSafetyAnalysis
		if shellCmd != "" {
			// Build a representative command array for this program
			cmdAnalysis = analyzeIndividualCommand(program, command, workingDir)
		} else {
			cmdAnalysis = analyzeIndividualCommand(program, command, workingDir)
		}

		// Track the highest risk level
		if cmdAnalysis.RiskLevel > maxRiskLevel {
			maxRiskLevel = cmdAnalysis.RiskLevel
		}

		// Collect all reasons
		allReasons = append(allReasons, cmdAnalysis.Reasons...)

		// If any command is unsafe, the whole chain is unsafe
		if cmdAnalysis.Level == SafetyUnsafe {
			analysis = cmdAnalysis
			break
		}

		// If any command is conditional and analysis is not set yet
		if cmdAnalysis.Level == SafetyConditional && analysis == nil {
			analysis = cmdAnalysis
		}

		// If all are safe so far
		if analysis == nil {
			analysis = cmdAnalysis
		}
	}

	// Update with accumulated data
	if analysis != nil {
		analysis.RiskLevel = maxRiskLevel
		if len(allReasons) > 0 {
			analysis.Reasons = allReasons
		}
	}

	return analysis
}

// analyzeIndividualCommand analyzes a single command with its arguments
func analyzeIndividualCommand(program string, fullCommand []string, workingDir string) *CommandSafetyAnalysis {
	// Extract the base program name without path
	baseProgram := filepath.Base(program)

	// Check if it's an always-safe command
	if isAlwaysSafeCommand(baseProgram) {
		return &CommandSafetyAnalysis{
			Level:            SafetyAlwaysSafe,
			RequiresApproval: false,
			Reasons:          []string{},
			RiskLevel:        RiskLow,
		}
	}

	// Check for conditionally safe commands (need argument inspection)
	if analysis := analyzeConditionalCommand(baseProgram, fullCommand, workingDir); analysis != nil {
		return analysis
	}

	// Check for always-unsafe commands
	if isAlwaysUnsafeCommand(baseProgram) {
		return &CommandSafetyAnalysis{
			Level:            SafetyUnsafe,
			RequiresApproval: true,
			Reasons:          []string{baseProgram + " requires approval"},
			RiskLevel:        RiskCritical,
		}
	}

	// Unknown commands default to unsafe with medium risk
	return &CommandSafetyAnalysis{
		Level:            SafetyUnsafe,
		RequiresApproval: true,
		Reasons:          []string{"unknown command: " + baseProgram},
		RiskLevel:        RiskMedium,
	}
}

// isAlwaysSafeCommand checks if a command is always safe (read-only)
func isAlwaysSafeCommand(cmd string) bool {
	safeCommands := map[string]bool{
		"ls":     true,
		"pwd":    true,
		"echo":   true,
		"cat":    true,
		"grep":   true,
		"which":  true,
		"type":   true,
		"head":   true,
		"tail":   true,
		"wc":     true,
		"date":   true,
		"whoami": true,
		"id":     true,
		"uname":  true,
		"file":   true,
		"stat":   true,
		"true":   true,
		"false":  true,
		"test":   true,
		"[":      true,
	}
	return safeCommands[cmd]
}

// isAlwaysUnsafeCommand checks if a command always requires approval
func isAlwaysUnsafeCommand(cmd string) bool {
	unsafeCommands := map[string]bool{
		"dd":        true,
		"mkfs":      true,
		"fdisk":     true,
		"parted":    true,
		"sudo":      true,
		"su":        true,
		"reboot":    true,
		"shutdown":  true,
		"halt":      true,
		"init":      true,
		"systemctl": true,
		"service":   true,
		"curl":      true,
		"wget":      true,
		"nc":        true,
		"netcat":    true,
		"telnet":    true,
		"ssh":       true,
		"scp":       true,
		"rsync":     true,
		"ftp":       true,
	}
	return unsafeCommands[cmd]
}

// analyzeConditionalCommand analyzes commands that are safe with certain arguments
func analyzeConditionalCommand(program string, fullCommand []string, workingDir string) *CommandSafetyAnalysis {
	// For shell-wrapped commands, extract the actual command and its arguments
	var cmdArgs []string
	if len(fullCommand) >= 3 && (fullCommand[0] == "sh" || fullCommand[0] == "bash") && fullCommand[1] == "-c" {
		// Parse the shell command to extract tokens for this specific program
		shellCmd := fullCommand[2]
		cmdArgs = extractCommandTokens(shellCmd, program)
	} else {
		// Direct command, use as-is
		cmdArgs = fullCommand
	}

	switch program {
	case "rm":
		return analyzeRmCommand(cmdArgs)
	case "chmod":
		return analyzeChmodCommand(cmdArgs)
	case "find":
		return analyzeFindCommand(cmdArgs)
	case "git":
		return analyzeGitCommand(cmdArgs)
	case "sed":
		return analyzeSedCommand(cmdArgs)
	case "rg":
		return analyzeRipgrepCommand(cmdArgs)
	case "mv", "cp":
		return analyzeMoveOrCopyCommand(program, cmdArgs, workingDir)
	default:
		return nil
	}
}

// extractCommandTokens extracts the command and its arguments from a shell string
// for a specific program. For example, from "ls && rm -rf /" it would extract
// ["rm", "-rf", "/"] for program "rm".
func extractCommandTokens(shellCmd string, targetProgram string) []string {
	var tokens []string
	var currentToken []rune
	var inQuote bool
	var quoteChar rune
	var escaped bool
	var foundProgram bool

	// Helper to flush current token
	flushToken := func() {
		if len(currentToken) > 0 {
			token := string(currentToken)
			if !foundProgram && token == targetProgram {
				foundProgram = true
				tokens = append(tokens, token)
			} else if foundProgram {
				tokens = append(tokens, token)
			}
			currentToken = nil
		}
	}

	runes := []rune(shellCmd)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Handle escape sequences
		if escaped {
			currentToken = append(currentToken, ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inQuote {
			escaped = true
			continue
		}

		// Handle quotes
		if (ch == '\'' || ch == '"') && !inQuote {
			inQuote = true
			quoteChar = ch
			continue
		}
		if inQuote && ch == quoteChar {
			inQuote = false
			quoteChar = 0
			continue
		}

		// If inside quotes, accumulate
		if inQuote {
			currentToken = append(currentToken, ch)
			continue
		}

		// Check for command separators - stop if we found our program
		if foundProgram && (ch == ';' || ch == '|' || ch == '&') {
			flushToken()
			break
		}

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' {
			flushToken()
			continue
		}

		// Check for operators that end the current command
		if ch == '&' && i+1 < len(runes) && runes[i+1] == '&' {
			flushToken()
			if foundProgram {
				break
			}
			i++ // Skip next &
			continue
		}

		if ch == '|' {
			flushToken()
			if foundProgram {
				break
			}
			// Check if it's ||
			if i+1 < len(runes) && runes[i+1] == '|' {
				i++ // Skip next |
			}
			continue
		}

		// Regular character
		currentToken = append(currentToken, ch)
	}

	// Flush any remaining token
	flushToken()

	return tokens
}

// analyzeRmCommand checks for dangerous rm flags
func analyzeRmCommand(command []string) *CommandSafetyAnalysis {
	hasRecursive := false
	hasForce := false

	for _, arg := range command {
		// Check for -rf, -fr, -r, -f flags
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "r") {
				hasRecursive = true
			}
			if strings.Contains(arg, "f") {
				hasForce = true
			}
		}

		// Check for long form flags
		if arg == "--recursive" || arg == "-R" {
			hasRecursive = true
		}
		if arg == "--force" {
			hasForce = true
		}
	}

	reasons := []string{}
	riskLevel := RiskMedium

	if hasRecursive && hasForce {
		reasons = append(reasons, "rm -rf detected (destructive operation)")
		riskLevel = RiskCritical
	} else if hasRecursive {
		reasons = append(reasons, "rm -r detected (recursive deletion)")
		riskLevel = RiskHigh
	} else if hasForce {
		reasons = append(reasons, "rm -f detected (forced deletion)")
		riskLevel = RiskHigh
	} else {
		reasons = append(reasons, "rm command (file deletion)")
		riskLevel = RiskMedium
	}

	return &CommandSafetyAnalysis{
		Level:            SafetyUnsafe,
		RequiresApproval: true,
		Reasons:          reasons,
		RiskLevel:        riskLevel,
	}
}

// analyzeChmodCommand checks for dangerous chmod flags
func analyzeChmodCommand(command []string) *CommandSafetyAnalysis {
	reasons := []string{}
	riskLevel := RiskMedium

	// Check for chmod 777 or other overly permissive modes
	for _, arg := range command {
		if arg == "777" || arg == "a+rwx" || arg == "ugo+rwx" {
			reasons = append(reasons, "chmod 777 detected (world-writable permissions)")
			riskLevel = RiskHigh
		} else if strings.HasPrefix(arg, "7") {
			reasons = append(reasons, "chmod with execute permissions detected")
			riskLevel = RiskMedium
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "chmod command (permission modification)")
	}

	return &CommandSafetyAnalysis{
		Level:            SafetyUnsafe,
		RequiresApproval: true,
		Reasons:          reasons,
		RiskLevel:        riskLevel,
	}
}

// analyzeFindCommand checks for dangerous find options
func analyzeFindCommand(command []string) *CommandSafetyAnalysis {
	dangerousOptions := []string{
		"-delete",
		"-exec",
		"-execdir",
		"-ok",
		"-okdir",
		"-fls",
		"-fprint",
		"-fprint0",
		"-fprintf",
	}

	reasons := []string{}
	riskLevel := RiskLow

	for _, arg := range command {
		for _, dangerous := range dangerousOptions {
			if arg == dangerous {
				reasons = append(reasons, "find "+dangerous+" detected")
				if dangerous == "-delete" {
					riskLevel = RiskHigh
				} else if strings.HasPrefix(dangerous, "-exec") || strings.HasPrefix(dangerous, "-ok") {
					riskLevel = RiskCritical
				} else {
					riskLevel = RiskMedium
				}
			}
		}
	}

	if len(reasons) > 0 {
		return &CommandSafetyAnalysis{
			Level:            SafetyUnsafe,
			RequiresApproval: true,
			Reasons:          reasons,
			RiskLevel:        riskLevel,
		}
	}

	// Safe find command
	return &CommandSafetyAnalysis{
		Level:            SafetyAlwaysSafe,
		RequiresApproval: false,
		Reasons:          []string{},
		RiskLevel:        RiskLow,
	}
}

// analyzeGitCommand checks for dangerous git operations
func analyzeGitCommand(command []string) *CommandSafetyAnalysis {
	if len(command) < 2 {
		return &CommandSafetyAnalysis{
			Level:            SafetyConditional,
			RequiresApproval: true,
			Reasons:          []string{"git command without subcommand"},
			RiskLevel:        RiskMedium,
		}
	}

	subcommand := command[1]

	// Safe git commands (read-only)
	safeGitCommands := map[string]bool{
		"status":     true,
		"log":        true,
		"diff":       true,
		"show":       true,
		"branch":     true,
		"remote":     true,
		"config":     true,
		"rev-parse":  true,
		"ls-files":   true,
		"ls-tree":    true,
		"cat-file":   true,
		"rev-list":   true,
		"describe":   true,
		"symbolic-ref": true,
	}

	if safeGitCommands[subcommand] {
		return &CommandSafetyAnalysis{
			Level:            SafetyAlwaysSafe,
			RequiresApproval: false,
			Reasons:          []string{},
			RiskLevel:        RiskLow,
		}
	}

	// Dangerous git commands
	dangerousGitCommands := map[string]RiskLevel{
		"reset":    RiskHigh,
		"clean":    RiskHigh,
		"rm":       RiskHigh,
		"push":     RiskMedium,
		"pull":     RiskMedium,
		"fetch":    RiskLow,
		"checkout": RiskMedium,
		"merge":    RiskMedium,
		"rebase":   RiskHigh,
		"commit":   RiskLow,
		"add":      RiskLow,
		"stash":    RiskMedium,
	}

	if risk, found := dangerousGitCommands[subcommand]; found {
		return &CommandSafetyAnalysis{
			Level:            SafetyUnsafe,
			RequiresApproval: true,
			Reasons:          []string{"git " + subcommand + " modifies repository"},
			RiskLevel:        risk,
		}
	}

	// Unknown git command
	return &CommandSafetyAnalysis{
		Level:            SafetyUnsafe,
		RequiresApproval: true,
		Reasons:          []string{"unknown git command: " + subcommand},
		RiskLevel:        RiskMedium,
	}
}

// analyzeSedCommand checks for safe sed usage
func analyzeSedCommand(command []string) *CommandSafetyAnalysis {
	// Special case: sed -n {N|M,N}p is safe (read-only print)
	if len(command) >= 3 && command[1] == "-n" {
		arg := command[2]
		if isValidSedNArg(arg) {
			return &CommandSafetyAnalysis{
				Level:            SafetyAlwaysSafe,
				RequiresApproval: false,
				Reasons:          []string{},
				RiskLevel:        RiskLow,
			}
		}
	}

	// Check for in-place editing
	hasInPlace := false
	for _, arg := range command {
		if arg == "-i" || strings.HasPrefix(arg, "-i.") || arg == "--in-place" {
			hasInPlace = true
			break
		}
	}

	if hasInPlace {
		return &CommandSafetyAnalysis{
			Level:            SafetyUnsafe,
			RequiresApproval: true,
			Reasons:          []string{"sed -i detected (in-place file modification)"},
			RiskLevel:        RiskMedium,
		}
	}

	// Other sed commands are conditional
	return &CommandSafetyAnalysis{
		Level:            SafetyConditional,
		RequiresApproval: true,
		Reasons:          []string{"sed command may modify output"},
		RiskLevel:        RiskLow,
	}
}

// analyzeRipgrepCommand checks for dangerous rg options
func analyzeRipgrepCommand(command []string) *CommandSafetyAnalysis {
	dangerousFlags := map[string]string{
		"--pre":          "executes arbitrary command",
		"--hostname-bin": "executes command for hostname",
		"--search-zip":   "calls external decompression tools",
		"-z":             "calls external decompression tools",
	}

	reasons := []string{}

	for _, arg := range command {
		for flag, reason := range dangerousFlags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				reasons = append(reasons, "rg "+flag+": "+reason)
			}
		}
	}

	if len(reasons) > 0 {
		return &CommandSafetyAnalysis{
			Level:            SafetyUnsafe,
			RequiresApproval: true,
			Reasons:          reasons,
			RiskLevel:        RiskMedium,
		}
	}

	// Safe ripgrep usage
	return &CommandSafetyAnalysis{
		Level:            SafetyAlwaysSafe,
		RequiresApproval: false,
		Reasons:          []string{},
		RiskLevel:        RiskLow,
	}
}

// analyzeMoveOrCopyCommand checks for path escaping
func analyzeMoveOrCopyCommand(cmd string, command []string, workingDir string) *CommandSafetyAnalysis {
	// Check if any paths escape the workspace
	escapesWorkspace := false

	for _, arg := range command {
		if strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, workingDir) {
			escapesWorkspace = true
			break
		}
		if strings.Contains(arg, "..") {
			escapesWorkspace = true
			break
		}
	}

	reasons := []string{cmd + " command (file operation)"}
	riskLevel := RiskLow

	if escapesWorkspace {
		reasons = append(reasons, "path may escape workspace")
		riskLevel = RiskMedium
	}

	return &CommandSafetyAnalysis{
		Level:            SafetyConditional,
		RequiresApproval: true,
		Reasons:          reasons,
		RiskLevel:        riskLevel,
	}
}

// isValidSedNArg checks if an argument matches /^(\d+,)?\d+p$/
func isValidSedNArg(arg string) bool {
	if !strings.HasSuffix(arg, "p") {
		return false
	}

	core := strings.TrimSuffix(arg, "p")
	parts := strings.Split(core, ",")

	if len(parts) > 2 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}

	return true
}

// hasCommandChaining checks if the command contains chaining operators
func hasCommandChaining(command []string) bool {
	if len(command) < 3 {
		return false
	}

	// Check if it's a shell-wrapped command
	if command[0] == "sh" || command[0] == "bash" {
		if len(command) >= 3 && command[1] == "-c" {
			shellCmd := command[2]
			// Check for operators outside of quotes
			inQuote := false
			quoteChar := rune(0)

			for i, ch := range shellCmd {
				if !inQuote {
					if ch == '"' || ch == '\'' {
						inQuote = true
						quoteChar = ch
					} else {
						// Check for operators
						if ch == '|' || ch == ';' {
							return true
						}
						if ch == '&' && i+1 < len(shellCmd) && shellCmd[i+1] == '&' {
							return true
						}
					}
				} else {
					if ch == quoteChar {
						inQuote = false
						quoteChar = 0
					}
				}
			}
		}
	}

	return false
}

// containsUnsafeCommand checks if any program in the list is unsafe
func containsUnsafeCommand(programs []string, fullCommand []string) bool {
	for _, program := range programs {
		baseProgram := filepath.Base(program)
		if isAlwaysUnsafeCommand(baseProgram) {
			return true
		}

		// Check conditionals that might be unsafe
		analysis := analyzeConditionalCommand(baseProgram, fullCommand, "")
		if analysis != nil && analysis.Level == SafetyUnsafe {
			return true
		}
	}
	return false
}

// ValidatePathSafety checks if a path is safe to access within the workspace
func ValidatePathSafety(path string, workingDir string) bool {
	// Resolve path relative to working directory
	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		absPath = filepath.Join(workingDir, path)
		absPath = filepath.Clean(absPath)
	}

	// Check if the resolved path is within the workspace
	// Use filepath.Clean to normalize workingDir as well
	cleanWorkingDir := filepath.Clean(workingDir)

	// Add trailing slash to prevent false positives like:
	// /workspace/project-evil starting with /workspace/project
	if !strings.HasSuffix(cleanWorkingDir, string(filepath.Separator)) {
		cleanWorkingDir += string(filepath.Separator)
	}

	if !strings.HasPrefix(absPath, cleanWorkingDir) {
		// Also check if it's exactly the working directory itself
		if absPath != filepath.Clean(workingDir) {
			return false
		}
	}

	return true
}

// CreateRiskAssessment creates a risk assessment for an approval request
func CreateRiskAssessment(analysis *CommandSafetyAnalysis) *RiskAssessment {
	if analysis == nil {
		return &RiskAssessment{
			Level:      RiskMedium,
			Reasons:    []string{"unable to assess command safety"},
			Mitigation: "Sandbox restrictions will limit potential damage",
		}
	}

	mitigation := "Sandbox restrictions will limit potential damage"
	switch analysis.RiskLevel {
	case RiskCritical:
		mitigation = "This operation cannot be safely sandboxed. Proceed with extreme caution."
	case RiskHigh:
		mitigation = "Sandbox will restrict filesystem and network access, but data loss may still occur."
	case RiskMedium:
		mitigation = "Sandbox will prevent access outside the workspace."
	case RiskLow:
		mitigation = "Operation is read-only or low-impact. Minimal risk."
	}

	return &RiskAssessment{
		Level:      analysis.RiskLevel,
		Reasons:    analysis.Reasons,
		Mitigation: mitigation,
	}
}
