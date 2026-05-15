# Code Review: options.go

**File:** `/Users/williamcory/codex/codex-go/pkg/sdk/options.go`
**Review Date:** 2025-10-26
**Lines of Code:** 64

---

## Executive Summary

The `options.go` file defines configuration structures for the SDK and session initialization. While the code is clean and well-documented, there are **significant issues** with validation, type safety, and completeness. The most critical concerns are:

1. **No validation** for string-based policy fields (ApprovalPolicy, SandboxPolicy)
2. **Missing test coverage** for the options structs themselves
3. **Type safety issues** with string enums
4. **Incomplete features** (history management not fully implemented)
5. **Documentation inconsistencies** with actual usage

---

## 1. Incomplete Features or Functionality

### 1.1 History Management (HIGH PRIORITY)

**Issue:** The history management feature is partially implemented but appears incomplete.

**Evidence:**
- Lines 22-29 define `EnableHistory` and `HistoryPath` options
- The SDK stores these values (sdk.go:92-93) but there's no evidence they're actually used
- No persistence implementation found in the SDK layer

**Impact:** Users may enable history thinking it persists, but it may not actually work.

**Recommendation:**
```go
// Add validation in sdk.New() to ensure history path is writable
if opts.EnableHistory {
    if opts.HistoryPath == "" {
        opts.HistoryPath = defaultHistoryPath() // e.g., ~/.codex/history.jsonl
    }
    // Validate path is writable
    if err := validateHistoryPath(opts.HistoryPath); err != nil {
        return nil, fmt.Errorf("invalid history path: %w", err)
    }
}
```

### 1.2 Session Resume Functionality (MEDIUM PRIORITY)

**Issue:** `SessionOptions.ConversationID` (line 62) suggests conversation resumption but no validation or loading logic.

**Evidence:**
- Field documented as "for resuming existing conversations"
- sdk.go:116-118 uses it as session ID generator, but doesn't load existing conversation
- No history loading mechanism

**Impact:** Setting a ConversationID creates a new session rather than resuming an old one.

**Recommendation:** Either implement resume functionality or update documentation to clarify this creates a session with a specific ID, not resume.

### 1.3 Model Override Not Implemented (MEDIUM PRIORITY)

**Issue:** `SessionOptions.Model` (line 58) is stored but never used to override the client's default model.

**Evidence:**
- Field stored in Session struct (sdk.go:110)
- Session methods (session.go:146-266) never reference model field
- No model selection logic in conversation flow

**Recommendation:** Implement model override in the actual API call or remove the field.

---

## 2. TODO Comments and Technical Debt

**STATUS:** No explicit TODO comments found in this file.

However, implicit technical debt exists:
- Validation logic is absent (should be added)
- Type safety improvements needed (see section 3)
- Test coverage missing (see section 4)

---

## 3. Code Quality Issues

### 3.1 String-Based Enums (CRITICAL)

**Issue:** Policy fields use unvalidated strings instead of typed constants.

**Problem Areas:**
```go
// Line 48: ApprovalPolicy string
ApprovalPolicy string // Values: "auto" (default), "always", "never"

// Line 52: SandboxPolicy string
SandboxPolicy string // Values: "read_only", "workspace_write", "full_access"
```

**Why This Is Bad:**
1. Typos cause silent failures ("alwyas" vs "always")
2. No IDE autocomplete
3. No compile-time safety
4. Documentation can drift from implementation

**Evidence of Inconsistency:**
- Documentation says: "auto", "always", "never" (line 47)
- Code actually uses: "auto", "always", "never", "manual", "semi-auto" (approval_handler.go:45-60)
- README says: "always" but tests use "manual"

**Recommendation:**
```go
// Define typed constants
type ApprovalPolicy string

const (
    ApprovalPolicyAuto      ApprovalPolicy = "auto"
    ApprovalPolicyManual    ApprovalPolicy = "manual"
    ApprovalPolicySemiAuto  ApprovalPolicy = "semi-auto"
    ApprovalPolicyNever     ApprovalPolicy = "never"
)

type SandboxPolicy string

const (
    SandboxPolicyOff              SandboxPolicy = "off"
    SandboxPolicyReadOnly         SandboxPolicy = "read-only"
    SandboxPolicyWorkspaceWrite   SandboxPolicy = "workspace-write"
    SandboxPolicyNative           SandboxPolicy = "native"
    SandboxPolicyDangerFullAccess SandboxPolicy = "danger-full-access"
)

// Update SessionOptions to use these types
type SessionOptions struct {
    // ...
    ApprovalPolicy ApprovalPolicy
    SandboxPolicy  SandboxPolicy
    // ...
}
```

