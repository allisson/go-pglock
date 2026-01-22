package pglock

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDB creates a new database connection for testing.
func newDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL environment variable not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "failed to open database connection")
	require.NoError(t, db.Ping(), "failed to ping database")
	return db
}

// closeDB closes the database connection and reports any errors.
func closeDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("failed to close database: %v", err)
	}
}

// TestNewLock verifies that NewLock creates a valid Lock instance.
func TestNewLock(t *testing.T) {
	db := newDB(t)
	defer closeDB(t, db)

	id := int64(10)
	lock, err := NewLock(context.Background(), id, db)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, lock.Close())
	}()

	assert.Equal(t, id, lock.id)
	assert.NotNil(t, lock.conn)
}

// TestNewLock_ContextCancelled tests NewLock with a cancelled context.
func TestNewLock_ContextCancelled(t *testing.T) {
	db := newDB(t)
	defer closeDB(t, db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := NewLock(ctx, 1, db)
	assert.Error(t, err, "NewLock should fail with cancelled context")
}

// TestLock_BasicAcquisitionAndRelease tests basic lock acquisition and release.
func TestLock_BasicAcquisitionAndRelease(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()
	id := int64(1)

	lock1, err := NewLock(ctx, id, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(ctx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// lock1 should acquire the lock
	ok, err := lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "lock1 should acquire the lock")

	// lock2 should fail to acquire the lock
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "lock2 should not acquire the lock")

	// Release lock1
	err = lock1.Unlock(ctx)
	require.NoError(t, err)

	// lock2 should now acquire the lock
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "lock2 should acquire the lock after lock1 releases")

	// Clean up
	err = lock2.Unlock(ctx)
	require.NoError(t, err)
}

// TestLock_SameSessionMultipleAcquisitions tests that the same session can acquire a lock multiple times.
func TestLock_SameSessionMultipleAcquisitions(t *testing.T) {
	db := newDB(t)
	defer closeDB(t, db)

	ctx := context.Background()
	id := int64(2)

	lock, err := NewLock(ctx, id, db)
	require.NoError(t, err)
	defer lock.Close()

	// Acquire lock multiple times
	ok, err := lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "same session should be able to acquire lock multiple times")

	ok, err = lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "same session should be able to acquire lock multiple times")

	// Unlock the same number of times
	require.NoError(t, lock.Unlock(ctx))
	require.NoError(t, lock.Unlock(ctx))
	require.NoError(t, lock.Unlock(ctx))
}

// TestLock_Stacking verifies that locks stack and require equal unlocks.
func TestLock_Stacking(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()
	id := int64(3)

	lock1, err := NewLock(ctx, id, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(ctx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// Acquire lock twice with lock1
	ok, err := lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	// lock2 should not be able to acquire
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.False(t, ok)

	// Unlock once
	require.NoError(t, lock1.Unlock(ctx))

	// lock2 should still not be able to acquire (lock is still held once)
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "lock should still be held after one unlock")

	// Unlock second time
	require.NoError(t, lock1.Unlock(ctx))

	// Now lock2 should be able to acquire
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "lock2 should acquire after all unlocks")

	require.NoError(t, lock2.Unlock(ctx))
}

