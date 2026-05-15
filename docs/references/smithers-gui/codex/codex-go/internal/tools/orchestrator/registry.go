package orchestrator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

const (
	MaxCallIDLength      = 256
	MaxToolNameLength    = 128
	MaxArgumentsSize     = 1 * 1024 * 1024 // 1MB serialized
	MaxArgumentDepth     = 10
	MaxArraySize         = 1000
	MaxStringSize        = 1 * 1024 * 1024 // 1MB per string
	MaxRequestsPerBatch  = 100
)

// SanitizeCallID validates and sanitizes a call ID
func SanitizeCallID(callID string) error {
	if callID == "" {
		return fmt.Errorf("call ID cannot be empty")
	}

	if len(callID) > MaxCallIDLength {
		return fmt.Errorf("call ID exceeds maximum length of %d", MaxCallIDLength)
	}

	// Check for null bytes first
	if strings.Contains(callID, "\x00") {
		return fmt.Errorf("call ID contains null byte")
	}

	// Allow only alphanumeric, hyphen, and underscore
	for i, c := range callID {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("call ID contains invalid character at position %d: %c", i, c)
		}
	}

	return nil
}

// SanitizeToolName validates and sanitizes a tool name
func SanitizeToolName(toolName string) error {
	if toolName == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if len(toolName) > MaxToolNameLength {
		return fmt.Errorf("tool name exceeds maximum length of %d", MaxToolNameLength)
	}

	// Check for null bytes first
	if strings.Contains(toolName, "\x00") {
		return fmt.Errorf("tool name contains null byte")
	}

	// Prevent path traversal
	if strings.Contains(toolName, "..") {
		return fmt.Errorf("tool name contains path traversal")
	}

	// Allow only alphanumeric, hyphen, underscore, and slash (for namespaced tools)
	for i, c := range toolName {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '/' || c == '.') {
			return fmt.Errorf("tool name contains invalid character at position %d: %c", i, c)
		}
	}

	return nil
}

// ValidateArgumentStructure validates the structure of tool arguments
func ValidateArgumentStructure(args map[string]interface{}) error {
	return validateValue(args, 0)
}

