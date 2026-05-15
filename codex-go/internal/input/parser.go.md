# Code Review: parser.go

**File:** `/Users/williamcory/codex/codex-go/internal/input/parser.go`
**Review Date:** 2025-10-26
**Overall Test Coverage:** 60.7% (package level)

---

## Executive Summary

The parser.go file implements file reference parsing functionality for the @ syntax (e.g., `@filename.txt`). The code is generally well-structured and includes basic security measures. However, there are significant issues with incomplete functionality, missing edge case handling, potential bugs, and insufficient test coverage. Several critical security and robustness improvements are needed before production use.

**Severity Levels:**
- 🔴 **CRITICAL** - Must fix before production
- 🟠 **HIGH** - Should fix soon
- 🟡 **MEDIUM** - Should improve
- 🟢 **LOW** - Nice to have

---

## 1. Incomplete Features & Functionality

### 🔴 CRITICAL: Escape Sequence Handling Incomplete (Line 100-102)
**Location:** `extractFileReference()` function, lines 100-102

```go
if input[i] == '\\' && i+1 < len(input) {
    // Handle escaped quotes
    i += 2
}
```

**Issue:** The code acknowledges escaped quotes in comments but doesn't actually use the unescaped value. When building `pathStr` from `input[start:i]`, it includes the backslashes. This means `@"file\"name.txt"` would try to open a file literally named `file\"name.txt` instead of `file"name.txt`.

**Impact:** Users cannot reference files with quotes in their names, and the escape sequence doesn't work as expected.

**Recommendation:** Implement proper escape sequence processing. Build the path string character by character, interpreting escape sequences:

```go
var pathBuilder strings.Builder
for i < len(input) && input[i] != '"' {
    if input[i] == '\\' && i+1 < len(input) && input[i+1] == '"' {
        pathBuilder.WriteByte('"')
        i += 2
    } else {
        pathBuilder.WriteByte(input[i])
        i++
    }
}
pathStr = pathBuilder.String()
```

### 🟠 HIGH: Incomplete Escape Sequence Support
**Location:** Multiple locations (lines 100-102, 224-229)

**Issue:** Only `\"` escape sequences are considered. Standard escape sequences like `\\`, `\n`, `\t` are not handled.

**Impact:**
- Users cannot reference files with backslashes in names on Windows
- Behavior is inconsistent with common escape sequence conventions
- No way to include literal backslashes in quoted paths

**Recommendation:** Decide on the complete set of supported escape sequences and document them clearly. Consider supporting at least:
- `\"` - literal quote
- `\\` - literal backslash
- Or follow Go string literal rules

### 🟡 MEDIUM: Missing Functionality - No Multiple @ References in Same Token
**Location:** `extractFileReference()` function

**Issue:** If input contains `@@file.txt` or `@file1@file2`, the parser behavior is undefined and likely incorrect.

**Test Cases Missing:**
- `"Check @@file.txt"` - Should this be an escaped @ or an error?
- `"Check @file1.txt@file2.txt"` - How should this be parsed?

**Recommendation:** Define and document behavior for edge cases involving multiple @ symbols.

---

## 2. Code Quality Issues

### 🟠 HIGH: Inconsistent Error Handling
**Location:** `ParseFileReferences()` function, lines 56-61

```go
if err != nil {
    // Not a valid file reference, keep the @ symbol
    processedText.WriteByte(input[i])
    i++
    continue
}
```

**Issue:** Errors are silently swallowed. The function signature returns an error, but `extractFileReference` errors are caught and ignored. This makes debugging difficult and behavior unclear.

**Impact:**
- Users don't know why their @ reference didn't work
- No distinction between "file not found" and "invalid syntax"
- Silent failures lead to confusion

**Recommendation:** Consider one of these approaches:
1. Collect errors and return them as warnings alongside the result
2. Add a strict mode that fails on any parsing error
3. At minimum, log errors at debug level

### 🟡 MEDIUM: Magic Numbers Without Constants
**Location:** Multiple locations

```go
ti.CharLimit = 500  // In app.go (not parser.go but related)
```

**Issue:** While parser.go doesn't have magic numbers, there are no documented limits on:
- Maximum path length
- Maximum number of @ references per input
- Maximum input length

**Recommendation:** Define and document limits with named constants.

