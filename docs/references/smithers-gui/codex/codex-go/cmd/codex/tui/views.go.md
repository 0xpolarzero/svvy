# Code Review: views.go

**File:** `/Users/williamcory/codex/codex-go/cmd/codex/tui/views.go`
**Review Date:** 2025-10-26
**Reviewer:** Claude Code
**Lines of Code:** 193

---

## Executive Summary

The `views.go` file implements the rendering layer for a Terminal User Interface (TUI) application. Overall, the code is well-structured with good security practices (ANSI sanitization), but it lacks several important features and has room for improvement in error handling, documentation, and test coverage.

**Critical Issues:** 0
**High Priority:** 3
**Medium Priority:** 6
**Low Priority:** 5

---

## 1. Incomplete Features & Functionality

### 1.1 Missing String Method for ViewMode Enum
**Priority:** Medium
**Location:** Lines 10-17

The `ViewMode` type is an enum (iota-based integer) but lacks a `String()` method for debugging and logging purposes. This makes debugging more difficult.

```go
type ViewMode int

const (
	ViewModeSessionList ViewMode = iota
	ViewModeConversation
	ViewModeToolApproval
)
```

**Recommendation:** Add a `String()` method:
```go
func (v ViewMode) String() string {
	switch v {
	case ViewModeSessionList:
		return "SessionList"
	case ViewModeConversation:
		return "Conversation"
	case ViewModeToolApproval:
		return "ToolApproval"
	default:
		return "Unknown"
	}
}
```

### 1.2 Limited Message Role Support
**Priority:** Medium
**Location:** Lines 112-125

The message rendering only handles "user", "assistant", and "system" roles with a generic fallback. Modern LLM applications may require additional roles like "function", "tool", "context", etc.

**Recommendation:** Consider expanding role support or document the intentionally limited scope.

### 1.3 No Message Metadata Support
**Priority:** Low
**Location:** Lines 189-193

The `Message` struct only contains `Role` and `Content`. Real-world conversations often need:
- Timestamps
- Token counts
- Message IDs
- Metadata (model version, temperature, etc.)

**Recommendation:** Expand the Message struct:
```go
type Message struct {
	ID        string    // Unique identifier
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    int       // Token count for this message
	Metadata  map[string]interface{}
}
```

### 1.4 No Viewport/Scrolling Support
**Priority:** High
**Location:** Lines 101-147 (RenderConversation)

The `RenderConversation` function renders all messages without any viewport management or scrolling. This will cause issues with:
- Long conversations exceeding terminal height
- Memory consumption for large message histories
- User experience (inability to scroll back)

**Recommendation:** Implement viewport functionality using `github.com/charmbracelet/bubbles/viewport` or custom scrolling logic.

### 1.5 Fixed Width Input Style
**Priority:** Medium
**Location:** Lines 64-67

The input style uses a fixed width which may not work well with responsive terminal sizing:

```go
inputStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
```

**Recommendation:** Make input style width-aware and pass width parameter from terminal size.

---

## 2. TODO Comments & Technical Debt

### 2.1 No Explicit TODOs Found
**Priority:** N/A

No TODO, FIXME, HACK, XXX, or BUG comments were found in the file. This is good practice, but cross-referencing with `app.go` shows several incomplete features:
- Message persistence (line 467-470 in app.go: "In a real implementation, load message history")
- Legacy streaming support (lines 687-691 in app.go marked as "Legacy")

**Recommendation:** Consider adding inline TODOs where functionality is intentionally incomplete to help future maintainers.

---

## 3. Code Quality Issues

### 3.1 Inconsistent Color Usage
**Priority:** Low
**Location:** Lines 19-72

Colors are hard-coded as magic numbers (e.g., "39", "170", "82") without explanation. This makes:
- Color scheme changes difficult
- Understanding the visual hierarchy unclear
- Color accessibility issues hard to detect

**Recommendation:** Create named color constants with semantic names:
```go
const (
	ColorPrimary    = "39"  // Blue
	ColorAccent     = "170" // Purple
	ColorSuccess    = "82"  // Green
	ColorError      = "196" // Red
	ColorMuted      = "241" // Gray
	ColorWarning    = "214" // Orange
	ColorHighlight  = "62"  // Cyan
)
```