### 3.2 Conflicting Tool Configuration (HIGH PRIORITY)

**Issue:** Lines 14-20 create an ambiguous tool configuration pattern.

```go
// Tools is the list of tools available to the agent.
// If nil, a default set of tools will be registered.
Tools []runtime.ToolRuntime

// ToolRegistry allows providing a pre-configured tool registry.
// If provided, Tools will be ignored.
ToolRegistry *runtime.ToolRegistry
```

**Problems:**
1. "Tools will be ignored" is surprising behavior
2. No validation prevents setting both
3. Silent precedence is error-prone

**Better Design:**
```go
type ToolConfig struct {
    // Only one of these should be set
    Tools        []runtime.ToolRuntime  // For simple cases
    Registry     *runtime.ToolRegistry  // For advanced cases
    UseDefaults  bool                   // Explicit default selection
}

// In validation:
func (tc ToolConfig) Validate() error {
    setCount := 0
    if len(tc.Tools) > 0 { setCount++ }
    if tc.Registry != nil { setCount++ }
    if tc.UseDefaults { setCount++ }

    if setCount > 1 {
        return fmt.Errorf("only one tool configuration method can be set")
    }
    return nil
}
```

### 3.3 Missing Validation Functions (CRITICAL)

**Issue:** Neither `Options` nor `SessionOptions` has a `Validate()` method.

**Current Behavior:**
- Validation happens in `sdk.New()` (only checks if Client is nil)
- No validation for SessionOptions at all
- Invalid configurations are silently accepted

**Recommendation:**
```go
// Validate checks if the Options are valid
func (o Options) Validate() error {
    if o.Client == nil {
        return fmt.Errorf("client is required")
    }

    // Validate tool configuration
    if o.ToolRegistry != nil && len(o.Tools) > 0 {
        return fmt.Errorf("cannot specify both ToolRegistry and Tools")
    }

    // Validate history configuration
    if o.EnableHistory && o.HistoryPath != "" {
        if !filepath.IsAbs(o.HistoryPath) {
            return fmt.Errorf("HistoryPath must be absolute, got: %s", o.HistoryPath)
        }
    }

    return nil
}

// Validate checks if SessionOptions are valid
func (so SessionOptions) Validate() error {
    // Validate ApprovalPolicy
    validApproval := map[ApprovalPolicy]bool{
        ApprovalPolicyAuto:     true,
        ApprovalPolicyManual:   true,
        ApprovalPolicySemiAuto: true,
        ApprovalPolicyNever:    true,
        "": true, // empty means use default
    }
    if !validApproval[so.ApprovalPolicy] {
        return fmt.Errorf("invalid ApprovalPolicy: %s", so.ApprovalPolicy)
    }

    // Validate SandboxPolicy
    validSandbox := map[SandboxPolicy]bool{
        SandboxPolicyOff:              true,
        SandboxPolicyReadOnly:         true,
        SandboxPolicyWorkspaceWrite:   true,
        SandboxPolicyNative:           true,
        SandboxPolicyDangerFullAccess: true,
        "": true, // empty means use default
    }
    if !validSandbox[so.SandboxPolicy] {
        return fmt.Errorf("invalid SandboxPolicy: %s", so.SandboxPolicy)
    }

    // Validate working directory if provided
    if so.WorkingDirectory != "" && !filepath.IsAbs(so.WorkingDirectory) {
        return fmt.Errorf("WorkingDirectory must be absolute, got: %s", so.WorkingDirectory)
    }

    return nil
}
```

### 3.4 OnToolApproval Callback Design (MEDIUM PRIORITY)

**Issue:** Line 44 callback signature is too simplistic.

```go
OnToolApproval func(toolName, operation string) bool
```