### 🟡 MEDIUM: Inefficient String Building
**Location:** Lines 110, 122 in `extractFileReference()`

```go
pathStr = input[start:i]
```

**Issue:** While slicing is efficient, the escaped quote handling (if fixed) will require character-by-character building anyway. Current code is fine, but will need refactoring when escape sequences are properly implemented.

### 🟢 LOW: Redundant Path Cleaning
**Location:** `resolvePath()` function, line 149

```go
cleaned := filepath.Clean(path)
```

**Issue:** `filepath.Join()` on line 156 also cleans the path, making this first cleaning redundant for relative paths. However, this doesn't hurt performance significantly.

---

## 3. Potential Bugs & Edge Cases

### 🔴 CRITICAL: Race Condition Potential
**Location:** `ParseFileReferences()` function, lines 39-45

```go
if workingDir == "" {
    var err error
    workingDir, err = os.Getwd()
    if err != nil {
        return nil, fmt.Errorf("failed to get working directory: %w", err)
    }
}
```

**Issue:** If the working directory changes between parsing and file validation/reading (TOCTOU - Time of Check Time of Use), resolved paths may become invalid.

**Impact:**
- Files might not be found even though parsing succeeded
- Security implications if working directory changes unexpectedly

**Recommendation:**
1. Always require `workingDir` parameter (don't allow empty)
2. Or document that callers must not change working directory during operation
3. Add validation that working directory exists and is accessible

### 🔴 CRITICAL: Path Traversal Vulnerability (Partial)
**Location:** `resolvePath()` function, lines 161-167

```go
if !filepath.IsAbs(path) {
    // For relative paths, ensure they don't escape working directory
    relPath, err := filepath.Rel(workingDir, absPath)
    if err != nil || strings.HasPrefix(relPath, "..") {
        return "", "", fmt.Errorf("path traversal not allowed: %s", path)
    }
}
```

**Issues:**

1. **Absolute paths bypass check entirely:** If user provides an absolute path like `@/etc/passwd`, the check is skipped (line 161 condition is false). While line 153 checks `filepath.IsAbs(cleaned)`, the security check only applies to relative paths.

2. **Symlink attack vector:** The code doesn't check for symbolic links. A file `safe.txt -> /etc/passwd` would pass all checks but access a sensitive file.

3. **Working directory validation missing:** If `workingDir` is itself malicious or contains symlinks, the protection fails.

**Impact:**
- Users can potentially read any file on the system
- Symlinks can be used to bypass path restrictions
- Critical security vulnerability for multi-tenant or sandboxed environments

**Recommendation:**
```go
// 1. Validate working directory is absolute and exists
if !filepath.IsAbs(workingDir) {
    return "", "", fmt.Errorf("working directory must be absolute")
}

// 2. For absolute paths, verify they're within allowed directories
if filepath.IsAbs(path) {
    // Either reject all absolute paths, or maintain a whitelist
    // Current code allows ANY absolute path - dangerous!
    if !isPathAllowed(absPath, allowedPaths) {
        return "", "", fmt.Errorf("absolute path not allowed: %s", path)
    }
}

// 3. Check for symlinks and resolve them
realPath, err := filepath.EvalSymlinks(absPath)
if err == nil {
    // Verify the real path is still within bounds
    relPath, err := filepath.Rel(workingDir, realPath)
    if err != nil || strings.HasPrefix(relPath, "..") {
        return "", "", fmt.Errorf("symlink escapes working directory: %s", path)
    }
    absPath = realPath
}
```

### 🟠 HIGH: Unicode Handling Issues
**Location:** Multiple locations using `input[i]`

**Issue:** The code treats strings as byte arrays (`input[i]`), which breaks with multi-byte UTF-8 characters. A file named `@café.txt` might be misparsed if the cursor lands in the middle of `é`.

**Impact:**
- International file names may be incorrectly parsed
- Potential panics or corruption with non-ASCII characters
- `GetCurrentToken()` may return partial characters

**Recommendation:** Use `[]rune` conversion or the `unicode/utf8` package for proper UTF-8 handling:

```go
runes := []rune(input)
for i < len(runes) {
    if runes[i] == '@' {
        // ...
    }
}
```

### 🟠 HIGH: GetCurrentToken() Boundary Issues
**Location:** `GetCurrentToken()` function, lines 196-255

**Issues:**

1. **Off-by-one potential:** Line 202 adjusts `searchStart` but doesn't validate bounds properly:
   ```go
   if searchStart > 0 && (searchStart >= len(input) || unicode.IsSpace(rune(input[searchStart]))) {
       searchStart--
   }
   ```
   If `searchStart == len(input)` and `searchStart > 0`, it decrements to `len(input)-1`, which is correct. But the logic is hard to follow.

2. **Cursor at end edge case:** When cursor is at `len(input)`, the function should still work, but the logic is convoluted.

3. **No validation of return values:** `startPos` can be returned without validation that it's a valid index.

**Test Cases Missing:**
- Cursor at position 0
- Cursor at position `len(input)`
- Empty string input
- Input with only `@` character

**Recommendation:** Add comprehensive boundary tests and simplify the logic.

### 🟡 MEDIUM: Directory Check After Path Resolution
**Location:** `resolvePath()` function, lines 179-181

```go
if info.IsDir() {
    return "", "", fmt.Errorf("path is a directory, not a file: %s", path)
}
```

**Issue:** This check comes AFTER path traversal validation, but before symlink resolution. A symlink to a directory would pass as a file initially, then fail later when actually read.

**Impact:** Inconsistent error messages and behavior with symlinks.

**Recommendation:** Perform all security checks (including symlink resolution) before any validation, then do feature checks (is it a file? is it readable?).

### 🟡 MEDIUM: No Maximum Path Length Validation
**Location:** Throughout parsing functions

**Issue:** Go's maximum path length varies by OS (typically 4096 on Unix, 260 on Windows without special flags). No validation is performed.

**Impact:**
- Potential buffer overflows in downstream code
- Platform-specific failures
- DoS potential with extremely long paths

**Recommendation:** Define and enforce `MaxPathLength` constant (e.g., 4096).

### 🟢 LOW: Empty File Reference Handling
**Location:** Line 119-121

```go
if i == start {
    return FileReference{}, 0, fmt.Errorf("empty file reference")
}
```

**Issue:** This correctly catches `@ ` (@ followed by space), but the error is then silently ignored by the caller. Users see `@ ` unchanged in output with no explanation.

**Impact:** Minor usability issue - users might not realize `@ ` is invalid.

---

## 4. Missing Test Coverage

**Current Coverage:** 60.7% (package level)

### 🔴 CRITICAL: Missing Security Tests

**Missing test cases:**

1. **Path traversal attacks:**
   ```go
   // Should add tests for:
   "@../../../etc/passwd"
   "@..\\..\\..\\windows\\system32\\config\\sam"  // Windows
   "@./../../../etc/passwd"
   "@/etc/passwd"  // Absolute path outside working dir
   ```

2. **Symlink attacks:**
   ```go
   // Create symlink pointing outside working dir
   // Verify it's rejected
   ```

3. **Malicious filenames:**
   ```go
   "@\x00file.txt"  // Null byte injection
   "@file\nname.txt"  // Newline in filename
   "@"  // Just @ alone
   ```

### 🟠 HIGH: Missing Edge Case Tests

1. **UTF-8 and Unicode:**
   ```go
   "@café.txt"
   "@文件.txt"  // Chinese characters
   "@file\u200B.txt"  // Zero-width space
   ```

2. **Escape sequence edge cases:**
   ```go
   "@\"file\\\".txt\""  // Escaped backslash before quote
   "@\"file\\"  // Unclosed quote with escape at end
   "@\"\\\\\""  // Just escaped backslashes
   ```

3. **Multiple @ symbols:**
   ```go
   "@@file.txt"
   "@file1@file2"
   "user@domain.com"  // Should not match email addresses
   ```

4. **Boundary conditions:**
   ```go
   ""  // Empty input
   "@"  // Just @
   "@\"\"" // Empty quoted string
   strings.Repeat("@file.txt ", 1000)  // Many references
   strings.Repeat("a", 10000) + "@file.txt"  // Long input
   ```

5. **GetCurrentToken edge cases:**
   ```go
   GetCurrentToken("", 0)  // Empty string
   GetCurrentToken("@", 0)  // Cursor before @
   GetCurrentToken("@", 1)  // Cursor after @
   GetCurrentToken("@\"unclosed", 10)  // Cursor in unclosed quote
   ```

### 🟡 MEDIUM: Missing Integration Tests

**Missing scenarios:**
1. Parsing multiple files and reading their contents
2. Interaction with `validator.go` functions
3. Concurrent parsing of the same input
4. Parsing with changing working directory

### 🟡 MEDIUM: Missing Performance/Stress Tests

**Missing tests:**
1. Large input strings (1MB+)
2. Many file references (100+)
3. Deep directory nesting (100+ levels)
4. Files with very long names (255+ characters)

---

## 5. Documentation Issues

### 🟠 HIGH: Missing Security Documentation

**Location:** File header and function comments

**Issues:**
1. No documentation about security model
2. No warning about path traversal protection limitations
3. No guidance on safe usage in multi-tenant environments
4. No mention of symlink handling

**Recommendation:** Add security section to package documentation:

```go
// Security Considerations:
//
// Path Traversal: The parser implements basic path traversal protection
// for relative paths. However, absolute paths are allowed by default.
// In security-sensitive environments, consider:
//   1. Disabling absolute paths via AllowAbsolutePaths option
//   2. Maintaining a whitelist of allowed directories
//   3. Running the application in a chroot or container
//
// Symlinks: The current implementation does NOT resolve symbolic links.
// A symlink can point outside the working directory. Use filepath.EvalSymlinks
// if strict containment is required.
//
// Race Conditions: File permissions and paths are checked at parse time
// but used later. Ensure the working directory and file permissions don't
// change between parsing and reading.
```

### 🟡 MEDIUM: Incomplete Function Documentation

**Issues:**

1. **ParseFileReferences:** Doesn't document error handling behavior (errors are ignored)
2. **extractFileReference:** Doesn't document supported escape sequences
3. **resolvePath:** Doesn't document security checks performed
4. **GetCurrentToken:** Doesn't document edge cases (cursor at end, empty input, etc.)

**Recommendation:** Expand each function's godoc with:
- Detailed parameter requirements
- Error conditions
- Edge case behavior
- Examples of edge cases

### 🟡 MEDIUM: Missing Package-Level Documentation

**Location:** File header (missing)

**Issue:** No package-level documentation explaining:
- What @ syntax is for
- How it's used in the larger application
- Design decisions (why certain choices were made)
- Relationship with `validator.go`

**Recommendation:** Add comprehensive package documentation:

```go
// Package input provides parsing and validation for user input with file references.
//
// The parser supports @ syntax for referencing files in user input:
//   - @file.txt - relative path from working directory
//   - @"file with spaces.txt" - quoted paths for special characters
//   - @/absolute/path.txt - absolute paths (if allowed)
//
// The parser extracts these references and replaces them with placeholders,
// allowing the application to read file contents and inject them into prompts.
//
// Usage:
//   result, err := ParseFileReferences("Check @config.json", "/working/dir")
//   for _, ref := range result.FileReferences {
//       content, _ := validator.ReadFileContent(ref)
//       // Use content...
//   }
//
// See validator.go for file content reading and validation options.
package input
```

### 🟢 LOW: Missing Examples in Documentation

**Issue:** No examples in function comments for complex functions.

**Recommendation:** Add more examples, especially for edge cases:

```go
// Examples:
//   ParseFileReferences("Check @file.txt", "/dir")
//   // => ProcessedText: "Check [file: file.txt]"
//
//   ParseFileReferences("Check @nonexistent.txt", "/dir")
//   // => ProcessedText: "Check @nonexistent.txt" (unchanged)
//
//   ParseFileReferences("@\"file with spaces.txt\"", "/dir")
//   // => ProcessedText: "[file: file with spaces.txt]"
```

---

## 6. Security Concerns

### 🔴 CRITICAL: Insufficient Path Validation
**See "Path Traversal Vulnerability" in Section 3**

### 🔴 CRITICAL: Symlink Attack Vector
**See "Path Traversal Vulnerability" in Section 3**

### 🟠 HIGH: No Rate Limiting or DoS Protection

**Location:** Entire parser

**Issue:** No protection against:
1. Extremely long input strings (MB or GB of input)
2. Thousands of @ references in a single input
3. Deeply nested directory structures
4. Repeated parsing of the same malicious input

**Impact:**
- DoS via CPU exhaustion
- DoS via memory exhaustion
- Resource exhaustion in multi-user environments

**Recommendation:**
```go
const (
    MaxInputLength = 1_000_000  // 1MB
    MaxFileReferences = 100
    MaxPathLength = 4096
)

func ParseFileReferences(input string, workingDir string) (*ParseResult, error) {
    if len(input) > MaxInputLength {
        return nil, fmt.Errorf("input too long: %d bytes (max %d)", len(input), MaxInputLength)
    }
    // ... continue with parsing
    if len(refs) >= MaxFileReferences {
        return nil, fmt.Errorf("too many file references: %d (max %d)", len(refs), MaxFileReferences)
    }
}
```

### 🟠 HIGH: No Input Sanitization

**Location:** Multiple locations processing user input

**Issue:**
1. No validation that input is valid UTF-8
2. No removal of control characters
3. No handling of null bytes (`\x00`)

**Impact:**
- Potential injection attacks in downstream code
- Log injection if file paths are logged
- Terminal escape sequence injection

**Recommendation:**
```go
func sanitizeInput(input string) (string, error) {
    // Ensure valid UTF-8
    if !utf8.ValidString(input) {
        return "", fmt.Errorf("invalid UTF-8 in input")
    }

    // Check for null bytes
    if strings.Contains(input, "\x00") {
        return "", fmt.Errorf("null bytes not allowed in input")
    }

    return input, nil
}
```

### 🟡 MEDIUM: Working Directory Validation Missing

**Location:** `ParseFileReferences()`, lines 39-45

**Issue:** The working directory from `os.Getwd()` is trusted without validation:
1. Not checked if it exists
2. Not checked if it's readable
3. Not checked if it's a valid absolute path

**Impact:**
- Confusing errors later in execution
- Potential security issues if working directory is attacker-controlled

**Recommendation:**
```go
if workingDir == "" {
    var err error
    workingDir, err = os.Getwd()
    if err != nil {
        return nil, fmt.Errorf("failed to get working directory: %w", err)
    }
}

// Validate working directory
if !filepath.IsAbs(workingDir) {
    return nil, fmt.Errorf("working directory must be absolute: %s", workingDir)
}
info, err := os.Stat(workingDir)
if err != nil {
    return nil, fmt.Errorf("cannot access working directory: %w", err)
}
if !info.IsDir() {
    return nil, fmt.Errorf("working directory is not a directory: %s", workingDir)
}
```

### 🟢 LOW: Error Messages Leak Path Information

**Location:** Multiple error messages

**Issue:** Error messages include full file paths, which could leak system information in multi-tenant environments:
```go
return "", "", fmt.Errorf("file not found: %s", path)
```

**Impact:** Information disclosure in logs or error messages shown to users.

**Recommendation:** Consider redacting sensitive parts of paths in error messages, or use display names only:
```go
return "", "", fmt.Errorf("file not found: %s", displayName)
```

---

## 7. Additional Recommendations

### 🟠 HIGH: Add Context Support

**Issue:** Functions don't accept `context.Context`, making it impossible to:
- Cancel long-running operations
- Add timeout protection
- Trace operations
- Pass request-scoped values

**Recommendation:**
```go
func ParseFileReferencesWithContext(ctx context.Context, input string, workingDir string) (*ParseResult, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // ... continue with parsing
}
```

### 🟡 MEDIUM: Consider Caching Parsed Results

**Issue:** If the same input is parsed multiple times, it's wasteful. Consider adding optional caching.

**Recommendation:**
```go
type ParserCache struct {
    mu    sync.RWMutex
    cache map[string]*ParseResult
}

func (pc *ParserCache) ParseFileReferences(input string, workingDir string) (*ParseResult, error) {
    key := workingDir + ":" + input

    pc.mu.RLock()
    if result, ok := pc.cache[key]; ok {
        pc.mu.RUnlock()
        return result, nil
    }
    pc.mu.RUnlock()

    // Parse and cache...
}
```

### 🟡 MEDIUM: Add Metrics/Telemetry

**Issue:** No visibility into parser usage, errors, or performance.

**Recommendation:** Add metrics for:
- Number of files parsed
- Parse errors by type
- Parse duration
- File reference counts

### 🟡 MEDIUM: Improve Error Types

**Issue:** All errors are plain strings. Callers can't programmatically distinguish error types.

**Recommendation:**
```go
type ErrorType int

const (
    ErrPathTraversal ErrorType = iota
    ErrFileNotFound
    ErrFileAccess
    ErrInvalidSyntax
    ErrQuoteUnclosed
)

type ParseError struct {
    Type    ErrorType
    Message string
    Path    string
    Pos     int
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("parse error at position %d: %s", e.Pos, e.Message)
}
```

### 🟢 LOW: Consider Builder Pattern for Options

**Issue:** `resolvePath` has a growing number of implicit security policies. Consider making them explicit:

```go
type ParserOptions struct {
    WorkingDir         string
    AllowAbsolute      bool
    AllowSymlinks      bool
    MaxPathLength      int
    MaxReferences      int
    AllowedDirectories []string
}

func ParseFileReferencesWithOptions(input string, opts ParserOptions) (*ParseResult, error) {
    // ...
}
```

---

## 8. Test Recommendations

### Priority 1: Security Tests (Must Add)
```go
func TestPathTraversalPrevention(t *testing.T) { /* ... */ }
func TestSymlinkHandling(t *testing.T) { /* ... */ }
func TestAbsolutePathSecurity(t *testing.T) { /* ... */ }
func TestMaliciousFilenames(t *testing.T) { /* ... */ }
func TestDoSPrevention(t *testing.T) { /* ... */ }
```

### Priority 2: Edge Cases (Should Add)
```go
func TestUnicodeHandling(t *testing.T) { /* ... */ }
func TestEscapeSequences(t *testing.T) { /* ... */ }
func TestMultipleAtSymbols(t *testing.T) { /* ... */ }
func TestBoundaryConditions(t *testing.T) { /* ... */ }
```

### Priority 3: Integration Tests (Nice to Have)
```go
func TestParseAndValidate(t *testing.T) { /* ... */ }
func TestConcurrentParsing(t *testing.T) { /* ... */ }
```

---

## 9. Summary of Critical Issues

| Issue | Severity | Line(s) | Status |
|-------|----------|---------|--------|
| Escape sequences not actually processed | 🔴 CRITICAL | 100-110 | Not Addressed |
| Path traversal via absolute paths | 🔴 CRITICAL | 161-167 | Partially Addressed |
| Symlink attack vector | 🔴 CRITICAL | 169-176 | Not Addressed |
| Race condition with working directory | 🔴 CRITICAL | 39-45 | Not Addressed |
| No DoS protection | 🟠 HIGH | All | Not Addressed |
| UTF-8 handling issues | 🟠 HIGH | Multiple | Not Addressed |
| Silent error swallowing | 🟠 HIGH | 56-61 | Not Addressed |
| Missing security documentation | 🟠 HIGH | File | Not Addressed |
| Incomplete test coverage (60.7%) | 🟠 HIGH | N/A | Partial |

---

## 10. Recommended Action Plan

### Phase 1: Critical Fixes (Do Immediately)
1. Fix escape sequence processing (lines 100-110)
2. Add symlink resolution and validation
3. Implement proper absolute path security
4. Add input length and reference count limits
5. Add comprehensive security tests

### Phase 2: High Priority (Do Soon)
1. Implement proper UTF-8 handling
2. Fix error handling - either fail hard or collect warnings
3. Add security documentation
4. Improve test coverage to >80%
5. Add context support for cancellation

### Phase 3: Medium Priority (Do When Possible)
1. Add proper error types
2. Implement options builder pattern
3. Add metrics/telemetry
4. Add integration tests
5. Consider caching implementation

### Phase 4: Low Priority (Nice to Have)
1. Improve documentation with more examples
2. Optimize string building (if escape sequences are fixed)
3. Add performance tests

---

## 11. Conclusion

The parser.go file provides a solid foundation for file reference parsing, but **it is not production-ready in its current state**. The critical security vulnerabilities (path traversal, symlink attacks, DoS potential) must be addressed before deployment in any security-sensitive environment.

The code demonstrates good structure and basic security awareness, but lacks depth in edge case handling, proper escape sequence support, and comprehensive error handling. The test coverage of 60.7% is insufficient, especially given the complete absence of security-focused tests.

**Recommended action:** Implement Phase 1 fixes immediately before any production deployment. Consider this code suitable only for trusted, single-user development environments in its current state.

---

**Reviewed by:** Claude Code Analysis
**Next Review:** After Phase 1 fixes are implemented
