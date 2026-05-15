# Code Review: rollout.go

**File:** `/Users/williamcory/codex/codex-go/internal/history/persistence/rollout.go`
**Review Date:** 2025-10-26
**Lines of Code:** 189

---

## Executive Summary

This file implements rollout (snapshot) management for conversation history files. The code is generally well-structured with good test coverage, but there are several areas that need attention including error handling edge cases, potential race conditions, missing validation, and unclear behavior in certain scenarios.

**Overall Quality Rating:** 7/10

---

## 1. Incomplete Features or Functionality

### 1.1 Missing Restore Functionality
**Severity:** Medium

The file provides `CreateRollout`, `ListRollouts`, `DeleteRollout`, `CleanupOldRollouts`, and `GetLatestRollout`, but conspicuously **lacks a `RestoreRollout` function** to actually restore a rollout back to the main history file.

**Impact:** The rollout system can create and manage snapshots, but there's no way to actually use them for their intended purpose - recovering from a bad state or rolling back to a previous version.

**Recommendation:** Implement a `RestoreRollout(fs afero.Fs, rolloutPath, historyPath string) error` function that:
- Validates the rollout file exists and is readable
- Creates a backup of the current history before restoring
- Atomically replaces the history file with the rollout content
- Maintains proper file permissions (SensitiveFileMode)

### 1.2 No Rollout Validation
**Severity:** Medium

There's no validation that rollout files contain valid JSONL data or can be successfully parsed. A corrupted rollout could go undetected until an attempted restore.

**Recommendation:** Add a `ValidateRollout(fs afero.Fs, rolloutPath string) error` function that:
- Checks file integrity
- Validates JSONL format
- Ensures the file can be parsed
- Optionally returns metadata (number of entries, size, etc.)

### 1.3 No Metadata Storage
**Severity:** Low

Rollouts only store a timestamp in the filename. There's no metadata about:
- Why the rollout was created
- What version/state it represents
- How many entries it contains
- File checksum for integrity verification

**Recommendation:** Consider storing metadata in a sidecar file (e.g., `history.jsonl.1234567890.meta`) with JSON containing creation reason, entry count, checksum, etc.

---

## 2. TODO Comments and Technical Debt

### 2.1 No Explicit TODOs
**Status:** Clean

The file contains no TODO comments or explicit technical debt markers. However, implicit technical debt exists (see other sections).

---

## 3. Code Quality Issues

### 3.1 Inconsistent Error Handling in `CleanupOldRollouts`
**Severity:** Medium
**Location:** Lines 113-133

```go
func CleanupOldRollouts(fs afero.Fs, historyPath string, keepCount int) error {
    rollouts, err := ListRollouts(fs, historyPath)
    if err != nil {
        return err  // ← Wrapped error from ListRollouts
    }

    // ...

    for i := 0; i < deleteCount; i++ {
        if err := DeleteRollout(fs, rollouts[i]); err != nil {
            return fmt.Errorf("failed to delete rollout %s: %w", rollouts[i], err)  // ← Stops on first failure
        }
    }

    return nil
}
```

**Issues:**
1. **Stops on first failure:** If deleting one rollout fails, the function returns immediately, leaving some old rollouts in place. This creates an inconsistent state.
2. **No transactional guarantee:** Partial cleanup can occur, which may be confusing to users.
3. **Missing context:** The error from `ListRollouts` on line 116 is returned directly without wrapping, unlike other functions.

**Recommendation:**
```go
// Option 1: Continue on error and return aggregated errors
var deleteErrors []error
for i := 0; i < deleteCount; i++ {
    if err := DeleteRollout(fs, rollouts[i]); err != nil {
        deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete %s: %w", rollouts[i], err))
    }
}
if len(deleteErrors) > 0 {
    return fmt.Errorf("cleanup incomplete: %v", deleteErrors)
}

// Option 2: Use best-effort approach and log warnings instead
```

### 3.2 Unclear Behavior with Negative `keepCount`
**Severity:** Low
**Location:** Line 120

```go
deleteCount := len(rollouts) - keepCount
if deleteCount <= 0 {
    return nil
}
```

