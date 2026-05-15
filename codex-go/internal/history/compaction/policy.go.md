# Code Review: policy.go

**File**: `/Users/williamcory/codex/codex-go/internal/history/compaction/policy.go`
**Review Date**: 2025-10-26
**Lines of Code**: 372

---

## Executive Summary

The `policy.go` file implements conversation history compaction policies for managing message preservation during token-based truncation. The code is generally well-structured and has comprehensive test coverage. However, there are several critical bugs, edge cases, and design issues that need attention.

**Severity Breakdown**:
- Critical Issues: 3
- Major Issues: 5
- Minor Issues: 7
- Documentation Gaps: 4

---

## 1. Incomplete Features & Functionality

### 1.1 Missing PreserveReasoningContent Implementation (CRITICAL)
**Location**: Lines 118-125
**Issue**: The `PreserveReasoningContent` field affects importance scoring but doesn't actually preserve reasoning content in the message preservation logic.

```go
// In GetImportanceScore
if msg.Reasoning != "" {
    if p.PreserveReasoningContent {
        score = 0.8
    } else {
        score = 0.4 // Lower score if we don't preserve reasoning
    }
}
```

**Problem**: The policy calculates different importance scores based on `PreserveReasoningContent`, but there's no mechanism to actually strip or preserve the `Reasoning` field from messages during compaction. The Compactor and Truncator don't have logic to remove reasoning content when `PreserveReasoningContent = false`.

**Impact**: Memory waste - reasoning content is scored lower but still preserved in memory, defeating the purpose of the flag.

### 1.2 Missing Validation in Policy Constructors
**Location**: Lines 29-59
**Issue**: The three policy constructors (`NewDefaultPolicy`, `NewAggressivePolicy`, `NewConservativePolicy`) don't validate their configurations.

```go
func NewAggressivePolicy() *Policy {
    return &Policy{
        PreserveSystemMessages:   true,
        PreserveRecentTurns:      1, // Only keep last turn
        PreserveToolCalls:        false,  // <-- Potentially dangerous
        MinImportanceScore:       0.5,
        PreserveReasoningContent: false,
    }
}
```

**Problem**: `NewAggressivePolicy` sets `PreserveToolCalls: false`, which could break conversation flows that depend on tool call/response sequences. There's no warning or validation.

---

## 2. Code Quality Issues

### 2.1 Inefficient containsMessage Implementation (MAJOR)
**Location**: Lines 285-313
**Issue**: The `containsMessage` function has O(n²) complexity in `PreserveMessages` and uses unreliable equality checks.

```go
func containsMessage(messages []client.Message, target client.Message) bool {
    for _, msg := range messages {
        // Simple equality check based on role and content
        if msg.Role == target.Role {
            // Compare content if both are strings
            msgContent, msgOk := msg.Content.(string)
            targetContent, targetOk := target.Content.(string)

            if msgOk && targetOk && msgContent == targetContent {
                return true
            }
            // ...
```

**Problems**:
1. **Performance**: Called within a loop in `PreserveMessages` (line 90), creating O(n²) complexity
2. **False Positives**: Two different messages with the same role and content would be considered identical
3. **Incomplete Comparison**: Doesn't compare `Name`, `Reasoning`, or other Message fields
4. **Inconsistent Logic**: Sometimes returns true on first tool call ID match (line 306), ignoring other fields

**Better Approach**: Use a map with message index or implement proper structural equality.

### 2.2 Duplicate Message Preservation Logic (MAJOR)
**Location**: Lines 63-98
**Issue**: `PreserveMessages` can add the same message multiple times to the result.

```go
// Preserve recent turns
if p.PreserveRecentTurns > 0 {
    recentMessages := p.getRecentMessages(messages, p.PreserveRecentTurns)
    preserved = append(preserved, recentMessages...)  // <-- No deduplication
}

// Preserve tool calls if configured
if p.PreserveToolCalls {
    for _, msg := range messages {
        if len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
            // Check if not already in preserved list
            if !containsMessage(preserved, msg) {  // <-- Only tool calls are checked
                preserved = append(preserved, msg)
            }
        }
    }
}
```

**Problem**:
- System messages and recent messages are not deduplicated
- A system message in the recent turns window will appear twice
- The deduplication only applies to tool calls, not to other preservation rules

### 2.3 getRecentMessages Filtering Logic Error (CRITICAL)
**Location**: Lines 241-247
**Issue**: The filtering logic for system messages is incorrect and causes messages to be excluded unexpectedly.