**Problems:**
1. Only returns bool (approve/deny), no way to express "ask me later" or "abort session"
2. Minimal context provided (no risk level, justification, or retry info)
3. Doesn't align with internal ApprovalRequest struct which has much richer data

**Better Design:**
```go
type ToolApprovalRequest struct {
    ToolName         string
    Operation        string
    Command          []string
    WorkingDirectory string
    Justification    string
    IsRetry          bool
    RetryReason      string
    RiskLevel        string   // "low", "medium", "high", "critical"
    RiskReasons      []string
}

type ToolApprovalDecision int

const (
    ToolApprovalApprove ToolApprovalDecision = iota
    ToolApprovalApproveSession // Approve for entire session
    ToolApprovalDeny
    ToolApprovalAbort // Abort the entire session
)

OnToolApproval func(req ToolApprovalRequest) ToolApprovalDecision
```

---

## 4. Missing Test Coverage

**CRITICAL ISSUE:** No direct test file for options.go

**What's Missing:**
1. No `options_test.go` file
2. Options are only tested indirectly through sdk_test.go
3. No validation testing (because no validation exists)
4. No edge case testing for policy values

**Test Coverage Analysis:**

From `sdk_test.go`:
- Tests Options.Client validation (lines 28-32) ✓
- Tests with custom tools (lines 34-40) ✓
- Tests with EnableHistory (lines 42-49) ✓
- Does NOT test invalid configurations ✗
- Does NOT test ToolRegistry vs Tools conflict ✗
- Does NOT test HistoryPath validation ✗

From `session_test.go`:
- Tests basic SessionOptions usage ✓
- Tests ApprovalPolicy (line 115) but only happy path ✓
- Does NOT test invalid policy strings ✗
- Does NOT test WorkingDirectory validation ✗

**Recommended Test Cases:**

```go
func TestOptions_Validate(t *testing.T) {
    tests := []struct {
        name    string
        opts    Options
        wantErr bool
        errMsg  string
    }{
        {
            name: "missing client",
            opts: Options{},
            wantErr: true,
            errMsg: "client is required",
        },
        {
            name: "both Tools and ToolRegistry",
            opts: Options{
                Client: &client.Client{},
                Tools: []runtime.ToolRuntime{mockTool{}},
                ToolRegistry: &runtime.ToolRegistry{},
            },
            wantErr: true,
            errMsg: "cannot specify both",
        },
        {
            name: "relative history path",
            opts: Options{
                Client: &client.Client{},
                EnableHistory: true,
                HistoryPath: "relative/path",
            },
            wantErr: true,
            errMsg: "must be absolute",
        },
        {
            name: "history enabled but path is directory",
            opts: Options{
                Client: &client.Client{},
                EnableHistory: true,
                HistoryPath: "/tmp",
            },
            wantErr: true,
            errMsg: "must be a file path",
        },
    }
    // ... run tests
}

func TestSessionOptions_Validate(t *testing.T) {
    tests := []struct {
        name    string
        opts    SessionOptions
        wantErr bool
        errMsg  string
    }{
        {
            name: "invalid approval policy",
            opts: SessionOptions{
                ApprovalPolicy: "invalid",
            },
            wantErr: true,
            errMsg: "invalid ApprovalPolicy",
        },
        {
            name: "invalid sandbox policy",
            opts: SessionOptions{
                SandboxPolicy: "invalid",
            },
            wantErr: true,
            errMsg: "invalid SandboxPolicy",
        },
        {
            name: "typo in approval policy",
            opts: SessionOptions{
                ApprovalPolicy: "alwyas", // typo
            },
            wantErr: true,
        },
        {
            name: "relative working directory",
            opts: SessionOptions{
                WorkingDirectory: "relative/path",
            },
            wantErr: true,
            errMsg: "must be absolute",
        },
        {
            name: "valid manual policy",
            opts: SessionOptions{
                ApprovalPolicy: "manual",
            },
            wantErr: false,
        },
    }
    // ... run tests
}
```

---

## 5. Potential Bugs and Edge Cases

### 5.1 Default Value Handling (HIGH PRIORITY)

**Issue:** Empty string policies have unclear semantics.

**Problem:**
```go
session, _ := codex.NewSession(ctx, SessionOptions{
    // ApprovalPolicy not set - what's the default?
})
```