**Issue:** If `keepCount` is negative, `deleteCount` becomes very large (e.g., `5 - (-3) = 8`), but the code would try to delete more rollouts than exist. The slice indexing would panic.

**Current Mitigation:** Test `TestCleanupOldRolloutsKeepNone` tests `keepCount=0` but not negative values.

**Recommendation:**
```go
func CleanupOldRollouts(fs afero.Fs, historyPath string, keepCount int) error {
    if keepCount < 0 {
        return fmt.Errorf("keepCount must be non-negative, got %d", keepCount)
    }
    // ... rest of function
}
```

### 3.3 Silent Failure in Sort Fallback
**Severity:** Low
**Location:** Lines 80-90

```go
sort.Slice(rollouts, func(i, j int) bool {
    bi := filepath.Base(rollouts[i])
    bj := filepath.Base(rollouts[j])
    ti, errI := parseRolloutTimestamp(bi)
    tj, errJ := parseRolloutTimestamp(bj)
    if errI != nil || errJ != nil {
        // Fallback to lexicographic order if parsing fails
        return bi < bj
    }
    return ti < tj
})
```

**Issues:**
1. If timestamp parsing fails, files are sorted lexicographically without any indication of the problem
2. This could mask corrupted rollout filenames that passed the initial `isRolloutFile` check
3. Lexicographic sorting might not produce the correct chronological order

**Recommendation:**
- Log warnings when timestamp parsing fails during sorting
- Consider returning an error if any rollout filenames can't be parsed
- Add a comment explaining why lexicographic fallback is acceptable

### 3.4 Potential Race Condition in `CreateRollout`
**Severity:** Medium
**Location:** Lines 19-44

```go
func CreateRollout(fs afero.Fs, historyPath string) (string, error) {
    exists, err := afero.Exists(fs, historyPath)
    if err != nil {
        return "", fmt.Errorf("failed to check if file exists: %w", err)
    }
    if !exists {
        return "", fmt.Errorf("history file does not exist: %s", historyPath)
    }

    rolloutPath := rolloutFilename(historyPath)  // ← Uses time.Now()

    data, err := afero.ReadFile(fs, historyPath)
    if err != nil {
        return "", fmt.Errorf("failed to read history file: %w", err)
    }

    if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
        return "", fmt.Errorf("failed to write rollout file: %w", err)
    }

    return rolloutPath, nil
}
```

**Issues:**
1. **TOCTOU (Time-of-check Time-of-use):** The history file could be deleted between the `Exists` check (line 21) and `ReadFile` call (line 33)
2. **No atomicity:** If the process crashes between reading and writing, the rollout is lost
3. **No verification:** Rollout filename collision (though unlikely with nanosecond timestamps) is not checked
4. **History file modification:** If the history file is being written to during the read, the rollout might capture an inconsistent state

**Recommendation:**
```go
// Option 1: Remove the Exists check and just try to read
// (ReadFile will fail anyway if file doesn't exist)
data, err := afero.ReadFile(fs, historyPath)
if err != nil {
    if os.IsNotExist(err) {
        return "", fmt.Errorf("history file does not exist: %s", historyPath)
    }
    return "", fmt.Errorf("failed to read history file: %w", err)
}

// Option 2: Use file locking if available (integrate with filelock.go)
// to ensure consistent read during active writes
```

### 3.5 Missing Concurrency Protection
**Severity:** High
**Location:** Entire file

**Issue:** None of the functions have any concurrency protection. If two goroutines call `CreateRollout` simultaneously:
- They could generate the same timestamp (unlikely but possible on fast systems)
- One could overwrite the other's rollout
- `CleanupOldRollouts` called concurrently could have undefined behavior

The `persistence.go` wrapper calls `writer.Flush()` before `CreateRollout`, but there's no guarantee that another goroutine isn't writing to the history file during the rollout creation.

**Recommendation:**
- Document that these functions are NOT goroutine-safe
- Use the existing `HistoryWriter`'s mutex when creating rollouts through the persistence layer
- Consider adding file locking (similar to writer.go) for cross-process safety
- Or clearly document that rollout operations should only be performed when no writes are occurring

