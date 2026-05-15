package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/cmd/codex/tui"
	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/openai"
	"github.com/evmts/codex/codex-go/internal/conversation/manager"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools"
	"github.com/evmts/codex/codex-go/internal/tools/orchestrator"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

var (
	messageFlag = flag.String("message", "", "Send a message in non-interactive mode")
	messageFlagShort = flag.String("m", "", "Send a message in non-interactive mode (shorthand)")
	sessionFlag = flag.String("session", "", "Session ID to use (optional, generates new one if not specified)")
	sessionFlagShort = flag.String("s", "", "Session ID to use (shorthand)")
	modelFlag   = flag.String("model", "", "Model to use (overrides MODEL env var)")
	approvalPolicyFlag = flag.String("approval-policy", "", "Approval policy: manual (default), semi-auto, auto, never")
	autoApproveFlag = flag.Bool("auto-approve", false, "DANGEROUS: Auto-approve all tool executions without prompting (equivalent to --approval-policy=auto)")
)

func main() {
	flag.Parse()

	// Determine message and session from flags
	message := *messageFlag
	if message == "" {
		message = *messageFlagShort
	}
	session := *sessionFlag
	if session == "" {
		session = *sessionFlagShort
	}
	model := *modelFlag

	// Determine approval policy
	approvalPolicy := determineApprovalPolicy()

	// Show security warning if auto-approve is enabled
	if approvalPolicy == "auto" {
		showAutoApproveWarning()
	}

	// Create manager with approval policy
	mgr, err := createManager(approvalPolicy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing manager: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	// Check if non-interactive mode
	if message != "" {
		// Non-interactive mode
		if err := runNonInteractive(mgr, message, session, model, approvalPolicy); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Interactive TUI mode
		if err := tui.Run(mgr); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	}
}

// determineApprovalPolicy determines the approval policy from flags.
func determineApprovalPolicy() string {
	// --auto-approve flag takes precedence
	if *autoApproveFlag {
		return "auto"
	}

	// Use --approval-policy if specified
	if *approvalPolicyFlag != "" {
		policy := strings.ToLower(*approvalPolicyFlag)
		switch policy {
		case "manual", "semi-auto", "auto", "never":
			return policy
		default:
			fmt.Fprintf(os.Stderr, "Warning: Invalid approval policy '%s', using 'manual'\n", policy)
			return "manual"
		}
	}

	// Default to manual approval (secure by default)
	return "manual"
}

// showAutoApproveWarning displays a security warning when auto-approve is enabled.
func showAutoApproveWarning() {
	warning := `
╔══════════════════════════════════════════════════════════════════════════════╗
║                            ⚠️  SECURITY WARNING ⚠️                            ║
╠══════════════════════════════════════════════════════════════════════════════╣
║ AUTO-APPROVE MODE IS ENABLED                                                 ║
║                                                                              ║
║ The AI will execute commands WITHOUT asking for your permission, including: ║
║   • Shell commands that can modify or delete files                          ║
║   • System operations that can affect your environment                      ║
║   • Network requests that can access external resources                     ║
║   • Any other operations the AI decides are necessary                       ║
║                                                                              ║
║ ONLY USE THIS MODE IF:                                                      ║
║   • You fully trust the AI model and prompts                                ║
║   • You are in a sandboxed/isolated environment                             ║
║   • You understand and accept the security risks                            ║
║                                                                              ║
║ For safer operation, use --approval-policy=manual (default)                 ║
╚══════════════════════════════════════════════════════════════════════════════╝
`
	fmt.Fprint(os.Stderr, warning)

	// Give user a chance to abort
	fmt.Fprintf(os.Stderr, "\nPress Enter to continue or Ctrl+C to abort...")
	fmt.Scanln()
}

// cliApprovalHandler creates an interactive CLI approval handler for non-interactive mode.
// It prompts the user on stderr and reads responses from stdin.
func cliApprovalHandler(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
	// Display approval request
	fmt.Fprintln(os.Stderr, "\n" + strings.Repeat("=", 80))
	fmt.Fprintln(os.Stderr, "⚠️  APPROVAL REQUIRED")
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 80))

	if req.IsRetry {
		fmt.Fprintf(os.Stderr, "Retry Reason: %s\n\n", req.RetryReason)
	}

	fmt.Fprintf(os.Stderr, "Tool: %s\n", req.ToolName)

	if len(req.Command) > 0 {
		fmt.Fprintf(os.Stderr, "Command: %s\n", strings.Join(req.Command, " "))
	}

	if req.WorkingDirectory != "" {
		fmt.Fprintf(os.Stderr, "Working Directory: %s\n", req.WorkingDirectory)
	}

	if req.Justification != "" {
		fmt.Fprintf(os.Stderr, "Justification: %s\n", req.Justification)
	}

	// Display risk assessment if available
	if req.Risk != nil {
		fmt.Fprintln(os.Stderr, "")
		riskLevelStr := "UNKNOWN"
		switch req.Risk.Level {
		case runtime.RiskLow:
			riskLevelStr = "LOW"
		case runtime.RiskMedium:
			riskLevelStr = "MEDIUM"
		case runtime.RiskHigh:
			riskLevelStr = "HIGH"
		case runtime.RiskCritical:
			riskLevelStr = "CRITICAL"
		}
		fmt.Fprintf(os.Stderr, "Risk Level: %s\n", riskLevelStr)

		if len(req.Risk.Reasons) > 0 {
			fmt.Fprintln(os.Stderr, "Risk Reasons:")
			for _, reason := range req.Risk.Reasons {
				fmt.Fprintf(os.Stderr, "  • %s\n", reason)
			}
		}

		if req.Risk.Mitigation != "" {
			fmt.Fprintf(os.Stderr, "Mitigation: %s\n", req.Risk.Mitigation)
		}
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 80))
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  [y] Approve this operation")
	fmt.Fprintln(os.Stderr, "  [a] Approve this and all similar operations for this session")
	fmt.Fprintln(os.Stderr, "  [n] Deny this operation")
	fmt.Fprintln(os.Stderr, "  [q] Abort the entire task")
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 80))

	// Read user response with timeout
	type response struct {
		decision runtime.ApprovalDecision
		err      error
	}
	respChan := make(chan response, 1)

	go func() {
		for {
			fmt.Fprint(os.Stderr, "\nYour choice [y/a/n/q]: ")
			var input string
			_, err := fmt.Scanln(&input)
			if err != nil {
				respChan <- response{runtime.ApprovalDenied, fmt.Errorf("failed to read input: %w", err)}
				return
			}

			input = strings.ToLower(strings.TrimSpace(input))
			switch input {
			case "y", "yes":
				respChan <- response{runtime.ApprovalApproved, nil}
				return
			case "a", "always", "all":
				respChan <- response{runtime.ApprovalApprovedForSession, nil}
				return
			case "n", "no", "deny":
				respChan <- response{runtime.ApprovalDenied, nil}
				return
			case "q", "quit", "abort":
				respChan <- response{runtime.ApprovalAbort, nil}
				return
			default:
				fmt.Fprintln(os.Stderr, "Invalid choice. Please enter y, a, n, or q.")
			}
		}
	}()

	// Wait for response or context cancellation
	select {
	case <-ctx.Done():
		return runtime.ApprovalDenied, ctx.Err()
	case resp := <-respChan:
		if resp.err != nil {
			return runtime.ApprovalDenied, resp.err
		}

		// Log the decision
		decisionStr := "UNKNOWN"
		switch resp.decision {
		case runtime.ApprovalApproved:
			decisionStr = "APPROVED"
		case runtime.ApprovalApprovedForSession:
			decisionStr = "APPROVED FOR SESSION"
		case runtime.ApprovalDenied:
			decisionStr = "DENIED"
		case runtime.ApprovalAbort:
			decisionStr = "ABORTED"
		}
		fmt.Fprintf(os.Stderr, "\nDecision: %s\n", decisionStr)
		fmt.Fprintln(os.Stderr, strings.Repeat("=", 80) + "\n")

		return resp.decision, nil
	}
}

