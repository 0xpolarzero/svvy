# Migration Guide: Rust to Go

This document provides guidance for developers familiar with the Rust version of Codex who are working on or migrating to the Go version.

## Overview

The Go rewrite maintains the same core functionality and user experience as the Rust version while leveraging Go's strengths for better maintainability and contributor accessibility.

## Why Go?

### Advantages of Go Rewrite

1. **Simpler Concurrency Model**
   - Goroutines vs complex async/await
   - Channels for communication
   - Built-in race detector

2. **Faster Build Times**
   - Significantly faster compilation
   - Quicker iteration cycles
   - Better CI/CD performance

3. **Lower Entry Barrier**
   - Easier for contributors to learn
   - Simpler dependency management
   - More straightforward error handling

4. **Better Tooling**
   - Integrated testing framework
   - Built-in profiling and tracing
   - Excellent IDE support

5. **Standard Library**
   - Rich standard library for common tasks
   - HTTP/JSON/terminal support built-in
   - Less dependency on external crates

## Key Differences

### Language-Level Differences

| Concept | Rust | Go |
|---------|------|-----|
| Error Handling | `Result<T, E>` | `(T, error)` |
| Null Safety | `Option<T>` | `*T` (nil-able pointers) |
| Generics | Full generics | Type parameters (Go 1.18+) |
| Ownership | Borrow checker | Garbage collector |
| Concurrency | async/await | goroutines/channels |
| Pattern Matching | `match` | `switch` (limited) |
| Memory Management | Manual (RAII) | Garbage collected |

### Project Structure Mapping

| Rust (codex) | Go (codex-go) | Purpose |
|--------------|---------------|---------|
| `src/main.rs` | `cmd/codex/main.go` | Entry point |
| `src/lib.rs` | `pkg/sdk/` | Public API |
| `src/protocol/` | `internal/protocol/` | Protocol types |
| `src/client/` | `internal/client/` | API client |
| `src/conversation/` | `internal/conversation/` | Session management |
| `src/tools/` | `internal/tools/` | Tool runtime |
| `src/tui/` | `internal/tui/` | Terminal UI |
| `Cargo.toml` | `go.mod` | Dependencies |
| `Cargo.lock` | `go.sum` | Lock file |

## Code Translation Examples

### Error Handling

**Rust:**
```rust
fn load_config() -> Result<Config, ConfigError> {
    let file = File::open("config.toml")?;
    let config = toml::from_reader(file)?;
    Ok(config)
}

// Usage
match load_config() {
    Ok(config) => println!("Loaded: {:?}", config),
    Err(e) => eprintln!("Error: {}", e),
}
```

**Go:**
```go
func LoadConfig() (*Config, error) {
    file, err := os.Open("config.toml")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var config Config
    if err := toml.NewDecoder(file).Decode(&config); err != nil {
        return nil, err
    }
    return &config, nil
}

// Usage
config, err := LoadConfig()
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    return
}
fmt.Printf("Loaded: %+v\n", config)
```

### Option/Null Handling

**Rust:**
```rust
fn find_user(id: u64) -> Option<User> {
    database.query(id)
}

// Usage
match find_user(123) {
    Some(user) => println!("Found: {}", user.name),
    None => println!("Not found"),
}

// Or with if-let
if let Some(user) = find_user(123) {
    println!("Found: {}", user.name);
}
```

**Go:**
```go
func FindUser(id uint64) *User {
    return database.Query(id)
}

// Usage
if user := FindUser(123); user != nil {
    fmt.Printf("Found: %s\n", user.Name)
} else {
    fmt.Println("Not found")
}
```

### Pattern Matching

**Rust:**
```rust
match event {
    Event::MessageDelta(delta) => handle_delta(delta),
    Event::ToolUse(tool) => handle_tool(tool),
    Event::Error(err) => handle_error(err),
    _ => {}
}
```

**Go:**
```go
switch event.Type {
case EventMessageDelta:
    handleDelta(event.Delta)
case EventToolUse:
    handleTool(event.Tool)
case EventError:
    handleError(event.Error)
}

// Or with type switch for interfaces
switch e := event.(type) {
case *MessageDeltaEvent:
    handleDelta(e)
case *ToolUseEvent:
    handleTool(e)
case *ErrorEvent:
    handleError(e)
}
```