### 3.6 `parseRolloutTimestamp` Assumes Specific Format
**Severity:** Low
**Location:** Lines 162-176

```go
func parseRolloutTimestamp(filename string) (int64, error) {
    parts := strings.Split(filename, ".")
    if len(parts) < 3 {  // ← Assumes at least 3 parts: name.ext.timestamp
        return 0, fmt.Errorf("invalid rollout filename format: %s", filename)
    }

    timestampStr := parts[len(parts)-1]
    timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid timestamp in rollout filename: %s", filename)
    }

    return timestamp, nil
}
```

**Issue:** This assumes the base filename has at least one dot (e.g., `history.jsonl`). If someone used a filename without an extension like `history`, then `history.1234567890` would only have 2 parts and fail validation.

**Impact:** Low, since this codebase specifically uses `.jsonl` files, but it's a hidden assumption.

**Recommendation:**
```go
if len(parts) < 2 {  // Only require basename + timestamp
    return 0, fmt.Errorf("invalid rollout filename format: %s", filename)
}
```

Or document the requirement that base filenames must contain at least one dot.

---

## 4. Missing Test Coverage

### 4.1 Concurrent Access Tests
**Status:** Missing
**Severity:** High

**Missing Tests:**
- Multiple goroutines creating rollouts simultaneously
- Creating rollout while history writer is active
- Listing/deleting rollouts while another operation is in progress
- Cleanup racing with rollout creation

### 4.2 Filesystem Error Scenarios
**Status:** Partially Missing
**Severity:** Medium

**Covered:**
- ✅ Non-existent history file (`TestCreateRolloutNonExistentFile`)
- ✅ Non-existent directory (`TestListRolloutsNonExistentDirectory`)
- ✅ Non-existent rollout deletion (`TestDeleteRolloutNonExistent`)

**Missing:**
- ❌ Read-only filesystem (cannot write rollout)
- ❌ Disk full scenario
- ❌ Permission denied on rollout directory
- ❌ Corrupted/truncated history file during rollout creation
- ❌ History file changes size/content during `CreateRollout` execution
- ❌ Rollout write succeeds but with wrong permissions

### 4.3 Edge Cases
**Status:** Partially Missing
**Severity:** Medium

**Missing Tests:**
- ❌ Negative `keepCount` in `CleanupOldRollouts`
- ❌ Very large `keepCount` (e.g., `keepCount=1000000`)
- ❌ Cleanup while some rollouts are corrupted/unreadable
- ❌ Rollout filenames with special characters
- ❌ Extremely long filenames
- ❌ Timestamp collision (two rollouts with same timestamp)
- ❌ Files matching rollout pattern but not created by this package

### 4.4 Partial Failure Tests
**Status:** Missing
**Severity:** Medium

**Missing Tests:**
- ❌ `CleanupOldRollouts` fails midway through deletion loop
- ❌ `DeleteRollout` succeeds but leaves orphaned metadata
- ❌ `ListRollouts` with some valid and some invalid rollout filenames
- ❌ Sort comparison function behavior when parsing fails for one but not both items

### 4.5 Integration Tests
**Status:** Missing
**Severity:** Low

**Missing:**
- ❌ Real filesystem test (only `TestRolloutFilePermissions` uses OS filesystem)
- ❌ Large file rollout (e.g., 100MB+ history file)
- ❌ Many rollouts (e.g., 1000+ rollouts)
- ❌ Cross-platform permission tests (Windows vs Unix)

---

## 5. Potential Bugs and Edge Cases

### 5.1 Timestamp Collision
**Severity:** Low
**Probability:** Very Low

**Scenario:** If two `CreateRollout` calls happen within the same nanosecond (or if `time.Now()` returns the same value), the second call will **overwrite** the first rollout.

**Code:**
```go
rolloutPath := rolloutFilename(historyPath)  // Uses time.Now().UnixNano()
// ... no check if rolloutPath already exists ...
if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
```