### 3.2 No Validation of Parameters
**Priority:** High
**Location:** Lines 75-98, 101-147, 150-170, 173-176

None of the render functions validate their input parameters:
- `RenderSessionList` doesn't check if `selectedIdx` is within bounds
- `RenderConversation` doesn't validate `sessionID` isn't empty
- `RenderToolApproval` doesn't validate `toolParams` isn't nil
- `RenderStatusBar` doesn't handle negative token counts or widths

**Example Issue:**
```go
func RenderSessionList(sessions []string, selectedIdx int) string {
	// No check if selectedIdx < 0 or >= len(sessions)
	// This could cause panics in line 87-89
}
```

**Recommendation:** Add defensive programming:
```go
func RenderSessionList(sessions []string, selectedIdx int) string {
	var b strings.Builder

	// Validate selectedIdx
	if selectedIdx < 0 {
		selectedIdx = 0
	}
	if selectedIdx >= len(sessions) && len(sessions) > 0 {
		selectedIdx = len(sessions) - 1
	}

	// ... rest of function
}
```

### 3.3 Hardcoded UI Constants
**Priority:** Medium
**Location:** Lines 82, 89, 91, 115, 118, 138

Magic strings and formatting are hardcoded throughout:
- "No sessions yet. Press 'n' to create a new session."
- Cursor symbol "▌"
- Prefix symbols "▸" and "  "

**Recommendation:** Extract to constants:
```go
const (
	CursorSymbol       = "▌"
	SelectedPrefix     = "▸ "
	UnselectedPrefix   = "  "
	NoSessionsMessage  = "No sessions yet. Press 'n' to create a new session."
)
```

### 3.4 String Concatenation in Loops
**Priority:** Low
**Location:** Lines 85-94

The function uses `strings.Builder` correctly, but the pattern could be more efficient by pre-allocating capacity when the size is known.

**Recommendation:**
```go
func RenderSessionList(sessions []string, selectedIdx int) string {
	// Pre-allocate approximate capacity
	var b strings.Builder
	b.Grow(len(sessions) * 50) // Approximate bytes per session
	// ... rest of function
}
```

### 3.5 Inconsistent Error Handling
**Priority:** Medium
**Location:** Lines 184-186

The `RenderError` function takes an `error` but doesn't check for nil:

```go
func RenderError(err error) string {
	return errorStyle.Render(fmt.Sprintf("Error: %v", err))
}
```

**Recommendation:**
```go
func RenderError(err error) string {
	if err == nil {
		return ""
	}
	return errorStyle.Render(fmt.Sprintf("Error: %v", err))
}
```

### 3.6 No Width/Height Responsive Rendering
**Priority:** High
**Location:** Lines 173-176

`RenderStatusBar` is the only function that accepts a width parameter, but other render functions don't account for terminal dimensions:

```go
func RenderStatusBar(model string, tokens int, mode string, width int) string {
	status := fmt.Sprintf(" Model: %s | Tokens: %d | Mode: %s ", model, tokens, mode)
	return statusBarStyle.Width(width).Render(status)
}
```

**Recommendation:** Add width/height parameters to all render functions for responsive design.

---

## 4. Missing Test Coverage

### 4.1 No Unit Tests for views.go
**Priority:** High
**Location:** Entire file

The file has **zero test coverage**. While `sanitize_test.go` exists with excellent coverage (563 lines of tests), `views.go` has no tests at all.

**Critical Missing Tests:**
1. `RenderSessionList` with various edge cases:
   - Empty sessions list
   - Single session
   - Selected index out of bounds
   - Very long session names

2. `RenderConversation` with:
   - Empty messages
   - Messages with special characters
   - Very long content
   - Streaming text edge cases
   - Empty sessionID

3. `RenderToolApproval` with:
   - Nil parameters map
   - Empty tool name
   - Nested parameter structures
   - Very long parameter values

4. `RenderStatusBar` with:
   - Negative widths
   - Zero width
   - Extremely large widths
   - Negative token counts

5. `RenderHelp` and `RenderError`:
   - Basic functionality tests
   - Nil error handling

**Recommendation:** Create `views_test.go` with comprehensive test coverage. Target at least 80% code coverage.

