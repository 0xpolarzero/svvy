package notify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NotificationJob represents a notification to be processed
type NotificationJob struct {
	ctx      context.Context
	config   *NotificationConfig
	event    *NotificationEvent
	executor *ScriptExecutor
}

// WorkerPool manages a pool of notification workers
type WorkerPool struct {
	workers  int
	queue    chan *NotificationJob
	wg       sync.WaitGroup
	shutdown chan struct{}
	once     sync.Once
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	pool := &WorkerPool{
		workers:  workers,
		queue:    make(chan *NotificationJob, queueSize),
		shutdown: make(chan struct{}),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker processes notification jobs
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case job := <-wp.queue:
			if job == nil {
				return
			}
			// Execute the notification
			_ = job.executor.Execute(job.ctx, job.config.Command, job.event)

		case <-wp.shutdown:
			return
		}
	}
}

// Submit adds a notification job to the queue
func (wp *WorkerPool) Submit(job *NotificationJob) error {
	select {
	case wp.queue <- job:
		return nil
	default:
		// Queue is full, drop the notification
		return fmt.Errorf("notification queue full")
	}
}

// Close gracefully shuts down the worker pool
func (wp *WorkerPool) Close(timeout time.Duration) error {
	wp.once.Do(func() {
		close(wp.shutdown)

		// Wait for workers with timeout
		done := make(chan struct{})
		go func() {
			wp.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All workers finished
		case <-time.After(timeout):
			// Timeout waiting for workers
		}
	})
	return nil
}