**Mitigation:**
```go
func rolloutFilename(basePath string) string {
    for {
        timestamp := time.Now().UnixNano()
        path := rolloutPathWithTimestamp(basePath, timestamp)
        // Check if file already exists
        if exists, _ := afero.Exists(fs, path); !exists {
            return path
        }
        time.Sleep(1 * time.Nanosecond)  // Wait and retry
    }
}
```

Note: This would require passing `fs` to the function or restructuring the code.

### 5.2 `GetLatestRollout` Ambiguity
**Severity:** Low
**Location:** Lines 136-148

**Issue:** If timestamp parsing fails during sorting in `ListRollouts`, files fall back to lexicographic order. This means `GetLatestRollout` might not actually return the chronologically latest rollout.

**Example:**
```
history.jsonl.1000000000  (valid timestamp)
history.jsonl.corrupted   (invalid, sorts last lexicographically)
```
Would return `history.jsonl.corrupted` as "latest" even though it's not a valid rollout.

**Mitigation:**
- `ListRollouts` should filter out invalid rollouts OR
- `GetLatestRollout` should validate the returned rollout OR
- Document this behavior clearly

### 5.3 Large File Performance
**Severity:** Medium
**Location:** Line 33

```go
data, err := afero.ReadFile(fs, historyPath)
```

**Issue:** The entire history file is read into memory. For long-running sessions with thousands of conversation turns, this could consume significant memory (potentially 10s-100s of MB).

**Impact:**
- Memory spike during rollout creation
- Potential OOM on resource-constrained systems
- Poor performance for large files

**Recommendation:**
Use streaming copy instead:
```go
func CreateRollout(fs afero.Fs, historyPath string) (string, error) {
    // ... existence check ...

    rolloutPath := rolloutFilename(historyPath)

    // Open source for reading
    src, err := fs.Open(historyPath)
    if err != nil {
        return "", fmt.Errorf("failed to open history file: %w", err)
    }
    defer src.Close()

    // Open destination for writing
    dst, err := fs.OpenFile(rolloutPath, os.O_CREATE|os.O_WRONLY, SensitiveFileMode)
    if err != nil {
        return "", fmt.Errorf("failed to create rollout file: %w", err)
    }
    defer dst.Close()

    // Stream copy
    if _, err := io.Copy(dst, src); err != nil {
        fs.Remove(rolloutPath)  // Cleanup on failure
        return "", fmt.Errorf("failed to copy data: %w", err)
    }

    return rolloutPath, nil
}
```

### 5.4 Directory Traversal Vulnerability
**Severity:** Low
**Location:** Multiple functions

**Issue:** The code doesn't validate that `historyPath` is a safe path. A malicious input like `../../../../etc/passwd` could cause rollouts to be created in unexpected locations.

**Current Mitigation:** This is likely validated at a higher layer, but defense in depth is valuable.

**Recommendation:**
```go
func validatePath(path string) error {
    clean := filepath.Clean(path)
    if !filepath.IsAbs(clean) {
        return fmt.Errorf("path must be absolute: %s", path)
    }
    // Could add additional checks like blocking /etc, /sys, etc.
    return nil
}
```

### 5.5 Incomplete Cleanup on Write Failure
**Severity:** Low
**Location:** Line 39-41

```go
if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
    return "", fmt.Errorf("failed to write rollout file: %w", err)
}
```

**Issue:** If `WriteFile` partially succeeds (e.g., writes some data then fails), the incomplete rollout file is left on disk. This could fill up disk space over time if failures are repeated.

**Recommendation:**
```go
if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
    // Best-effort cleanup of partial file
    _ = fs.Remove(rolloutPath)
    return "", fmt.Errorf("failed to write rollout file: %w", err)
}
```

---

## 6. Documentation Issues

### 6.1 Missing Package-Level Documentation
**Severity:** Medium

The file lacks a package-level comment explaining:
- What rollouts are and why they exist
- When rollouts should be created
- Lifecycle of rollouts (create → use → cleanup)
- Relationship between rollouts and history files
- Thread-safety guarantees (or lack thereof)