### 4.2 No Visual Regression Tests
**Priority:** Medium

TUI rendering is visual, and snapshot testing would help catch unintended changes.

**Recommendation:** Consider using golden file testing to capture expected output:
```go
func TestRenderSessionList_GoldenFiles(t *testing.T) {
	sessions := []string{"session-1", "session-2", "session-3"}
	output := RenderSessionList(sessions, 1)

	golden.Assert(t, output, "session_list_selected_2.golden")
}
```

---

## 5. Potential Bugs & Edge Cases

### 5.1 Index Out of Bounds Risk
**Priority:** High
**Location:** Lines 87-89

```go
if i == selectedIdx {
	style = selectedSessionStyle
	b.WriteString(style.Render(fmt.Sprintf("▸ %s", session)))
}
```

If `selectedIdx` is negative or greater than `len(sessions)`, this will either skip highlighting or cause undefined behavior.

**Severity:** High - Could cause runtime panics
**Likelihood:** Medium - Depends on caller validation

**Recommendation:** Add bounds checking at function entry.

### 5.2 Nil Pointer Dereference Risk
**Priority:** Medium
**Location:** Lines 159-164

```go
for key, value := range toolParams {
	// Sanitize both key and value to prevent injection
	sanitizedKey := SanitizeContent(key)
	sanitizedValue := SanitizeContent(fmt.Sprintf("%v", value))
	b.WriteString(fmt.Sprintf("  %s: %s\n", sanitizedKey, sanitizedValue))
}
```

If `toolParams` is nil, this will panic. The function should check for nil.

**Recommendation:**
```go
func RenderToolApproval(toolName string, toolParams map[string]interface{}, riskLevel string) string {
	var b strings.Builder

	b.WriteString("Tool Approval Required\n\n")
	b.WriteString(fmt.Sprintf("Tool: %s\n", SanitizeContent(toolName)))
	b.WriteString(fmt.Sprintf("Risk Level: %s\n\n", SanitizeContent(riskLevel)))

	if toolParams == nil || len(toolParams) == 0 {
		b.WriteString("Parameters: (none)\n")
	} else {
		b.WriteString("Parameters:\n")
		for key, value := range toolParams {
			sanitizedKey := SanitizeContent(key)
			sanitizedValue := SanitizeContent(fmt.Sprintf("%v", value))
			b.WriteString(fmt.Sprintf("  %s: %s\n", sanitizedKey, sanitizedValue))
		}
	}
	// ... rest of function
}
```

### 5.3 Unchecked Type Assertion for toolParams Values
**Priority:** Medium
**Location:** Line 162

```go
sanitizedValue := SanitizeContent(fmt.Sprintf("%v", value))
```

Using `%v` formatter with `interface{}` can produce unexpected output for complex types (structs, functions, channels). This could leak sensitive information or produce unhelpful output.

**Recommendation:** Add type-specific formatting:
```go
func formatToolParamValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int, int32, int64, uint, uint32, uint64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		return fmt.Sprintf("(array with %d items)", len(v))
	case map[string]interface{}:
		return fmt.Sprintf("(object with %d keys)", len(v))
	default:
		return fmt.Sprintf("(type: %T)", v)
	}
}
```

### 5.4 Race Condition in Streaming Text
**Priority:** Low
**Location:** Lines 134-140

The streaming text is rendered directly without any consideration of concurrent updates. While the Bubble Tea framework handles this through message passing, there's no documentation about thread safety.

**Recommendation:** Add comment about thread safety expectations.

### 5.5 Empty Session ID Handling
**Priority:** Medium
**Location:** Line 104

```go
b.WriteString(titleStyle.Render(fmt.Sprintf("Session: %s", sessionID)))
```

If `sessionID` is an empty string, the title will render as "Session: " which looks broken.

**Recommendation:**
```go
displayID := sessionID
if displayID == "" {
	displayID = "(no session)"
}
b.WriteString(titleStyle.Render(fmt.Sprintf("Session: %s", displayID)))
```

### 5.6 No Maximum Content Length
**Priority:** Medium
**Location:** Lines 128-129, 136-137

Messages and streaming text are rendered without length limits. Extremely long messages could:
- Cause performance issues
- Exceed terminal buffer limits
- Create poor UX