### Async/Concurrency

**Rust:**
```rust
async fn fetch_completion(req: Request) -> Result<Response, Error> {
    let resp = client.post("/complete")
        .json(&req)
        .send()
        .await?;

    resp.json().await
}

// Usage with tokio
#[tokio::main]
async fn main() {
    let response = fetch_completion(req).await.unwrap();
}
```

**Go:**
```go
func FetchCompletion(req Request) (*Response, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }

    resp, err := http.Post("/complete", "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var response Response
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, err
    }
    return &response, nil
}

// Usage
func main() {
    response, err := FetchCompletion(req)
    if err != nil {
        log.Fatal(err)
    }
}

// For concurrent work
go func() {
    response, err := FetchCompletion(req)
    // handle response
}()
```

### Streaming

**Rust:**
```rust
async fn stream_events() -> impl Stream<Item = Event> {
    let stream = client.get("/stream").send().await?.bytes_stream();

    stream.map(|chunk| parse_event(chunk))
}

// Usage
let mut stream = stream_events().await;
while let Some(event) = stream.next().await {
    handle_event(event);
}
```

**Go:**
```go
func StreamEvents() (<-chan Event, error) {
    resp, err := http.Get("/stream")
    if err != nil {
        return nil, err
    }

    events := make(chan Event)
    go func() {
        defer close(events)
        defer resp.Body.Close()

        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            event := parseEvent(scanner.Bytes())
            events <- event
        }
    }()

    return events, nil
}

// Usage
events, err := StreamEvents()
if err != nil {
    log.Fatal(err)
}

for event := range events {
    handleEvent(event)
}
```

### Trait/Interface

**Rust:**
```rust
trait Tool {
    fn name(&self) -> &str;
    fn execute(&self, params: Params) -> Result<Output, Error>;
}

struct FileReader {}

impl Tool for FileReader {
    fn name(&self) -> &str {
        "file_reader"
    }

    fn execute(&self, params: Params) -> Result<Output, Error> {
        // implementation
    }
}
```

**Go:**
```go
type Tool interface {
    Name() string
    Execute(params Params) (Output, error)
}

type FileReader struct{}

func (f *FileReader) Name() string {
    return "file_reader"
}

func (f *FileReader) Execute(params Params) (Output, error) {
    // implementation
}
```

## Architectural Differences

### Memory Management

**Rust:**
- Manual memory management via ownership
- No runtime overhead
- Compile-time guarantees
- Explicit lifetimes

**Go:**
- Garbage collected
- Small runtime overhead
- Simpler to write
- No lifetime annotations

**Impact:** Go code may allocate more frequently, but GC pauses are typically <1ms. For a TUI application, this is acceptable.

### Concurrency Model

**Rust:**
- Async/await with tokio/async-std
- Complex but efficient
- Manual executor management
- Future combinators

**Go:**
- Goroutines and channels
- Simple to use
- Built-in scheduler
- CSP-style concurrency

**Impact:** Go's model is simpler for the typical patterns in Codex (streaming, background tasks).

### Error Handling Philosophy

**Rust:**
- Explicit with `Result<T, E>`
- Type-safe error propagation
- Pattern matching for handling
- Custom error types via enums

**Go:**
- Multiple return values
- Check and handle immediately
- Error wrapping for context
- `error` interface

**Impact:** Go requires more boilerplate but is more explicit about error paths.

## Feature Parity Checklist

Track progress migrating features from Rust to Go:

### Core Features

- [x] Configuration loading
- [x] API client with streaming
- [ ] Session management
- [ ] Message history
- [ ] Token counting
- [ ] Context window management

### Tools

- [ ] File operations (read, write, edit)
- [ ] Shell execution
- [ ] Patch application
- [ ] Approval workflow
- [ ] MCP tool integration

### TUI

- [ ] Chat view
- [ ] Message rendering
- [ ] Streaming updates
- [ ] Input handling
- [ ] Sidebar/status
- [ ] Image viewer
- [ ] Diff viewer

### Advanced Features

- [ ] Session resume
- [ ] History search
- [ ] Multi-conversation
- [ ] Web search rendering
- [ ] Plan updates (todo panel)

## Testing Strategy Differences