**Recommendation:**
Add comprehensive package documentation at the top of the file or in a `doc.go`.

### 6.2 Insufficient Function Documentation
**Severity:** Low

Some functions lack important details:

**`CreateRollout` (Line 14-18):**
- ✅ Describes what it does
- ❌ Doesn't mention it's NOT atomic
- ❌ Doesn't mention thread-safety concerns
- ❌ Doesn't explain rollout file permissions
- ❌ Doesn't warn about large file memory usage

**`ListRollouts` (Line 46-47):**
- ✅ Describes basic behavior
- ❌ Doesn't explain what happens with invalid rollout files
- ❌ Doesn't mention the lexicographic fallback

**`CleanupOldRollouts` (Line 112):**
- ✅ Basic description
- ❌ Doesn't explain behavior when `keepCount` is 0 or negative
- ❌ Doesn't mention it stops on first deletion failure
- ❌ Doesn't clarify if "most recent" means by timestamp or creation order

**`GetLatestRollout` (Line 135):**
- ✅ Basic description
- ❌ Should document that it returns an error if no rollouts exist
- ❌ Should explain what "most recent" means

### 6.3 Missing Usage Examples in Code
**Severity:** Low

While `example_test.go` has an example, inline examples in the function documentation would be helpful:

```go
// CreateRollout creates a timestamped snapshot of the history file.
//
// Example:
//     fs := afero.NewOsFs()
//     rolloutPath, err := CreateRollout(fs, "/home/user/.codex/session1/history.jsonl")
//     // rolloutPath: "/home/user/.codex/session1/history.jsonl.1234567890123456789"
//
// The rollout is an exact copy of the history file at the time of creation.
// ...
```

### 6.4 Undocumented Helper Functions
**Severity:** Low

Functions `rolloutFilename`, `rolloutPathWithTimestamp`, `parseRolloutTimestamp`, and `isRolloutFile` have no documentation, making it harder to understand the rollout naming scheme.

**Recommendation:**
Add brief comments:
```go
// rolloutFilename generates a rollout filename with the current timestamp.
// Format: {basePath}.{unixNanoTimestamp}
// Example: "/path/history.jsonl" -> "/path/history.jsonl.1698765432000000000"
func rolloutFilename(basePath string) string {
```

### 6.5 Missing Error Context
**Severity:** Low

Some error messages could be more helpful:

```go
// Current:
return "", fmt.Errorf("history file does not exist: %s", historyPath)

// Better:
return "", fmt.Errorf("cannot create rollout: history file does not exist: %s", historyPath)
```

This provides better context when errors bubble up through the call stack.

---

## 7. Security Concerns

### 7.1 File Permission Verification
**Severity:** Low
**Status:** Partially Addressed

**Good:**
- Rollouts are created with `SensitiveFileMode (0600)` on line 39
- This is tested in `TestRolloutFilePermissions`

**Issue:**
- No verification that the write actually resulted in correct permissions
- On some filesystems or with certain afero implementations, permissions might not be honored
- No audit/logging when rollouts are created or accessed

**Recommendation:**
```go
if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
    return "", fmt.Errorf("failed to write rollout file: %w", err)
}

// Verify permissions were set correctly (for OS filesystem)
if _, ok := fs.(*afero.OsFs); ok {
    info, err := fs.Stat(rolloutPath)
    if err != nil {
        _ = fs.Remove(rolloutPath)
        return "", fmt.Errorf("failed to verify rollout permissions: %w", err)
    }
    if info.Mode().Perm() != SensitiveFileMode {
        _ = fs.Remove(rolloutPath)
        return "", fmt.Errorf("rollout created with incorrect permissions: %o", info.Mode().Perm())
    }
}
```

### 7.2 Information Disclosure via Rollout Filenames
**Severity:** Very Low

**Issue:** Rollout filenames include timestamps that could reveal:
- When conversations occurred
- Frequency of conversations
- System timezone (if timestamps are displayed in local time)

**Mitigation:**
- Using Unix nanoseconds minimizes information leakage
- Timestamps are necessary for proper ordering
- File permissions (0600) prevent unauthorized access to filenames in directory listings (on Unix systems)