**Current Behavior:**
- Empty string is passed to internal systems
- approval_handler.go treats empty string as NOT matching any policy
- Falls through to "manual" behavior by default

**Recommendation:**
```go
// In sdk.go NewSession():
func (s *SDK) NewSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // Apply defaults
    if opts.ApprovalPolicy == "" {
        opts.ApprovalPolicy = "auto" // Make default explicit
    }
    if opts.SandboxPolicy == "" {
        opts.SandboxPolicy = "native" // Make default explicit
    }

    // Validate after defaults applied
    if err := opts.Validate(); err != nil {
        return nil, fmt.Errorf("invalid session options: %w", err)
    }

    // Continue with session creation...
}
```

### 5.2 Nil OnToolApproval with "always" Policy (MEDIUM PRIORITY)

**Issue:** Line 42-43 comment is misleading.

```go
// OnToolApproval is called when a tool requires user approval.
// If nil, tools will be auto-approved based on the approval policy.
OnToolApproval func(toolName, operation string) bool
```

**Problem:** If `ApprovalPolicy = "always"` but `OnToolApproval = nil`, what happens?

**Current Behavior Analysis:**
- approval_handler.go:45 checks policy first
- If policy != "auto", it needs approval
- But if OnToolApproval is nil, there's no way to get approval
- Likely causes timeout or error

**Recommendation:**
```go
// In Validate():
if so.ApprovalPolicy == "manual" && so.OnToolApproval == nil {
    return fmt.Errorf("OnToolApproval callback required when ApprovalPolicy is 'manual'")
}
```

### 5.3 WorkingDirectory Not Validated (HIGH PRIORITY)

**Issue:** Line 55 WorkingDirectory is passed without validation.

**Potential Bugs:**
1. Non-existent directory
2. Relative path (ambiguous base)
3. Permission issues
4. Not a directory (e.g., file path)

**Recommendation:**
```go
// In NewSession() before creating session:
if opts.WorkingDirectory != "" {
    // Validate it exists
    if info, err := os.Stat(opts.WorkingDirectory); err != nil {
        return nil, fmt.Errorf("invalid working directory: %w", err)
    } else if !info.IsDir() {
        return nil, fmt.Errorf("working directory is not a directory: %s", opts.WorkingDirectory)
    }

    // Ensure it's absolute
    if !filepath.IsAbs(opts.WorkingDirectory) {
        return nil, fmt.Errorf("working directory must be absolute: %s", opts.WorkingDirectory)
    }
}
```

### 5.4 Model Override Edge Case (LOW PRIORITY)

**Issue:** Line 58 Model field not validated.

**Edge Cases:**
- Empty string (should use default or error?)
- Invalid model name (no validation)
- Model not supported by client

**Recommendation:** Add validation or document that model names aren't validated until API call.

---

## 6. Documentation Issues

### 6.1 Comment Inaccuracies (MEDIUM PRIORITY)

**Issue 1: Line 47 - Incomplete policy list**
```go
// ApprovalPolicy controls when tools require approval.
// Values: "auto" (default), "always", "never"
ApprovalPolicy string
```

**Actual Values Used:**
- "auto" ✓
- "always" ✗ (not used in codebase)
- "never" ✓
- "manual" (MISSING from docs)
- "semi-auto" (MISSING from docs)

**Issue 2: Line 51 - Wrong policy values**
```go
// SandboxPolicy controls tool sandboxing.
// Values: "read_only", "workspace_write", "full_access"
SandboxPolicy string
```

**Actual Values Used:**
- "off"
- "read-only" (not "read_only")
- "workspace-write" (not "workspace_write")
- "native"
- "danger-full-access" (not "full_access")

**Issue 3: README Inconsistency**

README.md line 170 shows:
```go
ApprovalPolicy: "always",  // "auto", "always", or "never"
```

But "always" is not a valid value! Should be "manual".

README.md line 220 shows:
```go
SandboxPolicy: "read_only",  // Wrong: uses underscore
```

Actual protocol uses hyphens: "read-only"

**Recommendation:** Update all documentation to match actual implementation.

### 6.2 Missing GoDoc Examples (MEDIUM PRIORITY)

**Issue:** No examples for complex configurations.