```go
// Extract recent messages (excluding system messages already preserved)
recent := make([]client.Message, 0, len(messages)-startIdx)
for i := startIdx; i < len(messages); i++ {
    if messages[i].Role != "system" || !p.PreserveSystemMessages {
        recent = append(recent, messages[i])
    }
}
```

**Problem**: This logic says "include message if it's NOT a system message OR if we're NOT preserving system messages". This means:
- If `PreserveSystemMessages = true`: excludes system messages from recent (correct)
- If `PreserveSystemMessages = false`: includes ALL messages including system (incorrect - should still include system messages)

**Correct Logic**: Should be `if messages[i].Role != "system"`

### 2.4 Turn Counting Inconsistency (MAJOR)
**Location**: Lines 208-250 and Lines 253-283
**Issue**: Different functions count turns differently, leading to off-by-one errors.

In `getRecentMessages`:
```go
// Count user messages as turn boundaries
if msg.Role == "user" {
    turnCount++
    if turnCount > turns {
        startIdx = i + 1
        break
    }
}
```

In `CalculatePositions`:
```go
if msg.Role == "user" {
    currentTurn++
}
pos.PairIndex = currentTurn
pos.TurnsFromEnd = turnCount - currentTurn
```

**Problems**:
1. `getRecentMessages` counts turns backward but increments before checking, causing it to skip one turn
2. `CalculatePositions` assigns the turn number to all messages in a turn, but increments on user messages, meaning the assistant response gets the next turn number
3. The definition of "turn" is ambiguous - is it a user message, or a user+assistant pair?

### 2.5 Scoring Logic Can Exceed 1.0 (MINOR)
**Location**: Lines 102-145
**Issue**: Despite the cap at 1.0, the scoring logic can theoretically exceed it before capping.

```go
score := 0.5 // Base score for regular messages

// User messages slightly more important than assistant (they define intent)
if msg.Role == "user" {
    score += 0.1  // Now 0.6
}

// Long messages may contain important context
if content, ok := msg.Content.(string); ok {
    if len(content) > 500 {
        score += 0.05  // Now 0.65
    }
}
```

**Problem**: While capped at 1.0, the separate conditions can combine in unexpected ways. For example, a tool result with reasoning could theoretically hit `0.85 + 0.1 + 0.05 = 1.0` before the cap, but the logic flow doesn't make this clear.

**Better Approach**: Use a multiplier approach or make the scoring more explicit about maximum possible scores.

### 2.6 Hardcoded Magic Numbers (MINOR)
**Location**: Throughout file
**Issue**: Magic numbers throughout the code without constants or documentation.

Examples:
- Line 34: `PreserveRecentTurns: 3, // Keep last 3 turns (6 messages)` - Why 3? Why not 2 or 5?
- Line 34: `MinImportanceScore: 0.3` - Why 0.3?
- Line 112: `score = 0.9` - Tool call importance
- Line 115: `score = 0.85` - Tool call ID importance
- Line 134: `if len(content) > 500` - Why 500 characters?

**Better Approach**: Extract as named constants with documentation explaining the rationale.

---

## 3. Potential Bugs & Edge Cases

### 3.1 Empty Messages After Preservation (MAJOR)
**Location**: Lines 63-98
**Issue**: `PreserveMessages` can return an empty slice even when messages exist.

```go
func (p *Policy) PreserveMessages(messages []client.Message) []client.Message {
    if len(messages) == 0 {
        return messages
    }

    preserved := make([]client.Message, 0, len(messages))

    // Always preserve system messages if configured
    if p.PreserveSystemMessages {
        // ...
    }

    // ...
    return preserved  // Could be empty if all conditions are false
}
```

**Problem**: If `PreserveSystemMessages = false`, `PreserveRecentTurns = 0`, and `PreserveToolCalls = false`, the function returns an empty slice even though messages were provided. This could cause issues in compaction logic that expects at least some messages to be preserved.

**Impact**: Potential for losing entire conversation history.

### 3.2 Nil Policy Not Handled (MINOR)
**Location**: Throughout file
**Issue**: Methods on `*Policy` don't check for nil receiver, which could cause panics.

```go
func (p *Policy) PreserveMessages(messages []client.Message) []client.Message {
    if len(messages) == 0 {
        return messages
    }
    // No nil check for p
    if p.PreserveSystemMessages {  // Panic if p is nil
```

**Impact**: While `NewCompactor` provides a default policy, direct use of Policy methods could panic.

### 3.3 Integer Overflow in Token Counting (MINOR)
**Location**: Lines 315-330, 338-371
**Issue**: Token counting uses `int`, which could overflow with very large conversations.

