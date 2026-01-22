package main

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"time"

	"github.com/allisson/go-pglock/v3"
	_ "github.com/lib/pq"
)

type TaskProcessor struct {
	db *sql.DB
}

func NewTaskProcessor(db *sql.DB) *TaskProcessor {
	return &TaskProcessor{db: db}
}

func (tp *TaskProcessor) ProcessTask(taskID string) error {
	ctx := context.Background()

	// Use hash of task ID as lock ID
	lockID := hashToInt64(taskID)

	lock, err := pglock.NewLock(ctx, lockID, tp.db)
	if err != nil {
		return err
	}
	defer lock.Close()

	// Try to acquire lock
	acquired, err := lock.Lock(ctx)
	if err != nil {
		return err
	}

	if !acquired {
		return fmt.Errorf("task %s is already being processed by another instance", taskID)
	}
	defer lock.Unlock(ctx)

	fmt.Printf("✓ Processing task %s...\n", taskID)

	// Execute task
	if err := tp.executeTask(taskID); err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	fmt.Printf("✓ Task %s completed successfully\n", taskID)
	return nil
}

func (tp *TaskProcessor) executeTask(taskID string) error {
	// Simulate task execution
	time.Sleep(2 * time.Second)
	return nil
}

func hashToInt64(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

func main() {
	// Get database URL from environment
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:5432/pglock?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	processor := NewTaskProcessor(db)

	// Simulate tasks
	tasks := []string{
		"send-email-123",
		"process-payment-456",
		"generate-report-789",
	}

	fmt.Println("Processing tasks...")
	fmt.Println("Each task will only run once even if multiple instances try to process it.")
	fmt.Println()

	for _, taskID := range tasks {
		if err := processor.ProcessTask(taskID); err != nil {
			fmt.Printf("✗ Error processing task %s: %v\n", taskID, err)
		}
		fmt.Println()
	}

	// Try to process the same task again (should be rejected or complete)
	fmt.Println("Attempting to process a task again immediately...")
	if err := processor.ProcessTask(tasks[0]); err != nil {
		fmt.Printf("✗ Expected behavior: %v\n", err)
	}

	fmt.Println("\nTask processing example completed!")
}
