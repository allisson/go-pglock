package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/allisson/go-pglock/v3"
	_ "github.com/lib/pq"
)

func main() {
	// Get database URL from environment
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:5432/pglock?sslmode=disable"
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	lockID := int64(100)

	// Create lock
	lock, err := pglock.NewLock(ctx, lockID, db)
	if err != nil {
		log.Fatal(err)
	}
	defer lock.Close()

	fmt.Println("Attempting to acquire lock...")

	// Acquire lock
	acquired, err := lock.Lock(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if !acquired {
		fmt.Println("Lock is held by another process")
		return
	}

	// Critical section
	fmt.Println("Lock acquired! Executing critical section...")
	fmt.Println("Doing important work...")

	// Release lock
	if err := lock.Unlock(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Lock released successfully!")
}