// validateValue recursively validates argument values
func validateValue(val interface{}, depth int) error {
	// Check depth to prevent stack overflow
	if depth > MaxArgumentDepth {
		return fmt.Errorf("argument nesting exceeds maximum depth of %d", MaxArgumentDepth)
	}

	switch v := val.(type) {
	case nil:
		// Nil is okay
		return nil

	case bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		// Primitive types are okay
		return nil

	case string:
		if len(v) > MaxStringSize {
			return fmt.Errorf("string exceeds maximum size of %d bytes", MaxStringSize)
		}
		// Check for null bytes
		if strings.Contains(v, "\x00") {
			return fmt.Errorf("string contains null byte")
		}
		return nil

	case []interface{}:
		if len(v) > MaxArraySize {
			return fmt.Errorf("array exceeds maximum size of %d elements", MaxArraySize)
		}
		for i, item := range v {
			if err := validateValue(item, depth+1); err != nil {
				return fmt.Errorf("array[%d]: %w", i, err)
			}
		}
		return nil

	case map[string]interface{}:
		if len(v) > MaxArraySize {
			return fmt.Errorf("object exceeds maximum size of %d properties", MaxArraySize)
		}
		for key, item := range v {
			// Validate key
			if key == "" {
				return fmt.Errorf("object key cannot be empty")
			}
			if len(key) > 256 {
				return fmt.Errorf("object key exceeds maximum length of 256")
			}
			if strings.Contains(key, "\x00") {
				return fmt.Errorf("object key contains null byte")
			}
			// Validate value
			if err := validateValue(item, depth+1); err != nil {
				return fmt.Errorf("object[%s]: %w", key, err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported argument type: %T", v)
	}
}

// RegistryHelper provides convenience methods for working with tool registries.
// The core ToolRegistry is defined in runtime package; this adds orchestrator-specific helpers.
type RegistryHelper struct {
	registry *runtime.ToolRegistry
}

// NewRegistryHelper wraps a tool registry with helper methods.
func NewRegistryHelper(registry *runtime.ToolRegistry) *RegistryHelper {
	return &RegistryHelper{
		registry: registry,
	}
}

// GetOrError retrieves a tool by name or returns a structured error.
func (h *RegistryHelper) GetOrError(name string) (runtime.ToolRuntime, error) {
	tool := h.registry.Get(name)
	if tool == nil {
		return nil, &runtime.ToolError{
			Kind:    runtime.ErrorInternal,
			Message: fmt.Sprintf("tool not found: %s", name),
		}
	}
	return tool, nil
}

// ListSorted returns all tool names sorted alphabetically.
func (h *RegistryHelper) ListSorted() []string {
	names := h.registry.List()
	sort.Strings(names)
	return names
}

// CountTools returns the number of registered tools.
func (h *RegistryHelper) CountTools() int {
	return len(h.registry.List())
}

// HasTool checks if a tool with the given name is registered.
func (h *RegistryHelper) HasTool(name string) bool {
	return h.registry.Get(name) != nil
}

// GetToolsByCapability returns all tools matching a specific capability.
func (h *RegistryHelper) GetToolsByCapability(check func(runtime.ToolRuntime) bool) []runtime.ToolRuntime {
	tools := []runtime.ToolRuntime{}
	for _, name := range h.registry.List() {
		tool := h.registry.Get(name)
		if tool != nil && check(tool) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// GetParallelTools returns all tools that support parallel execution.
func (h *RegistryHelper) GetParallelTools() []runtime.ToolRuntime {
	return h.GetToolsByCapability(func(t runtime.ToolRuntime) bool {
		return t.SupportsParallel()
	})
}

// GetSequentialTools returns all tools that don't support parallel execution.
func (h *RegistryHelper) GetSequentialTools() []runtime.ToolRuntime {
	return h.GetToolsByCapability(func(t runtime.ToolRuntime) bool {
		return !t.SupportsParallel()
	})
}

// GetToolsRequiringSandbox returns all tools that require sandboxing.
func (h *RegistryHelper) GetToolsRequiringSandbox() []runtime.ToolRuntime {
	return h.GetToolsByCapability(func(t runtime.ToolRuntime) bool {
		return t.SandboxPreference() == runtime.SandboxRequire
	})
}

// GetToolsForbiddingSandbox returns all tools that forbid sandboxing.
func (h *RegistryHelper) GetToolsForbiddingSandbox() []runtime.ToolRuntime {
	return h.GetToolsByCapability(func(t runtime.ToolRuntime) bool {
		return t.SandboxPreference() == runtime.SandboxForbid
	})
}

// ToolInfo provides metadata about a registered tool.
type ToolInfo struct {
	Name              string
	SupportsParallel  bool
	SandboxPreference runtime.SandboxPreference
	EscalateOnFailure bool
}

// GetToolInfo returns metadata about a tool.
func (h *RegistryHelper) GetToolInfo(name string) (*ToolInfo, error) {
	tool, err := h.GetOrError(name)
	if err != nil {
		return nil, err
	}

	return &ToolInfo{
		Name:              tool.Name(),
		SupportsParallel:  tool.SupportsParallel(),
		SandboxPreference: tool.SandboxPreference(),
		EscalateOnFailure: tool.EscalateOnFailure(),
	}, nil
}

// GetAllToolInfo returns metadata for all registered tools.
func (h *RegistryHelper) GetAllToolInfo() []*ToolInfo {
	infos := []*ToolInfo{}
	for _, name := range h.ListSorted() {
		info, err := h.GetToolInfo(name)
		if err == nil {
			infos = append(infos, info)
		}
	}
	return infos
}

// ValidateToolRequests validates a batch of tool requests.
// Returns the first invalid request or nil if all are valid.
func (h *RegistryHelper) ValidateToolRequests(requests []*runtime.ToolRequest) error {
	if len(requests) == 0 {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInvalidArguments,
			Message: "requests cannot be empty",
		}
	}

	if len(requests) > MaxRequestsPerBatch {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInvalidArguments,
			Message: fmt.Sprintf("batch exceeds maximum of %d requests", MaxRequestsPerBatch),
		}
	}

	for i, req := range requests {
		if err := h.ValidateToolRequest(req); err != nil {
			return fmt.Errorf("request[%d]: %w", i, err)
		}
	}
	return nil
}

// ValidateToolRequest validates a single tool request.
func (h *RegistryHelper) ValidateToolRequest(req *runtime.ToolRequest) error {
	if req == nil {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInvalidArguments,
			Message: "request is nil",
		}
	}

	// Validate CallID
	if err := SanitizeCallID(req.CallID); err != nil {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInvalidArguments,
			Message: fmt.Sprintf("invalid CallID: %v", err),
		}
	}

	// Validate ToolName
	if err := SanitizeToolName(req.ToolName); err != nil {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInvalidArguments,
			Message: fmt.Sprintf("invalid ToolName: %v", err),
		}
	}

	// Check if tool exists
	if !h.HasTool(req.ToolName) {
		return &runtime.ToolError{
			Kind:    runtime.ErrorInternal,
			Message: fmt.Sprintf("tool not found: %s", req.ToolName),
		}
	}

	// Validate arguments if present
	if req.Arguments != "" {
		// Check serialized size first
		if len(req.Arguments) > MaxArgumentsSize {
			return &runtime.ToolError{
				Kind:    runtime.ErrorInvalidArguments,
				Message: fmt.Sprintf("arguments exceed maximum size of %d bytes", MaxArgumentsSize),
			}
		}

		// Validate JSON format
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
			return &runtime.ToolError{
				Kind:    runtime.ErrorInvalidArguments,
				Message: fmt.Sprintf("arguments are not valid JSON: %v", err),
			}
		}

		// Validate structure
		if err := ValidateArgumentStructure(args); err != nil {
			return &runtime.ToolError{
				Kind:    runtime.ErrorInvalidArguments,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			}
		}
	}

	return nil
}

