// +build standalone

package native

import (
	"context"
	"testing"
)

// Simple standalone test to verify compilation and basic functionality
// Run with: go test -tags standalone -v
func TestStandalone(t *testing.T) {
	// Test nil context
	sb := New()
	_, err := sb.Execute(nil, nil)
	if err == nil {
		t.Fatal("Expected error for nil context")
	}
	t.Logf("✓ Nil context validation works: %v", err)

	// Test nil command
	ctx := context.Background()
	_, err = sb.Execute(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil command")
	}
	t.Logf("✓ Nil command validation works: %v", err)

	// Test options
	opts := &Options{
		MaxOutputSize:         1024,
		WarnOnIgnoredSecurity: false,
	}
	sb2 := NewWithOptions(opts)
	if sb2.opts.MaxOutputSize != 1024 {
		t.Fatalf("Expected MaxOutputSize=1024, got %d", sb2.opts.MaxOutputSize)
	}
	t.Logf("✓ Options configuration works")

	t.Log("All standalone tests passed!")
}