**Recommendation:** Document this as acceptable risk, as the benefits outweigh the minimal information disclosure.

### 7.3 Sensitive Data in Error Messages
**Severity:** Low

**Issue:** Error messages include full file paths which could leak information about directory structure:

```go
return "", fmt.Errorf("history file does not exist: %s", historyPath)
```

In logs, this could reveal:
- Username
- Session IDs
- Directory structure

**Recommendation:**
- Sanitize paths in error messages for logging
- Or document that error messages should be handled carefully (not sent to telemetry, etc.)

### 7.4 No Checksum/Integrity Verification
**Severity:** Low

**Issue:** Rollouts are created and stored with no integrity verification. A bit flip, disk corruption, or tampering could go undetected.

**Recommendation:**
- Store SHA256 hash in metadata
- Verify hash before using rollout
- Especially important if rollouts will be used for auditing or compliance

### 7.5 Rollout Directory Access
**Severity:** Very Low

**Issue:** While rollout files have 0600 permissions, the parent directory permissions are managed elsewhere (`SensitiveDirMode` in writer.go). If the directory has overly permissive permissions, attackers could:
- List rollout filenames (revealing timestamps)
- Delete rollout files (DoS attack)
- Replace rollout files with malicious ones (if they can bypass file permissions)

**Mitigation:** The `writer.go` already creates directories with 0700, but this isn't verified in rollout.go.

**Recommendation:** Document the dependency on proper directory permissions or add validation.

---

## 8. Performance Considerations

### 8.1 Memory Usage in `CreateRollout`
**Severity:** Medium
**Already Discussed:** Section 5.3

### 8.2 Inefficient Sorting
**Severity:** Low
**Location:** Lines 80-90

**Issue:** Sorting calls `filepath.Base()` twice per comparison and `parseRolloutTimestamp()` twice per comparison. For N rollouts, this is O(N log N) comparisons with expensive string operations each time.

**Optimization:**
```go
// Pre-compute sort keys
type rolloutInfo struct {
    path      string
    timestamp int64
    parseErr  error
}

rolloutInfos := make([]rolloutInfo, len(rollouts))
for i, path := range rollouts {
    base := filepath.Base(path)
    ts, err := parseRolloutTimestamp(base)
    rolloutInfos[i] = rolloutInfo{path, ts, err}
}

sort.Slice(rolloutInfos, func(i, j int) bool {
    if rolloutInfos[i].parseErr != nil || rolloutInfos[j].parseErr != nil {
        return rolloutInfos[i].path < rolloutInfos[j].path
    }
    return rolloutInfos[i].timestamp < rolloutInfos[j].timestamp
})

// Extract sorted paths
for i, info := range rolloutInfos {
    rollouts[i] = info.path
}
```

Impact: Moderate improvement when listing 100+ rollouts.

### 8.3 `CleanupOldRollouts` Calls `ListRollouts`
**Severity:** Low
**Location:** Line 114

**Issue:** `ListRollouts` reads the entire directory, parses all filenames, sorts them, etc. If cleanup is called frequently or the directory has many non-rollout files, this could be expensive.

**Mitigation:** Already optimal - we need the sorted list to know which are oldest.

**Recommendation:** Document that cleanup should be called judiciously (e.g., after creating a rollout, not on every operation).

---

## 9. Architectural Concerns

### 9.1 No Abstraction for Rollout Storage
**Severity:** Low

All rollout logic is filesystem-based. If in the future rollouts should be stored elsewhere (S3, database, etc.), significant refactoring would be needed.

**Recommendation:** Consider introducing a `RolloutStore` interface:
```go
type RolloutStore interface {
    Create(historyPath string) (string, error)
    List(historyPath string) ([]string, error)
    Delete(rolloutPath string) error
    GetLatest(historyPath string) (string, error)
}
```

Then implement `FilesystemRolloutStore` for the current behavior.

### 9.2 Tight Coupling with Filename Format
**Severity:** Low

