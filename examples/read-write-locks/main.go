package main

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"sync"
	"time"

	"github.com/allisson/go-pglock/v3"
	_ "github.com/lib/pq"
)

type DataCache struct {
	db       *sql.DB
	recordID string
	lockID   int64
}

func NewDataCache(db *sql.DB, recordID string) *DataCache {
	return &DataCache{
		db:       db,
		recordID: recordID,
		lockID:   hashToInt64("cache-" + recordID),
	}
}

// ReadData acquires a shared lock for reading
// Multiple readers can hold this lock simultaneously
func (dc *DataCache) ReadData(ctx context.Context, readerID int) (string, error) {
	lock, err := pglock.NewLock(ctx, dc.lockID, dc.db)
	if err != nil {
		return "", err
	}
	defer lock.Close()

	fmt.Printf("Reader %d: attempting to acquire shared (read) lock...\n", readerID)

	// Acquire shared lock - multiple readers can hold this simultaneously
	if err := lock.WaitAndRLock(ctx); err != nil {
		return "", fmt.Errorf("failed to acquire read lock: %w", err)
	}
	defer lock.RUnlock(ctx)

	fmt.Printf("Reader %d: ✓ acquired shared lock, reading data...\n", readerID)

	// Simulate reading from database or cache
	time.Sleep(2 * time.Second)
	data := "cached-data-for-" + dc.recordID

	fmt.Printf("Reader %d: finished reading, releasing lock\n", readerID)
	return data, nil
}

// WriteData acquires an exclusive lock for writing
// This blocks all other locks (both read and write)
func (dc *DataCache) WriteData(ctx context.Context, data string) error {
	lock, err := pglock.NewLock(ctx, dc.lockID, dc.db)
	if err != nil {
		return err
	}
	defer lock.Close()

	fmt.Println("Writer: attempting to acquire exclusive (write) lock...")

	// Acquire exclusive lock - blocks all other locks (read and write)
	if err := lock.WaitAndLock(ctx); err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	defer lock.Unlock(ctx)

	fmt.Println("Writer: ✓ acquired exclusive lock, writing data...")

	// Simulate writing to database and invalidating cache
	time.Sleep(3 * time.Second)

	fmt.Println("Writer: finished writing, releasing lock")
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

	fmt.Println("Read-Write Lock Example")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("This example demonstrates shared (read) and exclusive (write) locks:")
	fmt.Println("- Multiple readers can read simultaneously (shared locks)")
	fmt.Println("- Writers block all readers and other writers (exclusive locks)")
	fmt.Println()

	cache := NewDataCache(db, "user-123")
	ctx := context.Background()

	var wg sync.WaitGroup

	// Scenario 1: Multiple concurrent readers
	fmt.Println("Scenario 1: Starting 3 concurrent readers...")
	fmt.Println("Expected: All 3 readers acquire locks simultaneously")
	fmt.Println()

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			data, err := cache.ReadData(ctx, readerID)
			if err != nil {
				log.Printf("Reader %d error: %v", readerID, err)
				return
			}
			fmt.Printf("Reader %d got: %s\n", readerID, data)
		}(i)
		time.Sleep(100 * time.Millisecond) // Stagger starts slightly
	}

	wg.Wait()
	fmt.Println()
	time.Sleep(1 * time.Second)

	// Scenario 2: Writer blocks readers
	fmt.Println("Scenario 2: Writer starts, then readers try to acquire locks...")
	fmt.Println("Expected: Writer acquires lock first, readers wait until writer completes")
	fmt.Println()

	// Start writer first
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cache.WriteData(ctx, "new-data"); err != nil {
			log.Printf("Writer error: %v", err)
		}
		fmt.Println("Writer: completed")
	}()

	// Give writer time to acquire the lock
	time.Sleep(500 * time.Millisecond)

	// Now start readers - they should wait for writer
	for i := 4; i <= 5; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			data, err := cache.ReadData(ctx, readerID)
			if err != nil {
				log.Printf("Reader %d error: %v", readerID, err)
				return
			}
			fmt.Printf("Reader %d got: %s\n", readerID, data)
		}(i)
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	fmt.Println()
	time.Sleep(1 * time.Second)

	// Scenario 3: Readers block writer
	fmt.Println("Scenario 3: Readers start, then writer tries to acquire lock...")
	fmt.Println("Expected: Readers acquire locks first, writer waits until all readers complete")
	fmt.Println()

	// Start readers first
	for i := 6; i <= 7; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			data, err := cache.ReadData(ctx, readerID)
			if err != nil {
				log.Printf("Reader %d error: %v", readerID, err)
				return
			}
			fmt.Printf("Reader %d got: %s\n", readerID, data)
		}(i)
		time.Sleep(100 * time.Millisecond)
	}

	// Give readers time to acquire locks
	time.Sleep(500 * time.Millisecond)

	// Now start writer - it should wait for readers
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cache.WriteData(ctx, "updated-data"); err != nil {
			log.Printf("Writer error: %v", err)
		}
		fmt.Println("Writer: completed")
	}()

	wg.Wait()

	fmt.Println()
	fmt.Println("All scenarios completed!")
	fmt.Println()
	fmt.Println("Key Takeaways:")
	fmt.Println("- Shared locks (RLock) allow multiple concurrent readers")
	fmt.Println("- Exclusive locks (Lock) block all other operations")
	fmt.Println("- Use RLock/RUnlock for read operations")
	fmt.Println("- Use Lock/Unlock for write operations")
}
