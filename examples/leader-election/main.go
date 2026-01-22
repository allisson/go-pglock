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

type LeaderElector struct {
	db         *sql.DB
	lockID     int64
	isLeader   bool
	mu         sync.RWMutex
	instanceID string
}

func NewLeaderElector(db *sql.DB, clusterName, instanceID string) *LeaderElector {
	return &LeaderElector{
		db:         db,
		lockID:     hashToInt64(clusterName),
		instanceID: instanceID,
	}
}

func (le *LeaderElector) RunElection(ctx context.Context) error {
	lock, err := pglock.NewLock(ctx, le.lockID, le.db)
	if err != nil {
		return err
	}
	defer lock.Close()

	// Try to become leader
	acquired, err := lock.Lock(ctx)
	if err != nil {
		return err
	}

	if acquired {
		le.mu.Lock()
		le.isLeader = true
		le.mu.Unlock()

		fmt.Printf("✓ Instance %s became leader\n", le.instanceID)
		defer func() {
			le.mu.Lock()
			le.isLeader = false
			le.mu.Unlock()
			lock.Unlock(context.Background())
			fmt.Printf("✗ Instance %s lost leadership\n", le.instanceID)
		}()

		// Perform leader duties
		le.performLeaderDuties(ctx)
	} else {
		fmt.Printf("Instance %s is a follower (another instance is leader)\n", le.instanceID)
	}

	return nil
}

func (le *LeaderElector) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

func (le *LeaderElector) performLeaderDuties(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count++
			fmt.Printf("  [Leader %s] Performing periodic task #%d...\n", le.instanceID, count)
			if count >= 5 {
				return // Exit after 5 iterations for demo
			}
		}
	}
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

	// Simulate multiple instances trying to become leader
	fmt.Println("Starting leader election simulation...")
	fmt.Println("Simulating 3 instances competing for leadership")
	fmt.Println()

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		instanceID := fmt.Sprintf("instance-%d", i)

		go func(id string) {
			defer wg.Done()

			elector := NewLeaderElector(db, "my-cluster", id)
			if err := elector.RunElection(ctx); err != nil {
				log.Printf("Instance %s error: %v", id, err)
			}
		}(instanceID)

		// Stagger the starts slightly
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	fmt.Println("\nLeader election simulation completed!")
}