```go
func (p *Policy) EstimateTokenSavings(messages []client.Message, counter TokenCounter) (canRemove, mustKeep int) {
    positions := p.CalculatePositions(messages)

    for i, msg := range messages {
        tokens := countMessageTokens(msg, counter)  // int

        if p.ShouldPreserve(msg, positions[i]) {
            mustKeep += tokens  // Could overflow
        } else {
            canRemove += tokens  // Could overflow
        }
    }
```

**Impact**: Low probability but catastrophic if it occurs - could result in negative values or incorrect compaction decisions.

**Better Approach**: Use `int64` for token counts, or add overflow checks.

### 3.4 ContentItem Text Not Validated (MINOR)
**Location**: Lines 349-353
**Issue**: The code assumes `ContentItem.Text` is always valid but doesn't check for empty items.

```go
case []client.ContentItem:
    for _, item := range content {
        tokens += counter.CountTokens(item.Text)  // What if Text is empty?
    }
```

**Problem**: Empty content items still get processed, potentially adding overhead. Also doesn't handle other content item types (images, etc.).

### 3.5 Tool Call Preservation Doesn't Preserve Tool Results (CRITICAL)
**Location**: Lines 86-95, 160
**Issue**: The policy preserves messages with `ToolCalls` or `ToolCallID`, but doesn't ensure both the tool call AND its result are preserved together.

```go
// Preserve tool calls if configured
if p.PreserveToolCalls {
    for _, msg := range messages {
        if len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
            if !containsMessage(preserved, msg) {
                preserved = append(preserved, msg)
            }
        }
    }
}
```