**Missing Examples:**
1. Using custom ToolRegistry
2. Setting up approval callbacks
3. Configuring sandbox policies
4. Resuming conversations

**Recommendation:**
```go
// Example_customTools shows how to register custom tools.
func Example_customTools() {
    registry := runtime.NewToolRegistry()
    registry.Register(myCustomTool)

    codex, _ := sdk.New(sdk.Options{
        Client: client,
        ToolRegistry: registry,
    })
    // ...
}

// Example_toolApproval shows how to handle tool approvals.
func Example_toolApproval() {
    session, _ := codex.NewSession(ctx, sdk.SessionOptions{
        ApprovalPolicy: sdk.ApprovalPolicyManual,
        OnToolApproval: func(req sdk.ToolApprovalRequest) sdk.ToolApprovalDecision {
            // Check risk level
            if req.RiskLevel == "critical" {
                return sdk.ToolApprovalDeny
            }
            // Prompt user
            return promptUser(req)
        },
    })
    // ...
}
```

### 6.3 Field Documentation Completeness (LOW PRIORITY)

**Good:**
- All fields have comments ✓
- Purpose is generally clear ✓

**Could Be Better:**
- Default values not documented
- Interaction between fields not explained
- No "see also" references to related types

**Example Improvement:**
```go
// ApprovalPolicy controls when tools require approval.
//
// Valid values:
//   - "auto" (default): Auto-approve known-safe operations
//   - "manual": Require approval for all tool calls
//   - "semi-auto": Require approval only for risky operations
//   - "never": Never request approval (may cause silent failures)
//
// When using "manual" or "semi-auto", you must provide an OnToolApproval
// callback, otherwise approval requests will timeout.
//
// See also: OnToolApproval, runtime.ApprovalRequest
ApprovalPolicy string
```

---

## 7. Security Concerns

### 7.1 Sandbox Policy Bypass (HIGH SEVERITY)

**Issue:** SandboxPolicy field has no validation, allowing dangerous values.

**Attack Scenario:**
```go
// Malicious or accidental bypass
session, _ := codex.NewSession(ctx, SessionOptions{
    SandboxPolicy: "danger-full-access", // Oops, typo allowed!
})
```

**Current Behavior:** String is passed through unchecked, interpreted by internal layers.

**Risk:**
- Typo could result in default policy (possibly less restrictive)
- User might think they're sandboxed when they're not
- No audit trail of invalid values

**Recommendation:**
1. Use typed constants (see section 3.1)
2. Validate in Validate() method
3. Log security-relevant policy selections
4. Consider requiring explicit opt-in for "danger-full-access"

```go
// Require explicit acknowledgment for dangerous policies
type SessionOptions struct {
    // ...
    SandboxPolicy SandboxPolicy

    // DangerAcceptFullAccess must be true to use SandboxPolicyDangerFullAccess.
    // This is a safety check to prevent accidental use of unrestricted execution.
    DangerAcceptFullAccess bool
}

// In Validate():
if so.SandboxPolicy == SandboxPolicyDangerFullAccess && !so.DangerAcceptFullAccess {
    return fmt.Errorf("must set DangerAcceptFullAccess=true to use danger-full-access policy")
}
```

### 7.2 HistoryPath Directory Traversal (MEDIUM SEVERITY)

**Issue:** Line 29 HistoryPath is not validated for directory traversal.

**Attack Scenario:**
```go
sdk.New(sdk.Options{
    Client: client,
    EnableHistory: true,
    HistoryPath: "/etc/passwd", // Or "../../../etc/passwd"
})
```

**Risk:** If the SDK writes to HistoryPath without validation, it could:
1. Overwrite system files
2. Write to unexpected locations
3. Fill up system partitions

