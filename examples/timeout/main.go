package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/allisson/go-pglock/v3"
	_ "github.com/lib/pq"
)

func processWithTimeout(db *sql.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lockID := int64(400)

	lock, err := pglock.NewLock(ctx, lockID, db)
	if err != nil {
		return err
	}
	defer lock.Close()

	fmt.Printf("Attempting to acquire lock with %v timeout...\n", timeout)

	// This will fail if lock is not acquired within timeout
	if err := lock.WaitAndLock(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("could not acquire lock within %v", timeout)
		}
		return err
	}
	defer lock.Unlock(context.Background()) // Use background for cleanup

	fmt.Println("✓ Lock acquired, processing...")
	time.Sleep(2 * time.Second)
	fmt.Println("✓ Processing complete")

	return nil
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

	// First, acquire a lock and hold it
	ctx := context.Background()
	lockID := int64(400)

	lock, err := pglock.NewLock(ctx, lockID, db)
	if err != nil {
		log.Fatal(err)
	}

	if err := lock.WaitAndLock(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Background lock acquired (will be held for 10 seconds)")

	// Spawn a goroutine to release after delay
	go func() {
		time.Sleep(10 * time.Second)
		lock.Unlock(ctx)
		lock.Close()
		fmt.Println("\nBackground lock released")
	}()

	// Give it a moment to ensure the lock is held
	time.Sleep(500 * time.Millisecond)

	// Now try with a short timeout (should fail)
	fmt.Println("\nTest 1: Attempting with 3 second timeout (should fail)...")
	if err := processWithTimeout(db, 3*time.Second); err != nil {
		fmt.Printf("✗ Expected failure: %v\n", err)
	}

	// Try again with longer timeout (should succeed)
	fmt.Println("\nTest 2: Attempting with 15 second timeout (should succeed)...")
	if err := processWithTimeout(db, 15*time.Second); err != nil {
		fmt.Printf("✗ Unexpected error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully acquired lock with timeout")
	}

	fmt.Println("\nTimeout example completed!")
}
