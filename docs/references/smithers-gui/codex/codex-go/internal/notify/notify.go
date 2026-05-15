package notify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DefaultScriptTimeout is the default timeout for notification scripts.
const DefaultScriptTimeout = 5 * time.Second

const (
	DefaultNotificationRate = 10  // notifications per second
	MaxWorkers             = 5    // worker pool size
	NotificationQueueSize  = 100  // buffered channel size
	ShutdownTimeout        = 10 * time.Second
)

// NotificationConfig contains configuration for a specific notification trigger.
type NotificationConfig struct {
	// Command is the script command to execute
	Command string

	// Enabled controls whether this notification is active
	Enabled bool

	// Env contains additional environment variables for the script
	Env map[string]string
}

// Config contains all notification configurations.
type Config struct {
	// OnTurnComplete is triggered when a turn completes successfully
	OnTurnComplete *NotificationConfig

	// OnError is triggered when a turn encounters an error
	OnError *NotificationConfig

	// OnApprovalNeeded is triggered when user approval is required
	OnApprovalNeeded *NotificationConfig

	// OnTurnAborted is triggered when a turn is aborted/interrupted
	OnTurnAborted *NotificationConfig

	// ScriptTimeout is the maximum time scripts can run (defaults to 5 seconds)
	ScriptTimeout time.Duration
}

// Notifier manages notification dispatching.
type Notifier struct {
	config      *Config
	executor    *ScriptExecutor
	workerPool  *WorkerPool
	rateLimiter *RateLimiter
	mu          sync.RWMutex
	enabled     bool
	closed      bool
}

// NewNotifier creates a new notification manager with the given configuration.
func NewNotifier(config *Config) *Notifier {
	if config == nil {
		config = &Config{}
	}

	// Set default timeout if not specified
	timeout := config.ScriptTimeout
	if timeout == 0 {
		timeout = DefaultScriptTimeout
	}

	// Create rate limiter
	rateLimiter := NewRateLimiter(DefaultNotificationRate, DefaultNotificationRate*2)

	// Create worker pool
	workerPool := NewWorkerPool(MaxWorkers, NotificationQueueSize)

	return &Notifier{
		config:      config,
		executor:    NewScriptExecutor(timeout),
		workerPool:  workerPool,
		rateLimiter: rateLimiter,
		enabled:     true,
	}
}

// Notify dispatches a notification event.
// This is non-blocking and will not fail even if the script fails to execute.
func (n *Notifier) Notify(ctx context.Context, event *NotificationEvent) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.enabled || n.closed {
		return nil
	}

	// Get the configuration for this event type
	notifConfig := n.getConfigForEventType(event.Type)
	if notifConfig == nil || !notifConfig.Enabled || notifConfig.Command == "" {
		return nil
	}

	// Validate command
	if err := validateNotificationCommand(notifConfig.Command); err != nil {
		// Log error but don't fail notification
		_ = fmt.Sprintf("notification command validation failed: %v", err)
		return nil
	}

	// Check rate limit
	if !n.rateLimiter.Allow() {
		// Rate limit exceeded, drop notification
		_ = fmt.Sprintf("notification rate limit exceeded, dropping event")
		return nil
	}

	// Set up executor environment with configured variables
	executor := NewScriptExecutor(n.executor.Timeout)
	for key, value := range notifConfig.Env {
		executor.SetEnv(key, value)
	}

	// Submit to worker pool
	job := &NotificationJob{
		ctx:      ctx,
		config:   notifConfig,
		event:    event,
		executor: executor,
	}

	if err := n.workerPool.Submit(job); err != nil {
		// Queue full, drop notification
		_ = fmt.Sprintf("notification queue full, dropping event")
	}

	return nil
}

// NotifyTurnComplete sends a turn completion notification.
func (n *Notifier) NotifyTurnComplete(ctx context.Context, sessionID, turnID, message string) error {
	event := NewNotificationEvent(EventTurnComplete, sessionID, turnID).
		WithStatus("success").
		WithMessage(message)
	return n.Notify(ctx, event)
}

// NotifyTurnError sends a turn error notification.
func (n *Notifier) NotifyTurnError(ctx context.Context, sessionID, turnID, errorMsg string) error {
	event := NewNotificationEvent(EventTurnError, sessionID, turnID).
		WithError(errorMsg)
	return n.Notify(ctx, event)
}

// NotifyApprovalNeeded sends an approval needed notification.
func (n *Notifier) NotifyApprovalNeeded(ctx context.Context, sessionID, turnID, toolName string) error {
	event := NewNotificationEvent(EventApprovalNeeded, sessionID, turnID).
		WithStatus("waiting").
		WithMetadata("tool_name", toolName)
	return n.Notify(ctx, event)
}

// NotifyTurnAborted sends a turn aborted notification.
func (n *Notifier) NotifyTurnAborted(ctx context.Context, sessionID, turnID, reason string) error {
	event := NewNotificationEvent(EventTurnAborted, sessionID, turnID).
		WithStatus("aborted").
		WithMessage(reason)
	return n.Notify(ctx, event)
}

// getConfigForEventType returns the notification config for the given event type.
func (n *Notifier) getConfigForEventType(eventType EventType) *NotificationConfig {
	switch eventType {
	case EventTurnComplete:
		return n.config.OnTurnComplete
	case EventTurnError:
		return n.config.OnError
	case EventApprovalNeeded:
		return n.config.OnApprovalNeeded
	case EventTurnAborted:
		return n.config.OnTurnAborted
	default:
		return nil
	}
}

// Enable enables notification dispatching.
func (n *Notifier) Enable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = true
}

// Disable disables notification dispatching.
func (n *Notifier) Disable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = false
}

// IsEnabled returns whether notifications are enabled.
func (n *Notifier) IsEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enabled
}

// UpdateConfig updates the notifier configuration.
func (n *Notifier) UpdateConfig(config *Config) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	n.config = config

	// Update executor timeout if specified
	if config.ScriptTimeout > 0 {
		n.executor.Timeout = config.ScriptTimeout
	}

	return nil
}

// Close gracefully shuts down the notifier
func (n *Notifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil
	}

	n.closed = true
	n.enabled = false

	// Shut down worker pool with timeout
	return n.workerPool.Close(ShutdownTimeout)
}