**Recommendation:**
```go
func validateHistoryPath(path string) error {
    // Must be absolute
    if !filepath.IsAbs(path) {
        return fmt.Errorf("history path must be absolute")
    }

    // Must not be a system directory
    systemDirs := []string{"/etc", "/bin", "/usr", "/var", "/sys", "/proc"}
    for _, sysDir := range systemDirs {
        if strings.HasPrefix(path, sysDir) {
            return fmt.Errorf("cannot use system directory for history: %s", path)
        }
    }

    // Parent directory must exist
    dir := filepath.Dir(path)
    if info, err := os.Stat(dir); err != nil {
        return fmt.Errorf("history directory does not exist: %s", dir)
    } else if !info.IsDir() {
        return fmt.Errorf("history path parent is not a directory: %s", dir)
    }

    // Check write permissions
    testFile := filepath.Join(dir, ".test_write")
    if f, err := os.Create(testFile); err != nil {
        return fmt.Errorf("history directory not writable: %w", err)
    } else {
        f.Close()
        os.Remove(testFile)
    }

    return nil
}
```

### 7.3 Working Directory Escape (MEDIUM SEVERITY)

**Issue:** WorkingDirectory not validated for safety.

**Attack Scenario:**
```go
SessionOptions{
    WorkingDirectory: "/",
    SandboxPolicy: "workspace-write", // Combined with root dir!
}
```

**Risk:** "workspace-write" policy at root directory = write to entire filesystem.

**Recommendation:**
```go
// Prevent using root or system directories as workspace
func validateWorkingDirectory(dir string) error {
    dangerous := []string{"/", "/etc", "/bin", "/usr", "/var", "/sys", "/proc", "/dev"}
    for _, d := range dangerous {
        if dir == d {
            return fmt.Errorf("cannot use %s as working directory", dir)
        }
    }
    return nil
}
```

### 7.4 Model Injection (LOW SEVERITY)

**Issue:** Model field is unvalidated string.

**Potential Risk:**
While unlikely to cause direct security issues, malformed model names could:
1. Cause unexpected API behavior
2. Be logged inappropriately
3. Contain injection payloads if model name is used in commands

**Recommendation:** Validate model name format (alphanumeric + hyphens only).

---

## Summary of Findings

### Critical Issues (Must Fix)
1. ✗ No validation for ApprovalPolicy and SandboxPolicy strings
2. ✗ Missing Validate() methods on both structs
3. ✗ No test coverage for options validation
4. ✗ Documentation lists wrong policy values
5. ✗ Security: Unvalidated HistoryPath and WorkingDirectory

### High Priority Issues (Should Fix)
1. ✗ History persistence appears incomplete
2. ✗ ConversationID resume functionality missing
3. ✗ ToolRegistry vs Tools conflict not validated
4. ✗ OnToolApproval callback too simplistic
5. ✗ Default value handling is implicit

### Medium Priority Issues (Nice to Have)
1. ✗ Model override not implemented
2. ✗ No GoDoc examples for complex scenarios
3. ✗ Approval policy "always" vs "manual" confusion

### Low Priority Issues (Polish)
1. ✗ Field documentation could be more detailed
2. ✗ Model field not validated

---

## Recommendations Priority Order

1. **Immediate (Pre-Release):**
   - Add typed constants for policy fields
   - Implement Validate() methods with comprehensive checks
   - Add security validations for paths
   - Fix documentation to match implementation
   - Add comprehensive test coverage

2. **Short Term (Next Sprint):**
   - Implement or remove history persistence
   - Clarify/implement ConversationID resume behavior
   - Improve OnToolApproval callback design
   - Add validation for ToolRegistry vs Tools conflict

3. **Long Term (Future Enhancement):**
   - Add GoDoc examples
   - Enhance field documentation
   - Consider more sophisticated approval policy system

---

## Code Quality Score

| Category | Score | Notes |
|----------|-------|-------|
| Functionality | 4/10 | Several incomplete features |
| Test Coverage | 3/10 | Indirect testing only, no validation tests |
| Documentation | 6/10 | Present but inaccurate |
| Type Safety | 3/10 | String enums everywhere |
| Security | 4/10 | Multiple unvalidated paths |
| Maintainability | 7/10 | Clean structure but validation missing |
| **Overall** | **4.5/10** | **Needs significant work before production** |

---

## Conclusion

The `options.go` file provides a clean, simple API surface but has significant gaps in implementation. The most critical issues are:

1. **Lack of validation** allowing invalid configurations to silently pass through
2. **Type safety** problems with string-based enums
3. **Security concerns** around unvalidated file paths
4. **Documentation inaccuracies** that will confuse users

These issues should be addressed before considering this SDK production-ready. The recommendations above provide concrete steps to resolve each concern.