// createManager creates a conversation manager with the specified approval policy.
// The approval policy determines how tool executions are authorized:
//   - "manual": Always prompt for approval (secure default)
//   - "semi-auto": Auto-approve safe operations, prompt for risky ones
//   - "auto": Auto-approve all operations (DANGEROUS - use only in trusted environments)
//   - "never": Never prompt, deny all operations (for read-only mode)
func createManager(approvalPolicy string) (manager.ConversationManager, error) {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// Also try ANTHROPIC_API_KEY for Claude models
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key required: set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable")
	}

	// Get model from environment or use default
	model := os.Getenv("MODEL")
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	// Get base URL from environment or use default
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		// Default to Anthropic API
		baseURL = "https://api.anthropic.com/v1"
	}

	// Create OpenAI-compatible client
	clientCfg := client.ClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}

	llmClient, err := openai.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create tool registry with all standard tools
	registry := tools.NewDefaultRegistry()

	// Create approval cache
	approvalCache := runtime.NewMemoryApprovalCache()

	// Create appropriate approval handler based on policy
	var approvalHandler orchestrator.ApprovalHandler
	switch approvalPolicy {
	case "auto":
		// Auto-approve everything (DANGEROUS - only for trusted environments)
		approvalHandler = func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
			// Log what we're auto-approving for audit trail
			if len(req.Command) > 0 {
				fmt.Fprintf(os.Stderr, "[AUTO-APPROVED] %s: %s\n", req.ToolName, strings.Join(req.Command, " "))
			} else {
				fmt.Fprintf(os.Stderr, "[AUTO-APPROVED] %s\n", req.ToolName)
			}
			return runtime.ApprovalApprovedForSession, nil
		}

	case "never":
		// Never prompt, always deny (for read-only mode)
		approvalHandler = func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
			return runtime.ApprovalDenied, fmt.Errorf("approval policy is 'never' - all operations are denied")
		}

	case "semi-auto":
		// Use a semi-automatic handler that auto-approves safe operations
		approvalHandler = func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
			// Auto-approve known safe commands
			if !req.IsRetry && len(req.Command) > 0 && runtime.IsKnownSafeCommand(req.Command) {
				fmt.Fprintf(os.Stderr, "[AUTO-APPROVED] Safe command: %s\n", strings.Join(req.Command, " "))
				return runtime.ApprovalApprovedForSession, nil
			}
			// For risky operations, use interactive prompt
			return cliApprovalHandler(ctx, req)
		}

	case "manual":
		fallthrough
	default:
		// Use interactive CLI approval handler (secure default)
		approvalHandler = cliApprovalHandler
	}

	orch := orchestrator.NewOrchestrator(registry, approvalCache, approvalHandler)

	// Create manager with orchestrator
	cfg := manager.ManagerConfig{
		Client:       llmClient,
		Orchestrator: orch,
	}
	return manager.NewManager(cfg)
}

