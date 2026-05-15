# File References with @ Syntax

Codex Go now supports referencing files directly in your input using the `@` syntax, similar to the Rust implementation of Codex.

## Overview

You can reference files in your repository by prefixing the file path with `@`. Codex will automatically read and include the file content in your conversation, allowing the AI to understand and work with your code.

## Usage

### Basic Syntax

```
Check @main.go for errors
```

### Multiple Files

```
Compare @src/app.go and @test/app_test.go
```

### File Paths with Spaces

For file paths containing spaces, wrap the path in double quotes:

```
Review @"file with spaces.txt"
```

### Relative and Absolute Paths

- **Relative paths**: `@src/main.go` (relative to current working directory)
- **Absolute paths**: `@/Users/username/project/main.go`

## How It Works

1. **Parsing**: When you submit a message with `@` references, Codex parses the input to extract file paths
2. **Validation**: Each file path is validated to ensure it exists and is readable
3. **Content Inclusion**: File content is read and formatted for the LLM
4. **Context**: The file content is included in the conversation with clear markers:
   ```
   [File: main.go]
   <file content here>
   [End File: main.go]
   ```

## Security Features

### Path Validation

- **Path traversal prevention**: Attempts to use `../` to escape the working directory are blocked
- **File existence check**: Only existing files can be referenced
- **Read permission check**: Files must be readable by the current user
- **File size limits**: Configurable maximum file size (default: 10MB)

### Gitignore Support

The file search functionality (used for autocomplete) respects `.gitignore` patterns to avoid suggesting:
- Build artifacts
- Dependencies (e.g., `node_modules/`)
- Temporary files
- Sensitive files

## Implementation Details

### Components

1. **Input Parser** (`internal/input/parser.go`)
   - Extracts `@` references from user input
   - Validates file paths
   - Returns structured file references

2. **File Search** (`internal/filesearch/search.go`)
   - Fuzzy file search for autocomplete
   - Respects `.gitignore` patterns
   - Parallel search with configurable workers

3. **Protocol Integration** (`internal/protocol/protocol.go`)
   - `UserInput` type supports `path` field
   - Constants for input types: `text`, `image_url`, `path`

4. **Turn Processing** (`internal/conversation/manager/turn.go`)
   - Reads file content from paths
   - Formats content for LLM consumption
   - Handles errors gracefully

5. **TUI Integration** (`cmd/codex/tui/app.go`)
   - Parses `@` references before submission
   - Creates protocol messages with file references

### Protocol Format

When a file is referenced, the protocol message includes both text and path items:

```json
{
  "items": [
    {
      "type": "text",
      "text": "Check [file: main.go] for errors"
    },
    {
      "type": "path",
      "path": "/absolute/path/to/main.go"
    }
  ]
}
```

### File Content Formatting

File content is formatted with clear markers:

```
[File: main.go]
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
[End File: main.go]
```

## Examples

### Reviewing Code

```
Review @src/auth.go and suggest improvements
```

### Debugging

```
I'm getting an error, check @main.go and @go.mod
```

### Comparing Implementations

```
Compare the implementation in @rust/main.rs with @go/main.go
```

### Working with Tests

```
Write tests for the functions in @src/calculator.go
```

## Future Enhancements

### Autocomplete (Planned)

The file search infrastructure is in place to support real-time autocomplete:
- Type `@` to trigger file search
- Fuzzy matching as you type
- Results sorted by relevance
- Keyboard navigation through results

### Directory References (Planned)

Support for referencing entire directories:
```
Review all files in @src/
```

### File Range Selection (Planned)

Support for referencing specific line ranges:
```
Check @main.go:10-20 for the bug
```

## Troubleshooting

### File Not Found

If you get a "file not found" error:
1. Check the file path is correct
2. Ensure the file exists in your working directory
3. Use absolute path if relative path doesn't work

### Permission Denied

If you get a "permission denied" error:
1. Check file permissions: `ls -l filename`
2. Ensure the file is readable: `chmod +r filename`

### Path Traversal Blocked

If you get a "path traversal not allowed" error:
1. Avoid using `../` in file paths
2. Use absolute paths for files outside working directory
3. Check that `AllowAbsolutePaths` is enabled in validation options

## Technical Notes

### Performance

- **Parallel processing**: File search uses multiple workers (default: 2)
- **Timeout**: Search operations timeout after 2 seconds
- **Caching**: No caching currently implemented (files are read on each reference)

### Limitations

- Maximum file size: 10MB (configurable)
- Binary files: Included as-is (may not be useful for LLM)
- Large files: May exceed LLM context window
- No incremental updates: Files are read in full each time

### Comparison with Rust Implementation

The Go implementation matches the Rust implementation in:
- ✅ `@` syntax for file references
- ✅ Quoted paths for files with spaces
- ✅ File content formatting
- ✅ Path validation and security checks
- ✅ Fuzzy file search infrastructure

Differences from Rust implementation:
- ⏳ Autocomplete UI (not yet implemented in TUI)
- ⏳ Debounced search (infrastructure ready, not integrated with TUI)
- ⏳ Image file detection (not yet implemented)
