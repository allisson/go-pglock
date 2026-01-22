package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/allisson/go-pglock/v3"
	_ "github.com/lib/pq"
)

func runWorker(workerID int, db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	ctx := context.Background()
	lockID := int64(500) // Same lock ID for all workers

	lock, err := pglock.NewLock(ctx, lockID, db)
	if err != nil {
		log.Printf("Worker %d: failed to create lock: %v", workerID, err)
		return
	}
	defer lock.Close()

	fmt.Printf("Worker %d: waiting for lock...\n", workerID)

	if err := lock.WaitAndLock(ctx); err != nil {
		log.Printf("Worker %d: failed to acquire lock: %v", workerID, err)
		return
	}

	fmt.Printf("Worker %d: ✓ acquired lock, processing...\n", workerID)

	// Simulate work
	time.Sleep(1 * time.Second)

	fmt.Printf("Worker %d: releasing lock\n", workerID)
	if err := lock.Unlock(ctx); err != nil {
		log.Printf("Worker %d: failed to release lock: %v", workerID, err)
	}
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

	fmt.Println("Starting 5 concurrent workers...")
	fmt.Println("They will compete for the same lock and execute sequentially.")
	fmt.Println()

	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go runWorker(i, db, &wg)
	}

	wg.Wait()
	fmt.Println("\nAll workers completed!")
}