// GroupRequestsByParallelism groups tool requests into parallel and sequential batches.
// This helps the execution engine optimize scheduling.
func (h *RegistryHelper) GroupRequestsByParallelism(
	requests []*runtime.ToolRequest,
) (parallel []*runtime.ToolRequest, sequential []*runtime.ToolRequest) {
	parallel = []*runtime.ToolRequest{}
	sequential = []*runtime.ToolRequest{}

	for _, req := range requests {
		tool := h.registry.Get(req.ToolName)
		if tool == nil {
			// If tool not found, treat as sequential to be safe
			sequential = append(sequential, req)
			continue
		}

		if tool.SupportsParallel() {
			parallel = append(parallel, req)
		} else {
			sequential = append(sequential, req)
		}
	}

	return parallel, sequential
}

// FilterRequestsByTool filters requests by tool name.
func (h *RegistryHelper) FilterRequestsByTool(
	requests []*runtime.ToolRequest,
	toolName string,
) []*runtime.ToolRequest {
	filtered := []*runtime.ToolRequest{}
	for _, req := range requests {
		if req.ToolName == toolName {
			filtered = append(filtered, req)
		}
	}
	return filtered
}

// DeduplicateRequests removes duplicate requests based on CallID.
// Returns deduplicated requests maintaining original order.
func (h *RegistryHelper) DeduplicateRequests(
	requests []*runtime.ToolRequest,
) []*runtime.ToolRequest {
	seen := make(map[string]bool)
	deduplicated := []*runtime.ToolRequest{}

	for _, req := range requests {
		if !seen[req.CallID] {
			seen[req.CallID] = true
			deduplicated = append(deduplicated, req)
		}
	}

	return deduplicated
}

// CreateRegistrySnapshot captures the current state of the registry.
// Useful for testing and debugging.
type RegistrySnapshot struct {
	ToolNames   []string
	ToolCount   int
	Parallel    int
	Sequential  int
	Sandboxed   int
	Unsandboxed int
}

// GetSnapshot returns a snapshot of the current registry state.
func (h *RegistryHelper) GetSnapshot() *RegistrySnapshot {
	snapshot := &RegistrySnapshot{
		ToolNames: h.ListSorted(),
		ToolCount: h.CountTools(),
	}

	for _, name := range snapshot.ToolNames {
		tool := h.registry.Get(name)
		if tool == nil {
			continue
		}

		if tool.SupportsParallel() {
			snapshot.Parallel++
		} else {
			snapshot.Sequential++
		}

		pref := tool.SandboxPreference()
		if pref == runtime.SandboxRequire || pref == runtime.SandboxAuto {
			snapshot.Sandboxed++
		} else {
			snapshot.Unsandboxed++
		}
	}

	return snapshot
}