The rollout naming scheme (`basename.timestamp`) is hardcoded throughout. Changing the format would require updates in multiple functions.

**Recommendation:** Centralize the format:
```go
const rolloutExtension = ".rollout"

func formatRolloutPath(basePath string, timestamp int64) string {
    return fmt.Sprintf("%s%s.%d", basePath, rolloutExtension, timestamp)
}
```

### 9.3 No Versioning
**Severity:** Very Low

If the rollout format changes in the future (e.g., compression, encryption), old rollouts might not be compatible with new code.

**Recommendation:** Include a version indicator in the filename or metadata:
```
history.jsonl.v1.1234567890  // Version 1 rollout
```

---

## 10. Testing Recommendations

### Priority 1 (High)
1. **Concurrency tests:** Multiple goroutines creating/deleting rollouts simultaneously
2. **Negative `keepCount` test:** Verify proper error handling
3. **Partial cleanup failure:** Test behavior when deletion fails midway
4. **Large file test:** Create rollout of 50MB+ file, verify memory usage

### Priority 2 (Medium)
5. **Filesystem errors:** Simulate disk full, permission denied, read-only filesystem
6. **Corrupted rollout:** Place invalid file matching rollout pattern, test behavior
7. **Timestamp collision:** Mock `time.Now()` to return same value twice
8. **Long filename test:** Test with very long history paths

### Priority 3 (Low)
9. **Cross-platform:** Test on Windows, macOS, Linux
10. **Performance benchmark:** Benchmark with 1000+ rollouts
11. **Integration test:** Full lifecycle from writer perspective

---

## 11. Positive Aspects

Despite the issues above, the code has many strengths:

1. **Clear naming:** Function and variable names are descriptive and follow Go conventions
2. **Good test coverage:** Most happy paths are well tested
3. **Proper error wrapping:** Uses `fmt.Errorf` with `%w` for error chains
4. **Security consciousness:** Uses restrictive file permissions
5. **Clean code structure:** Functions are reasonably sized and focused
6. **No premature optimization:** Code is straightforward and readable
7. **Consistent style:** Follows Go idioms throughout
8. **Defensive programming:** Checks for nil/empty cases
9. **Good documentation:** Most public functions have documentation comments
10. **Abstraction with afero:** Makes testing easy with mock filesystems

---

## 12. Recommendations Summary

### Must Fix (High Priority)
1. Add concurrency protection or clearly document lack of thread-safety
2. Implement `RestoreRollout` functionality
3. Fix potential memory issue in `CreateRollout` for large files
4. Add validation for negative/invalid `keepCount` values
5. Improve error handling in `CleanupOldRollouts` (don't stop on first failure)

### Should Fix (Medium Priority)
6. Add rollout validation functionality
7. Remove TOCTOU issue in `CreateRollout`
8. Improve documentation (especially package-level)
9. Add missing test coverage (see Section 4)
10. Handle timestamp collision edge case

### Nice to Have (Low Priority)
11. Add rollout metadata support
12. Optimize sorting performance
13. Consider abstraction layer for future extensibility
14. Add versioning to rollout format
15. Improve error message context

---

## 13. Conclusion

The `rollout.go` file provides a solid foundation for snapshot management, but has several gaps that should be addressed before considering it production-ready for critical use cases. The most significant concerns are:

1. **Missing restore functionality** - defeats the purpose of rollouts
2. **Lack of concurrency safety** - could lead to data corruption
3. **Memory inefficiency for large files** - scalability concern
4. **Incomplete error handling** - could leave system in inconsistent state

The code quality is generally good, with clear structure and decent test coverage. With the recommended fixes, this would be a robust and reliable rollout management system.

**Recommended Action Plan:**
1. Week 1: Implement `RestoreRollout` and add concurrency protection
2. Week 2: Fix memory usage and improve error handling
3. Week 3: Add missing tests and improve documentation
4. Week 4: Address lower-priority improvements

---

**Reviewer Note:** This review assumes the code is intended for production use in a system handling potentially sensitive conversation data. If this is an internal tool or prototype, some concerns (especially around edge cases and security) may be less critical.