// runNonInteractive runs a single message in non-interactive mode and streams the response to stdout
func runNonInteractive(mgr manager.ConversationManager, message, sessionID, model, approvalPolicy string) error {
	ctx := context.Background()

	// Use provided model or default
	if model == "" {
		model = os.Getenv("MODEL")
		if model == "" {
			model = "claude-3-5-sonnet-20241022"
		}
	}

	// Generate session ID if not provided
	if sessionID == "" {
		sessionID = fmt.Sprintf("cli-%d", time.Now().Unix())
	}

	// Create event handler for streaming output
	var streamingText strings.Builder
	done := make(chan struct{})
	hadError := false

	eventHandler := func(ctx context.Context, event *protocol.Event) error {
		switch msg := event.Msg.(type) {
		case *protocol.EventAgentMessageDelta:
			// Print streaming text immediately
			fmt.Print(msg.Delta)
			streamingText.WriteString(msg.Delta)

		case *protocol.EventExecCommandBegin:
			// Show tool execution
			fmt.Fprintf(os.Stderr, "\n[Executing: %s]\n", strings.Join(msg.Command, " "))

		case *protocol.EventExecCommandEnd:
			// Show tool completion
			if msg.ExitCode == 0 {
				fmt.Fprintf(os.Stderr, "[Command completed successfully]\n")
			} else {
				fmt.Fprintf(os.Stderr, "[Command failed with exit code %d]\n", msg.ExitCode)
			}

		case *protocol.EventTaskComplete:
			// Task complete
			fmt.Println() // Final newline
			close(done)

		case *protocol.EventError:
			// Error occurred
			fmt.Fprintf(os.Stderr, "\nError: %s\n", msg.Message)
			hadError = true
			close(done)
		}
		return nil
	}

	// Try to get existing session or create new one
	sess, err := mgr.GetSession(sessionID)
	if err != nil || sess == nil {
		// Get absolute path for current directory
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			cwd = "." // Fallback to relative path
		}

		// Create new session with event handler and configured approval policy
		sess, err = mgr.CreateSession(ctx, manager.SessionConfig{
			ID: sessionID,
			TurnContext: &manager.TurnContext{
				Cwd:            cwd,
				ApprovalPolicy: approvalPolicy,
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          model,
			},
			EventHandlers: []manager.EventHandler{eventHandler},
		})
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Submit message
	op := &protocol.OpUserInput{
		Items: []protocol.UserInput{
			{
				Type: "text",
				Text: &message,
			},
		},
	}

	err = mgr.SubmitOp(ctx, sessionID, op)
	if err != nil {
		return fmt.Errorf("failed to submit message: %w", err)
	}

	// Wait for completion with timeout
	select {
	case <-done:
		if hadError {
			return fmt.Errorf("turn processing failed")
		}
		return nil
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timeout waiting for response")
	}
}
