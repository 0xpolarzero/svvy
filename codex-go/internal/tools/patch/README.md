# Patch Tool

The `apply_patch` tool provides atomic, multi-file patching capabilities using unified diff format. It supports adding, updating, deleting, and moving files with automatic rollback on failure.

## Features

- **Unified Diff Format**: Standard unified diff format support (git diff, diff -u)
- **Atomic Operations**: All-or-nothing application with automatic rollback on any failure
- **Multi-File Patches**: Apply changes to multiple files in a single operation
- **Dry Run Mode**: Preview changes without modifying files
- **Path Safety**: Prevents path traversal attacks and restricts operations to root directory
- **Operation Types**: Add, Update, Delete, and Move file operations

## Usage

### Basic Example

```json
{
  "patch": "--- a/file.txt\n+++ b/file.txt\n@@ -1,2 +1,2 @@\n-old line\n+new line\n context"
}
```

### Arguments

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `patch` | string | Yes | - | Unified diff format patch |
| `dry_run` | bool | No | false | Preview changes without applying |
| `root` | string | No | cwd | Root directory for patch operations |
| `allow_outside_root` | bool | No | false | Allow paths outside root directory |
| `approve` | bool | No | false | Approval flag (handled by runtime) |

## Examples

### Add a New File

```json
{
  "patch": "--- /dev/null\n+++ b/newfile.txt\n@@ -0,0 +1,3 @@\n+Line 1\n+Line 2\n+Line 3\n"
}
```

**Result**: Creates `newfile.txt` with three lines.

### Update an Existing File

```json
{
  "patch": "--- a/config.json\n+++ b/config.json\n@@ -2,3 +2,3 @@\n   \"host\": \"localhost\",\n-  \"port\": 8080,\n+  \"port\": 9000,\n   \"debug\": false\n"
}
```

**Result**: Changes port from 8080 to 9000 in config.json.

### Delete a File

```json
{
  "patch": "--- a/obsolete.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-Old content\n-To be removed\n"
}
```

**Result**: Deletes `obsolete.txt`.

### Move/Rename a File

```json
{
  "patch": "--- a/old/path/file.txt\n+++ b/new/path/file.txt\n@@ -1,2 +1,2 @@\n Content line 1\n Content line 2\n"
}
```

**Result**: Moves file from `old/path/file.txt` to `new/path/file.txt`.

### Multi-File Patch

```json
{
  "patch": "--- a/file1.txt\n+++ b/file1.txt\n@@ -1 +1 @@\n-old\n+new\n--- a/file2.txt\n+++ b/file2.txt\n@@ -1 +1 @@\n-old\n+new\n"
}
```

**Result**: Updates both file1.txt and file2.txt atomically.

### Dry Run

```json
{
  "patch": "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-test\n+production\n",
  "dry_run": true
}
```

**Result**: Shows what changes would be made without modifying files.

### Custom Root Directory

```json
{
  "patch": "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n",
  "root": "/custom/project/path"
}
```

**Result**: Applies patch within `/custom/project/path` instead of current working directory.

## Unified Diff Format

The tool expects standard unified diff format, which includes:

### File Headers

```
--- a/original/path/file.txt
+++ b/new/path/file.txt
```

- `--- a/...` indicates the original file (use `/dev/null` for new files)
- `+++ b/...` indicates the new file (use `/dev/null` for deleted files)

### Hunk Headers

```
@@ -1,5 +1,6 @@
```

- `-1,5` means starting at line 1, 5 lines from original
- `+1,6` means starting at line 1, 6 lines in result

### Line Prefixes

- ` ` (space): Context line (unchanged)
- `-`: Line to be removed
- `+`: Line to be added

### Complete Example

```diff
--- a/example.go
+++ b/example.go
@@ -5,7 +5,7 @@
 package main

 import (
-	"fmt"
+	"log"
 )

 func main() {
@@ -15,5 +15,5 @@ func main() {
 	if err != nil {
-		fmt.Println("Error:", err)
+		log.Println("Error:", err)
 	}
 }
```

## Error Handling

The tool performs validation and returns errors in these cases:

### Parse Errors

- Invalid diff format
- Missing file headers
- Malformed hunk headers
- Binary files (not supported)

### Conflict Errors

- File content doesn't match expected patch context
- Line numbers out of range
- Context lines don't match

### Path Traversal