**Recommendation:** Add truncation with indicators:
```go
const MaxMessageLength = 10000

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "... (truncated)"
}
```

---

## 6. Documentation Issues

### 6.1 Missing Package Documentation
**Priority:** Medium
**Location:** Line 1

The file lacks a package-level comment explaining the purpose of the TUI package and the rendering architecture.

**Recommendation:**
```go
// Package tui implements the Terminal User Interface for Codex.
//
// The TUI uses the Bubble Tea framework for reactive state management
// and Lip Gloss for styling. This file contains view rendering functions
// that convert application state into styled terminal output.
//
// Security: All user-generated content is sanitized using SanitizeContent()
// to prevent ANSI injection attacks.
package tui
```

### 6.2 Insufficient Function Documentation
**Priority:** Medium
**Location:** Lines 74, 100, 149, 172, 178, 183

Most functions have brief single-line comments, but they lack:
- Parameter descriptions
- Return value descriptions
- Example usage
- Thread safety notes
- Performance considerations

**Example of current documentation:**
```go
// RenderSessionList renders the session list view
func RenderSessionList(sessions []string, selectedIdx int) string {
```

**Recommended improvement:**
```go
// RenderSessionList renders the session selection screen.
//
// Parameters:
//   - sessions: List of session IDs to display. Can be empty.
//   - selectedIdx: Zero-based index of the currently selected session.
//                  If out of bounds, no session will appear selected.
//
// Returns a styled string suitable for display in a Bubble Tea view.
// The output includes a title, session list with selection indicator,
// and a helpful message when the list is empty.
//
// Thread Safety: Safe for concurrent reads, but not safe for concurrent
// writes to the sessions slice.
func RenderSessionList(sessions []string, selectedIdx int) string {
```

### 6.3 No Examples
**Priority:** Low

The file contains no usage examples, making it harder for new developers to understand how to use these functions.

**Recommendation:** Add example tests:
```go
func ExampleRenderSessionList() {
	sessions := []string{"session-1", "session-2"}
	output := RenderSessionList(sessions, 0)
	fmt.Println(output)
}
```

### 6.4 Missing Style Documentation
**Priority:** Low
**Location:** Lines 19-72

The style variables lack documentation explaining their visual appearance and usage context.

**Recommendation:**
```go
// titleStyle is used for main headings (large, bold, blue)
var titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")). // Blue
		MarginBottom(1)

// sessionItemStyle is used for unselected session items (indented)
var sessionItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)
```

### 6.5 No Architecture Documentation
**Priority:** Medium

The relationship between `views.go`, `app.go`, and the Bubble Tea framework isn't documented. New developers need to understand:
- When these functions are called
- Who is responsible for state management
- How sanitization integrates with rendering

**Recommendation:** Add an architecture overview in the package documentation or a separate ARCHITECTURE.md file.

---

## 7. Security Concerns

### 7.1 Excellent ANSI Sanitization (Positive Finding)
**Priority:** N/A
**Location:** Lines 128, 136, 155-156, 161-162

The code properly sanitizes all user-generated content before rendering:

```go
sanitizedContent := SanitizeContent(msg.Content)
sanitizedStreamingText := SanitizeContent(streamingText)
sanitizedKey := SanitizeContent(key)
sanitizedValue := SanitizeContent(fmt.Sprintf("%v", value))
```

This is **excellent security practice** and prevents terminal injection attacks. The sanitization implementation in `sanitize.go` is comprehensive and well-tested (563 lines of tests).

**Positive Review:** The security implementation is exemplary.

### 7.2 No Session ID Validation
**Priority:** Low
**Location:** Line 104

Session IDs are rendered without validation. While not a direct security issue, malicious session IDs could contain:
- Extremely long strings (DoS)
- Special characters that break display
- Homograph attacks (similar-looking characters)

**Recommendation:** Add basic validation:
```go
func sanitizeSessionID(id string) string {
	// Limit length
	if len(id) > 100 {
		id = id[:100] + "..."
	}
	// Sanitize ANSI
	id = SanitizeContent(id)
	return id
}
```

### 7.3 Tool Parameter Exposure
**Priority:** Low
**Location:** Lines 159-164