### Rust Testing

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_example() {
        let result = function_under_test();
        assert_eq!(result, expected);
    }

    #[tokio::test]
    async fn test_async() {
        let result = async_function().await;
        assert!(result.is_ok());
    }
}
```

### Go Testing

```go
func TestExample(t *testing.T) {
    result := FunctionUnderTest()
    require.Equal(t, expected, result)
}

// No special async testing needed
func TestAsync(t *testing.T) {
    events := make(chan Event)
    go StreamEvents(events)

    event := <-events
    require.NotNil(t, event)
}
```

## Dependency Management

### Rust (Cargo)

```toml
[dependencies]
tokio = { version = "1.0", features = ["full"] }
serde = { version = "1.0", features = ["derive"] }
```

```bash
cargo add tokio
cargo update
cargo build
```

### Go (Modules)

```go
// go.mod
module github.com/evmts/codex/codex-go

go 1.24

require (
    github.com/charmbracelet/bubbletea v1.2.4
    github.com/stretchr/testify v1.9.0
)
```

```bash
go get github.com/charmbracelet/bubbletea
go mod tidy
go build
```

## Build and Development

### Rust

```bash
# Build
cargo build
cargo build --release

# Test
cargo test
cargo test -- --nocapture

# Run
cargo run

# Check without building
cargo check

# Format
cargo fmt

# Lint
cargo clippy
```

### Go

```bash
# Build
go build ./cmd/codex
make build

# Test
go test ./...
make test

# Run
go run ./cmd/codex

# Format
go fmt ./...
make fmt

# Lint
golangci-lint run
make lint
```

## Performance Considerations

### What Go Does Better

- **Compilation speed**: 10-100x faster build times
- **Simplicity**: Less cognitive overhead
- **Debugging**: Easier to debug concurrent code
- **Profiling**: Built-in profiling tools

### What Rust Does Better

- **Runtime performance**: No GC pauses
- **Memory efficiency**: More control over allocations
- **Zero-cost abstractions**: No runtime overhead
- **Safety guarantees**: More compile-time checks

### For Codex TUI

Go's tradeoffs are acceptable because:
1. UI responsiveness is limited by terminal refresh rates
2. Network I/O is the bottleneck, not CPU
3. Memory usage is low regardless
4. GC pauses are imperceptible in interactive use

## Migration Tips

### For Rust Developers

1. **Embrace simplicity**: Go is intentionally simple. Don't fight it.
2. **Use interfaces**: They're more flexible than Rust traits in some ways.
3. **Trust the GC**: Don't over-optimize memory allocation.
4. **Use goroutines liberally**: They're cheap (2KB stack).
5. **Error handling**: Get comfortable with `if err != nil`.
6. **Formatting**: Run `gofmt` - style is standardized.
7. **Defer is your friend**: Use it for cleanup instead of RAII.

### Common Pitfalls

1. **Goroutine leaks**: Always ensure goroutines exit
   ```go
   ctx, cancel := context.WithCancel(context.Background())
   defer cancel() // Ensures cleanup
   ```

2. **Nil pointer dereferencing**: Check for nil
   ```go
   if user != nil {
       fmt.Println(user.Name)
   }
   ```

3. **Range variable capture**: Copy in loops
   ```go
   for _, item := range items {
       item := item // Create copy
       go process(item)
   }
   ```

4. **Shadowing errors**: Watch for `:=` vs `=`
   ```go
   // Bad - shadows outer err
   if err := foo(); err != nil {
       return err
   }

   // Good - reuses outer err
   if err = foo(); err != nil {
       return err
   }
   ```

## Resources

### Go Learning Resources

- [Tour of Go](https://tour.golang.org/)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go by Example](https://gobyexample.com/)
- [Practical Go](https://dave.cheney.net/practical-go)

### Go for Rust Developers

- [Go for Rustaceans](https://fasterthanli.me/articles/a-half-hour-to-learn-go)
- [Rust vs Go comparison](https://bitfieldconsulting.com/golang/rust-vs-go)

### Project-Specific Resources

- [Bubble Tea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Anthropic API Docs](https://docs.anthropic.com/)
- [Architecture Guide](./ARCHITECTURE.md)
- [Testing Guide](./TESTING.md)

## Getting Help

- Review existing Go code in the repository
- Check documentation in `docs/`
- Ask questions in pull requests
- Reference the Rust implementation when unclear

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.