- Attempts to access files outside root directory
- Use `allow_outside_root: true` to override (use cautiously)

### File Not Found

- Attempting to update or delete non-existent files
- Missing source file for move operations

### I/O Errors

- Permission denied
- Disk full
- Directory creation failures

## Rollback

If any operation fails during multi-file patching:

1. All changes are automatically rolled back
2. Files are restored to their original state
3. Error details are included in response
4. No partial modifications are left on disk

## Response Format

Success response:

```json
{
  "content": "Files Updated (1):\n  ~ file.txt\n\nSuccessfully applied changes to 1 file(s).",
  "success": true
}
```

Dry run response:

```json
{
  "content": "DRY RUN MODE - No files were actually modified\n\nFiles Updated (1):\n  ~ file.txt\n\nWould affect 1 file(s). Run without dry_run to apply changes.",
  "success": true
}
```

Error response:

```json
{
  "content": "Errors (1):\n  ! conflict in file.txt: context mismatch at line 5\n\nAll changes have been rolled back due to errors.",
  "success": false
}
```

## Security

### Path Traversal Protection

By default, the tool prevents accessing files outside the root directory:

```
✅ Allowed: "src/file.txt"
✅ Allowed: "dir/subdir/file.txt"
❌ Blocked: "../etc/passwd"
❌ Blocked: "/etc/passwd"
❌ Blocked: "safe/../../etc/passwd"
```

### Sandbox Preference

The patch tool has `SandboxForbid` preference as it requires direct filesystem access for atomic operations. It should be used with appropriate approval policies.

### Approval Policies

Recommended approval policy: `ApprovalUnlessTrusted` or `ApprovalOnRequest` as the tool modifies files.

## Integration

### Tool Registry

```go
import (
    "github.com/evmts/codex/codex-go/internal/tools/patch"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
    "github.com/spf13/afero"
)

registry := runtime.NewToolRegistry()
fs := afero.NewOsFs()
patchTool := patch.NewPatchTool(fs)
registry.Register(patchTool)
```

### Testing with In-Memory Filesystem

```go
import (
    "github.com/evmts/codex/codex-go/test"
)

fs := test.NewMemFS(t)
patchTool := patch.NewPatchTool(fs)

// Test without touching real filesystem
```

## Generating Patches

You can generate unified diffs using standard tools:

### Git

```bash
git diff > changes.patch
git diff --no-prefix > changes.patch  # Without a/ b/ prefixes
git diff HEAD~1 HEAD > changes.patch   # Between commits
```

### Diff Command

```bash
diff -u original.txt modified.txt > changes.patch
diff -urN old_dir/ new_dir/ > changes.patch  # Recursive
```

### Programmatically

```go
import "github.com/sergi/go-diff/diffmatchpatch"

dmp := diffmatchpatch.New()
diffs := dmp.DiffMain(original, modified, false)
patch := dmp.PatchMake(original, diffs)
text := dmp.PatchToText(patch)
```

## Limitations

1. **Binary Files**: Not supported - patches must be text files
2. **File Permissions**: Permissions are not preserved (set to 0644)
3. **Symlinks**: Not followed by default (path validation prevents escaping root)
4. **Large Files**: No specific size limits, but memory-constrained by reading entire file
5. **Concurrent Access**: Tool does not support parallel execution to prevent conflicts

## Best Practices

1. **Always test with dry_run first** to preview changes
2. **Keep patches focused** - smaller patches are easier to debug
3. **Use version control** - commit before applying patches
4. **Validate context** - ensure files match expected state before patching
5. **Handle errors gracefully** - check response for rollback information
6. **Be cautious with allow_outside_root** - only use when absolutely necessary

## Architecture

### Components

- **parser.go**: Parses unified diff format into structured FilePatch objects
- **apply.go**: Applies patches atomically with rollback support
- **errors.go**: Custom error types for detailed error reporting
- **patch.go**: Main tool runtime implementation

### Atomic Application

1. Parse and validate all patches
2. Create backups of all affected files
3. Apply each patch sequentially
4. On any error, rollback all changes using backups
5. Use temp file + atomic rename for each write

### Testing

Comprehensive test coverage includes:
- Parser validation for all operation types
- Atomic application with rollback
- Path traversal protection
- Conflict detection and error handling
- Multi-file scenarios
- Edge cases (empty files, large files, etc.)

All tests use in-memory filesystem (afero) for speed and isolation.