Tool parameters are displayed directly in the approval panel. While sanitized, sensitive information (passwords, tokens, API keys) could be visible.

**Recommendation:** Add parameter filtering:
```go
var sensitiveKeys = map[string]bool{
	"password": true,
	"token": true,
	"api_key": true,
	"secret": true,
}

func sanitizeToolParamKey(key string) string {
	lowerKey := strings.ToLower(key)
	for sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return key + " (redacted)"
		}
	}
	return key
}
```

### 7.4 Information Disclosure via Status Bar
**Priority:** Low
**Location:** Lines 173-176

The status bar displays the model name and token count. In multi-user environments, this could leak:
- Which AI model is being used
- Usage patterns/token consumption
- Session activity

**Recommendation:** Make status bar information configurable based on privacy settings.

---

## 8. Performance Considerations

### 8.1 String Builder Usage (Positive Finding)
**Priority:** N/A

The code correctly uses `strings.Builder` for efficient string concatenation. This is good practice.

### 8.2 No Caching of Rendered Output
**Priority:** Low
**Location:** All render functions

Render functions compute output from scratch on every call. For static elements (help text, status bar with unchanging values), caching could improve performance.

**Recommendation:** Consider memoization for expensive or frequently-called renders:
```go
var cachedHelp string
var helpOnce sync.Once

func RenderHelp() string {
	helpOnce.Do(func() {
		cachedHelp = helpStyle.Render("Press 'q' to quit, 'n' for new session, 'a' to approve, 'd' to deny")
	})
	return cachedHelp
}
```

### 8.3 Regex Performance in Sanitization
**Priority:** Low
**Location:** Lines 128, 136, 155-156, 161-162

Every call to `SanitizeContent()` runs multiple regex patterns. For large messages or frequent updates, this could be a bottleneck.

**Recommendation:** Profile the sanitization performance and consider:
- Pre-checking with `ContainsANSI()` before sanitizing
- Caching sanitized versions of static content
- Lazy sanitization only when rendering

---

## 9. Maintainability Issues

### 9.1 Tight Coupling to lipgloss
**Priority:** Low

All rendering is tightly coupled to the `lipgloss` library. While this is likely intentional, it makes testing and alternative rendering backends difficult.

**Recommendation:** Consider adding an abstraction layer if you plan to support:
- Non-terminal rendering (HTML, Markdown)
- Testing without actual style computation
- Alternative styling libraries

### 9.2 No View State Separation
**Priority:** Medium
**Location:** All render functions

Render functions mix state handling and presentation. For example, `RenderConversation` handles both the message list iteration and styling.

**Recommendation:** Consider separating concerns:
```go
// ViewState holds pre-computed view data
type ConversationViewState struct {
	Title    string
	Messages []RenderedMessage
	Streaming *StreamingState
	Input    string
}

// RenderConversationState renders a pre-computed view state
func RenderConversationState(state ConversationViewState) string {
	// Pure rendering, no business logic
}
```

### 9.3 Magic Numbers Throughout
**Priority:** Low
**Location:** Lines 19-72

Style definitions use magic numbers for colors, padding, margins, etc. This makes global style changes difficult.

**Recommendation:** Create a theme configuration:
```go
type Theme struct {
	Colors struct {
		Primary   string
		Secondary string
		Success   string
		Error     string
		Muted     string
	}
	Spacing struct {
		Small  int
		Medium int
		Large  int
	}
}

var DefaultTheme = Theme{...}
```

---

## 10. Additional Recommendations

### 10.1 Accessibility Considerations
**Priority:** Medium

The TUI uses colors and symbols that may not be accessible to:
- Users with color blindness
- Screen reader users
- Users with certain terminal configurations

**Recommendations:**
1. Add option for high-contrast mode
2. Provide ASCII-only fallback for symbols (▸, ▌)
3. Document color meanings in text as well as color
4. Test with terminal accessibility tools

### 10.2 Internationalization (i18n)
**Priority:** Low

All strings are hardcoded in English. Future internationalization would require significant refactoring.