// TestWaitAndLock_BlockingBehavior tests that WaitAndLock blocks until lock is available.
func TestWaitAndLock_BlockingBehavior(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()
	id := int64(4)

	lock1, err := NewLock(ctx, id, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(ctx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// lock1 acquires the lock
	require.NoError(t, lock1.WaitAndLock(ctx))

	var lock2Acquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	// lock2 attempts to acquire in goroutine
	go func() {
		defer wg.Done()
		err := lock2.WaitAndLock(ctx)
		if err == nil {
			lock2Acquired.Store(true)
			lock2.Unlock(ctx)
		}
	}()

	// Give lock2 time to start waiting
	time.Sleep(100 * time.Millisecond)
	assert.False(t, lock2Acquired.Load(), "lock2 should be waiting")

	// Release lock1
	require.NoError(t, lock1.Unlock(ctx))

	// Wait for lock2 to acquire
	wg.Wait()
	assert.True(t, lock2Acquired.Load(), "lock2 should have acquired the lock")
}

// TestWaitAndLock_ContextCancellation tests that WaitAndLock respects context cancellation.
func TestWaitAndLock_ContextCancellation(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	bgCtx := context.Background()
	id := int64(5)

	lock1, err := NewLock(bgCtx, id, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(bgCtx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// lock1 acquires the lock
	require.NoError(t, lock1.WaitAndLock(bgCtx))
	defer lock1.Unlock(bgCtx)

	// Create a context with timeout for lock2
	ctx, cancel := context.WithTimeout(bgCtx, 200*time.Millisecond)
	defer cancel()

	// lock2 should fail due to context timeout
	err = lock2.WaitAndLock(ctx)
	assert.Error(t, err, "WaitAndLock should fail with context timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded", "error should indicate timeout")
}

// TestWaitAndLock_Concurrent tests multiple concurrent lock attempts.
func TestWaitAndLock_Concurrent(t *testing.T) {
	// Check if DATABASE_URL is set before spawning goroutines
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL environment variable not set")
	}

	const numGoroutines = 10
	id := int64(6)

	var wg sync.WaitGroup
	var counter int64
	ctx := context.Background()

	// Create multiple locks that will compete for the same resource
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			db := newDB(t)
			defer closeDB(t, db)

			lock, err := NewLock(ctx, id, db)
			if err != nil {
				t.Errorf("failed to create lock: %v", err)
				return
			}
			defer lock.Close()

			// Acquire lock
			if err := lock.WaitAndLock(ctx); err != nil {
				t.Errorf("failed to acquire lock: %v", err)
				return
			}

			// Critical section: increment counter
			current := atomic.LoadInt64(&counter)
			time.Sleep(10 * time.Millisecond) // Simulate work
			atomic.StoreInt64(&counter, current+1)

			// Release lock
			if err := lock.Unlock(ctx); err != nil {
				t.Errorf("failed to release lock: %v", err)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(numGoroutines), atomic.LoadInt64(&counter),
		"counter should equal number of goroutines")
}

// TestClose_ReleasesLocks verifies that closing the lock releases all held locks.
func TestClose_ReleasesLocks(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()
	id := int64(7)

	lock1, err := NewLock(ctx, id, db1)
	require.NoError(t, err)

	lock2, err := NewLock(ctx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// lock1 acquires the lock multiple times
	ok, err := lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	// lock2 cannot acquire
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.False(t, ok)

	// Close lock1 (should release all locks)
	require.NoError(t, lock1.Close())

	// lock2 should now be able to acquire
	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "lock2 should acquire after lock1 is closed")

	require.NoError(t, lock2.Unlock(ctx))
}

// TestUnlock_ExtraUnlockDoesNotError verifies that extra unlocks don't cause errors.
func TestUnlock_ExtraUnlockDoesNotError(t *testing.T) {
	db := newDB(t)
	defer closeDB(t, db)

	ctx := context.Background()
	id := int64(8)

	lock, err := NewLock(ctx, id, db)
	require.NoError(t, err)
	defer lock.Close()

	// Acquire and release normally
	ok, err := lock.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	require.NoError(t, lock.Unlock(ctx))

	// Extra unlock should not error (PostgreSQL behavior)
	err = lock.Unlock(ctx)
	assert.NoError(t, err, "extra unlock should not cause error")
}

// TestLock_DifferentIDs tests that different lock IDs don't interfere.
func TestLock_DifferentIDs(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()

	lock1, err := NewLock(ctx, 100, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(ctx, 200, db2)
	require.NoError(t, err)
	defer lock2.Close()

	// Both locks should be able to acquire (different IDs)
	ok, err := lock1.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = lock2.Lock(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "different lock IDs should not interfere")

	require.NoError(t, lock1.Unlock(ctx))
	require.NoError(t, lock2.Unlock(ctx))
}

// TestWaitAndLock_Sequential tests sequential lock acquisitions with timing.
func TestWaitAndLock_Sequential(t *testing.T) {
	db1 := newDB(t)
	defer closeDB(t, db1)
	db2 := newDB(t)
	defer closeDB(t, db2)

	ctx := context.Background()
	wg := sync.WaitGroup{}
	id := int64(9)

	lock1, err := NewLock(ctx, id, db1)
	require.NoError(t, err)
	defer lock1.Close()

	lock2, err := NewLock(ctx, id, db2)
	require.NoError(t, err)
	defer lock2.Close()

	start := time.Now()

	// Helper function that holds lock for duration
	holdLock := func(lock *Lock, duration time.Duration) {
		defer wg.Done()
		if err := lock.WaitAndLock(ctx); err != nil {
			t.Errorf("failed to acquire lock: %v", err)
			return
		}
		time.Sleep(duration)
		if err := lock.Unlock(ctx); err != nil {
			t.Errorf("failed to release lock: %v", err)
		}
	}

	wg.Add(2)
	go holdLock(&lock1, 300*time.Millisecond)
	go holdLock(&lock2, 300*time.Millisecond)

	wg.Wait()
	elapsed := time.Since(start)

	// Should take at least 600ms (sequential)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(600),
		"sequential lock acquisitions should take cumulative time")
}
