// Package progress provides progress tracking for long-running operations in Codex.
//
// # Overview
//
// The progress package enables the Go implementation to emit progress events for operations
// that take significant time, providing user feedback similar to the Rust implementation.
// This improves the user experience by showing real-time progress for:
//   - File reading/writing (large files)
//   - Patch application (large diffs)
//   - MCP tool discovery
//   - History reconstruction
//
// # Architecture
//
// The package follows a reporter pattern with three main components:
//
// 1. Progress Events (progress.go):
//   - StartedEvent: Emitted when an operation begins
//   - ProgressEvent: Emitted periodically with current/total/percentage
//   - CompletedEvent: Emitted when operation finishes with status
//
// 2. Progress Tracker (tracker.go):
//   - Manages lifecycle of a single operation
//   - Handles throttling of progress updates
//   - Automatic completion reporting
//   - Thread-safe for concurrent access
//
// 3. Reporters:
//   - Reporter interface: Abstract progress event emission
//   - ProtocolReporter: Bridges to protocol event queue
//   - NoOpReporter: Used when progress tracking is disabled
//
// # Configuration
//
// Progress tracking is controlled via Config:
//
//	config := progress.DefaultConfig()
//	config.Enabled = true                         // Enable/disable tracking
//	config.MinDuration = 100 * time.Millisecond   // Only track ops > 100ms
//	config.UpdateInterval = 250 * time.Millisecond // Throttle update frequency
//
// # Usage Examples
//
// ## Basic Progress Tracking
//
//	tracker := progress.NewTracker(config, reporter, progress.OperationFileRead)
//
//	// Start operation
//	total := int64(fileSize)
//	if err := tracker.Start(ctx, &total, "Reading large file"); err != nil {
//	    return err
//	}
//
//	// Report progress
//	for bytesRead := int64(0); bytesRead < fileSize; bytesRead += chunkSize {
//	    // ... read data ...
//	    if err := tracker.Update(ctx, bytesRead, "Reading..."); err != nil {
//	        return err
//	    }
//	}
//
//	// Complete
//	if err := tracker.Complete(ctx, "File read successfully"); err != nil {
//	    return err
//	}
//
// ## Simplified Tracking with TrackOperation
//
//	err := progress.TrackOperation(ctx, config, reporter,
//	    progress.OperationFileWrite, &total, "Writing file",
//	    func(ctx context.Context, tracker *progress.Tracker) error {
//	        // Operation code here
//	        for i := 0; i < count; i++ {
//	            // ... do work ...
//	            tracker.Update(ctx, int64(i), "Processing...")
//	        }
//	        return nil
//	    },
//	)
//
// ## Integration with Protocol Events
//
// Use ProtocolReporter to emit progress events through the protocol:
//
//	reporter := progress.NewProtocolReporter(func(event protocol.EventMsg) error {
//	    return eventQueue.Emit(event)
//	})
//
// # Operation Types
//
// The package defines several operation types:
//   - OperationFileRead: Reading files
//   - OperationFileWrite: Writing files
//   - OperationPatchApply: Applying code patches
//   - OperationMcpDiscovery: Discovering MCP tools
//   - OperationHistoryReconstruct: Loading conversation history
//
// Each operation type has heuristics for estimating duration and providing
// appropriate progress granularity.
//
// # Protocol Integration
//
// Progress events are emitted as protocol EventMsg types:
//   - EventOperationStarted
//   - EventOperationProgress
//   - EventOperationCompleted
//
// These events flow through the event queue to the client, which can display
// them as progress bars, spinners, or status messages.
//
// # Performance Considerations
//
// The tracker includes several optimizations:
//   - Update throttling: Limits event frequency to avoid overwhelming clients
//   - Minimum duration: Skips completion events for fast operations
//   - Thread safety: Lock-protected for concurrent access
//   - NoOp mode: Zero overhead when disabled
//
// # Testing
//
// The package includes comprehensive tests covering:
//   - Basic progress flow (start -> update -> complete)
//   - Increment-based updates
//   - Disabled tracking
//   - Minimum duration threshold
//   - Update throttling
//   - Failure and cancellation paths
//   - Multiple completion handling
//   - Protocol event serialization
//
// See progress_test.go for examples.
package progress