**Problem**:
- A message with `ToolCalls` (assistant's tool invocation) gets preserved
- A message with `ToolCallID` (tool's response) gets preserved
- BUT: The logical pairing isn't maintained. If one gets filtered out for other reasons, you can have orphaned tool calls or results.

**Impact**: Breaks conversation coherence for tool-using assistants. The LLM might see a tool invocation without its result, or vice versa.

### 3.6 Race Condition in Token Counter Usage
**Location**: Lines 316-330, 338-371
**Issue**: The `TokenCounter` interface is used concurrently without synchronization guarantees.

```go
func (p *Policy) EstimateTokenSavings(messages []client.Message, counter TokenCounter) (canRemove, mustKeep int) {
    positions := p.CalculatePositions(messages)

    for i, msg := range messages {
        tokens := countMessageTokens(msg, counter)  // Is counter thread-safe?
```

**Problem**: While the Policy itself doesn't have state, if the `TokenCounter` implementation is not thread-safe and is shared across goroutines, this could cause data races. The interface doesn't document thread-safety requirements.

---

## 4. Missing Test Coverage

### 4.1 Edge Cases Not Tested
**Missing Test Cases**:

1. **Empty recent turns**: What happens when `PreserveRecentTurns = 0`?
2. **Only system messages**: Conversation with only system messages
3. **Interleaved roles**: User->User->Assistant (non-standard turn structure)
4. **Tool call without result**: Orphaned tool calls
5. **Concurrent access**: Policy methods called from multiple goroutines
6. **Nil content**: Messages with `nil` Content field
7. **Very large turn count**: `PreserveRecentTurns > total turns`
8. **Negative importance scores**: Although impossible with current logic, should be validated
9. **All messages above importance threshold**: No messages to remove
10. **Turn boundaries with tool messages**: Does a tool message count as a turn?

### 4.2 Integration Tests Missing
**Issue**: No tests verify the interaction between Policy and the rest of the compaction system.

**Needed Tests**:
1. Policy + Truncator integration
2. Policy + Summarizer integration
3. Policy changes during compaction
4. Policy with different Message.Content types (string vs []ContentItem)

### 4.3 Performance Tests Missing
**Issue**: No benchmarks or performance tests for policy evaluation.

**Needed Tests**:
1. Benchmark `PreserveMessages` with large message lists
2. Benchmark `CalculatePositions` with many messages
3. Benchmark `containsMessage` (known O(n²) issue)
4. Memory allocation profiling

---

## 5. Documentation Issues

### 5.1 Ambiguous "Turn" Definition (MAJOR)
**Location**: Lines 15-16, 32, 204-205
**Issue**: The term "turn" is used inconsistently and never formally defined.

```go
// PreserveRecentTurns keeps the N most recent conversation turns
// A turn is a user message + assistant response pair
PreserveRecentTurns int
```

vs.

```go
// Count user messages as turn boundaries
if msg.Role == "user" {
    turnCount++
```

**Problem**: The comment says "user message + assistant response pair" but the code counts user messages. These are different:
- 3 turns as pairs: 6 messages (user, assistant, user, assistant, user, assistant)
- 3 user messages: could be 3-6 messages depending on assistant responses

### 5.2 Missing Package Examples (MINOR)
**Location**: N/A
**Issue**: No package-level examples showing how to use policies.

**Needed**:
```go
// Example_customPolicy shows how to create a custom policy
func Example_customPolicy() {
    policy := &Policy{
        PreserveSystemMessages:   true,
        PreserveRecentTurns:      5,
        PreserveToolCalls:        true,
        MinImportanceScore:       0.4,
        PreserveReasoningContent: true,
    }

    // Use policy...
}
```

### 5.3 Missing Field Validation Ranges (MINOR)
**Location**: Lines 10-26
**Issue**: No documentation on valid ranges for policy fields.

```go
type Policy struct {
    // PreserveSystemMessages ensures system prompts are never removed
    PreserveSystemMessages bool

    // PreserveRecentTurns keeps the N most recent conversation turns
    // A turn is a user message + assistant response pair
    PreserveRecentTurns int  // Valid range? Can it be negative? What's a reasonable max?

    // PreserveToolCalls ensures messages with tool calls are kept
    PreserveToolCalls bool

    // MinImportanceScore is the threshold for keeping messages (0.0 to 1.0)
    MinImportanceScore float64  // What happens if > 1.0 or < 0.0?
```

**Needed**: Document valid ranges and behavior for invalid values.

### 5.4 Undocumented Scoring Algorithm (MINOR)
**Location**: Lines 100-145
**Issue**: The importance scoring algorithm is not documented at the function level.

**Needed**: Document the scoring factors and weights:
```go
// GetImportanceScore calculates an importance score for a message (0.0 to 1.0).
// Higher scores indicate messages that should be preserved during compaction.
//
// Scoring Factors:
//   - System messages: 1.0 (maximum)
//   - Tool calls: 0.9
//   - Tool results: 0.85
//   - Reasoning (if preserved): 0.8
//   - Reasoning (if not preserved): 0.4
//   - User messages: +0.1 bonus (defines user intent)
//   - Long messages (>500 chars): +0.05 bonus (likely important context)
//   - Base score: 0.5
//
// Scores are capped at 1.0.
func (p *Policy) GetImportanceScore(msg client.Message) float64 {
```

---

## 6. Security Concerns

### 6.1 No Input Validation (MODERATE)
**Location**: Lines 63, 148, 253, 316
**Issue**: Public methods don't validate inputs, allowing potentially malicious data.

**Examples**:
```go
func (p *Policy) PreserveMessages(messages []client.Message) []client.Message {
    if len(messages) == 0 {
        return messages  // No validation of message content
    }
    // ...
}

func (p *Policy) ShouldPreserve(msg client.Message, position MessagePosition) bool {
    // No validation that position.Index matches actual position
    // No validation that position values are consistent
```

**Potential Issues**:
1. **Resource Exhaustion**: No limit on message count or size
2. **Invalid Position Data**: `MessagePosition` fields not validated (e.g., negative indices)
3. **Type Confusion**: `msg.Content` type not validated before casting

### 6.2 No Rate Limiting or Resource Controls (LOW)
**Location**: Throughout
**Issue**: No protection against resource exhaustion from large inputs.

**Examples**:
- `PreserveMessages` with 10,000+ messages could cause memory issues
- `CalculatePositions` allocates a slice of positions equal to message count
- No timeout mechanisms

**Recommendation**: Add resource limits or streaming APIs for large conversations.

### 6.3 Information Leakage in Default Policies (LOW)
**Location**: Lines 28-59
**Issue**: Default policies might not be suitable for sensitive contexts.

```go
func NewDefaultPolicy() *Policy {
    return &Policy{
        PreserveSystemMessages:   true,
        PreserveRecentTurns:      3, // Keep last 3 turns (6 messages)
        PreserveToolCalls:        true,
        MinImportanceScore:       0.3,
        PreserveReasoningContent: false, // Reasoning can be large, drop by default
    }
}
```

**Problem**: For sensitive applications, defaulting to preserve system messages (which might contain instructions with sensitive info) and tool calls (which might contain credentials or API keys) could be risky.

**Recommendation**: Document security considerations and provide a `NewSecurePolicy()` variant.

---

## 7. Recommendations

### 7.1 Critical Fixes (Immediate)

1. **Fix turn counting logic**: Standardize on a single definition of "turn" and implement consistently
2. **Fix tool call pairing**: Ensure tool calls and their results are preserved together
3. **Fix getRecentMessages filtering**: Correct the boolean logic for system message exclusion
4. **Add deduplication**: Prevent duplicate messages in `PreserveMessages`

### 7.2 High Priority Improvements

1. **Replace containsMessage**: Use index-based tracking instead of content comparison
2. **Add input validation**: Validate policy fields (ranges, consistency)
3. **Document scoring algorithm**: Add detailed comments explaining scoring weights
4. **Add validation method**: `func (p *Policy) Validate() error`

### 7.3 Medium Priority Improvements

1. **Extract magic numbers**: Create named constants for all thresholds
2. **Add nil checks**: Validate receivers and parameters
3. **Improve test coverage**: Add edge case and integration tests
4. **Use int64 for tokens**: Prevent potential overflow
5. **Add benchmarks**: Measure performance of key operations

### 7.4 Low Priority Improvements

1. **Add package examples**: Show common usage patterns
2. **Document security considerations**: Add security notes to README
3. **Add resource limits**: Protect against resource exhaustion
4. **Improve error messages**: More descriptive errors for policy violations

---

## 8. Positive Aspects

Despite the issues identified, the code has several strengths:

1. **Clear Structure**: The Policy abstraction is well-designed and separates concerns
2. **Good Test Coverage**: The `compaction_test.go` file has extensive tests (1065 lines)
3. **Flexible Design**: Three policy presets (Default, Aggressive, Conservative) cover common use cases
4. **Composable**: Policy works well with Truncator and Summarizer components
5. **Performance Conscious**: The `EstimateTokenSavings` allows for cost prediction before compaction
6. **Rich Metadata**: `MessagePosition` provides good context for decision making

---

## 9. Conclusion

The `policy.go` file implements a solid foundation for conversation history management but requires significant bug fixes and improvements before production use. The most critical issues are:

1. **Turn counting inconsistencies** leading to off-by-one errors
2. **Tool call preservation** not maintaining call-result pairs
3. **Duplicate message preservation** causing memory waste
4. **Performance issues** from O(n²) containsMessage

**Overall Assessment**: **6/10** - Functional but needs refactoring

**Recommended Action**: Address critical bugs first, then proceed with high-priority improvements before considering this production-ready.

---

## Appendix: Suggested Refactorings

### A.1 Improved containsMessage

```go
// Use index-based tracking instead
func (p *Policy) PreserveMessages(messages []client.Message) []client.Message {
    preservedIndices := make(map[int]bool)

    // Mark system messages
    if p.PreserveSystemMessages {
        for i, msg := range messages {
            if msg.Role == "system" {
                preservedIndices[i] = true
            }
        }
    }

    // Mark recent messages
    if p.PreserveRecentTurns > 0 {
        recentIndices := p.getRecentMessageIndices(messages, p.PreserveRecentTurns)
        for i := range recentIndices {
            preservedIndices[i] = true
        }
    }

    // Mark tool calls
    if p.PreserveToolCalls {
        for i, msg := range messages {
            if len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
                preservedIndices[i] = true
            }
        }
    }

    // Build result from indices
    result := make([]client.Message, 0, len(preservedIndices))
    for i := range messages {
        if preservedIndices[i] {
            result = append(result, messages[i])
        }
    }

    return result
}
```

### A.2 Policy Validation

```go
// Validate checks if the policy configuration is valid
func (p *Policy) Validate() error {
    if p == nil {
        return fmt.Errorf("policy cannot be nil")
    }

    if p.PreserveRecentTurns < 0 {
        return fmt.Errorf("PreserveRecentTurns cannot be negative: %d", p.PreserveRecentTurns)
    }

    if p.MinImportanceScore < 0.0 || p.MinImportanceScore > 1.0 {
        return fmt.Errorf("MinImportanceScore must be between 0.0 and 1.0: %f", p.MinImportanceScore)
    }

    return nil
}
```

### A.3 Named Constants

```go
const (
    // Importance scores
    ScoreSystemMessage    = 1.0
    ScoreToolCall        = 0.9
    ScoreToolResult      = 0.85
    ScoreReasoning       = 0.8
    ScoreReasoningUnused = 0.4
    ScoreBase            = 0.5
    ScoreUserBonus       = 0.1
    ScoreLongMessageBonus = 0.05

    // Thresholds
    LongMessageThreshold = 500 // characters

    // Default policy values
    DefaultPreserveRecentTurns = 3
    DefaultMinImportanceScore  = 0.3

    // Conservative policy values
    ConservativePreserveRecentTurns = 10
    ConservativeMinImportanceScore  = 0.1

    // Aggressive policy values
    AggressivePreserveRecentTurns = 1
    AggressiveMinImportanceScore  = 0.5
)
```

---

**End of Review**