**Recommendation:** Extract strings to constants or a localization system:
```go
var strings = struct {
	NoSessions      string
	PressQuit       string
	ToolApproval    string
	// ... etc
}{
	NoSessions:   "No sessions yet. Press 'n' to create a new session.",
	PressQuit:    "Press 'q' to quit, 'n' for new session, 'a' to approve, 'd' to deny",
	ToolApproval: "Tool Approval Required",
}
```

### 10.3 Add Render Context
**Priority:** Medium

Pass a context object to render functions for better control:

```go
type RenderContext struct {
	Width      int
	Height     int
	Theme      *Theme
	DebugMode  bool
	Locale     string
}

func RenderSessionList(ctx RenderContext, sessions []string, selectedIdx int) string {
	// Use ctx.Width, ctx.Theme, etc.
}
```

### 10.4 Add Debug Mode
**Priority:** Low

Add a debug rendering mode that shows:
- Function timing
- String lengths
- Sanitization statistics
- Layout boxes

This would help with performance profiling and debugging layout issues.

---

## Summary of Priority Issues

### Critical (Immediate Action Required)
None identified, but high-priority items should be addressed soon.

### High Priority (Address Before Production)
1. **No Test Coverage** - Create comprehensive unit tests
2. **No Viewport/Scrolling** - Implement scrolling for long conversations
3. **Parameter Validation** - Add defensive programming to prevent panics
4. **Index Out of Bounds Risk** - Add bounds checking
5. **No Responsive Rendering** - Add width/height parameters

### Medium Priority (Address in Next Sprint)
1. Missing String method for ViewMode
2. Limited message role support
3. Fixed width input style
4. Inconsistent color usage
5. Hardcoded UI constants
6. Inconsistent error handling
7. Missing package documentation
8. Insufficient function documentation
9. No architecture documentation
10. No view state separation

### Low Priority (Nice to Have)
1. No message metadata support
2. String concatenation optimization
3. No visual regression tests
4. Race condition documentation
5. Maximum content length limits
6. Missing style documentation
7. No examples
8. Session ID validation
9. Tool parameter privacy
10. Status bar information disclosure
11. Render output caching
12. Tight coupling to lipgloss
13. Magic numbers
14. i18n support
15. Debug mode

---

## Testing Recommendations

Create `views_test.go` with these test categories:

```go
// 1. Basic functionality tests
func TestRenderSessionList_Empty(t *testing.T)
func TestRenderSessionList_SingleSession(t *testing.T)
func TestRenderSessionList_MultipleSessionsWithSelection(t *testing.T)

// 2. Edge case tests
func TestRenderSessionList_NegativeIndex(t *testing.T)
func TestRenderSessionList_IndexOutOfBounds(t *testing.T)
func TestRenderSessionList_LongSessionNames(t *testing.T)

// 3. Security tests
func TestRenderSessionList_ANSIInjection(t *testing.T)
func TestRenderConversation_ANSIInMessages(t *testing.T)

// 4. Integration tests
func TestRenderConversation_WithAllRoles(t *testing.T)
func TestRenderToolApproval_NilParams(t *testing.T)

// 5. Benchmark tests
func BenchmarkRenderSessionList(b *testing.B)
func BenchmarkRenderConversation_LargeHistory(b *testing.B)
```

---

## Conclusion

The `views.go` file demonstrates good security practices with comprehensive ANSI sanitization and clean code structure. However, it suffers from:

1. **Complete lack of test coverage** (highest priority issue)
2. **Missing critical TUI features** (scrolling, responsive layout)
3. **Insufficient error handling** and parameter validation
4. **Limited documentation** for maintainers

The code is functional but needs significant work before it's production-ready. The security implementation is exemplary and should be maintained as new features are added.

**Overall Assessment:** 6.5/10
- **Security:** 9/10 (excellent)
- **Functionality:** 5/10 (incomplete)
- **Code Quality:** 7/10 (good but needs improvement)
- **Testing:** 1/10 (critical gap)
- **Documentation:** 5/10 (minimal)
- **Maintainability:** 6/10 (acceptable but could be better)

**Recommended Next Steps:**
1. Create comprehensive unit tests (1-2 days of work)
2. Implement scrolling/viewport (1 day of work)
3. Add parameter validation to all render functions (4 hours of work)
4. Improve documentation (4 hours of work)
5. Address medium-priority issues in next sprint
