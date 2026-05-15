package persistence_test

import (
	"fmt"
	"log"

	"github.com/evmts/codex/codex-go/internal/history/persistence"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/spf13/afero"
)

// Example demonstrates basic usage of the history persistence system
func Example_basicUsage() {
	// Use memory filesystem for this example
	fs := afero.NewMemMapFs()

	// Create persistence manager for a session
	hp, err := persistence.NewHistoryPersistence(fs, "/sessions/example-session")
	if err != nil {
		log.Fatal(err)
	}
	defer hp.Close()

	// Record a user submission
	submission := &protocol.Submission{
		ID: "sub-1",
		Op: &protocol.OpUserTurn{
			Items: []protocol.UserInput{
				{Type: "text", Text: stringPtr("Create a hello world program")},
			},
			Cwd:            "/home/user/projects",
			ApprovalPolicy: "auto",
			SandboxPolicy: protocol.SandboxPolicy{
				Mode: "unrestricted",
			},
			Model:   "claude-3-5-sonnet-20241022",
			Summary: "auto",
		},
	}
	err = hp.RecordSubmission(submission)
	if err != nil {
		log.Fatal(err)
	}

	// Record agent events
	events := []*protocol.Event{
		{
			ID:  "sub-1",
			Msg: &protocol.EventTaskStarted{ModelContextWindow: int64Ptr(200000)},
		},
		{
			ID:  "sub-1",
			Msg: &protocol.EventAgentMessage{Message: "I'll create a hello world program for you."},
		},
		{
			ID:  "sub-1",
			Msg: &protocol.EventTaskComplete{},
		},
	}

	for _, event := range events {
		err = hp.RecordEvent(event)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Load history back
	submissions, evts, err := hp.LoadHistory()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded %d submissions and %d events\n", len(submissions), len(evts))
	fmt.Printf("Session ID: %s\n", hp.SessionID())
	fmt.Printf("Session Dir: %s\n", hp.SessionDir())

	// Output:
	// Loaded 1 submissions and 3 events
	// Session ID: example-session
	// Session Dir: /sessions/example-session
}

// Example_rollouts demonstrates rollout management
func Example_rollouts() {
	fs := afero.NewMemMapFs()

	hp, err := persistence.NewHistoryPersistence(fs, "/sessions/rollout-demo")
	if err != nil {
		log.Fatal(err)
	}
	defer hp.Close()

	// Record some history
	for i := 1; i <= 3; i++ {
		submission := &protocol.Submission{
			ID: fmt.Sprintf("sub-%d", i),
			Op: &protocol.OpInterrupt{},
		}
		err = hp.RecordSubmission(submission)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create a rollout (snapshot)
	rolloutPath, err := hp.CreateRollout()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created rollout: %v\n", rolloutPath != "")

	// List all rollouts
	rollouts, err := hp.ListRollouts()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total rollouts: %d\n", len(rollouts))

	// Create more rollouts
	hp.CreateRollout()
	hp.CreateRollout()

	// Cleanup old rollouts (keep only 2)
	err = hp.CleanupOldRollouts(2)
	if err != nil {
		log.Fatal(err)
	}

	rollouts, _ = hp.ListRollouts()
	fmt.Printf("After cleanup: %d rollouts\n", len(rollouts))

	// Output:
	// Created rollout: true
	// Total rollouts: 1
	// After cleanup: 2 rollouts
}

// Example_streamingReads demonstrates streaming reads
func Example_streamingReads() {
	fs := afero.NewMemMapFs()

	// First, write some data
	writer, err := persistence.NewHistoryWriter(fs, "/test/history.jsonl")
	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= 5; i++ {
		submission := &protocol.Submission{
			ID: fmt.Sprintf("sub-%d", i),
			Op: &protocol.OpInterrupt{},
		}
		writer.Append(submission)
	}
	writer.Close()

	// Now read it back in streaming fashion
	reader, err := persistence.NewHistoryReader(fs, "/test/history.jsonl")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	count := 0
	for {
		sub, evt, err := reader.ReadNext()
		if err != nil {
			break // EOF or error
		}
		if sub != nil || evt != nil {
			count++
		}
	}

	fmt.Printf("Read %d items\n", count)

	// Output:
	// Read 5 items
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
