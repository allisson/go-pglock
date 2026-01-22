# go-pglock

[![Build Status](https://github.com/allisson/go-pglock/workflows/tests/badge.svg)](https://github.com/allisson/go-pglock/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/allisson/go-pglock/v3)](https://goreportcard.com/report/github.com/allisson/go-pglock/v3)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/allisson/go-pglock/v3)

Distributed locks using PostgreSQL session level advisory locks.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [API Reference](#api-reference)
- [Examples](#examples)
  - [Basic Lock Usage](#basic-lock-usage)
  - [Try Lock (Non-blocking)](#try-lock-non-blocking)
  - [Wait and Lock (Blocking)](#wait-and-lock-blocking)
  - [Lock with Timeout](#lock-with-timeout)
  - [Concurrent Workers](#concurrent-workers)
  - [Distributed Task Execution](#distributed-task-execution)
  - [Leader Election](#leader-election)
  - [Resource Pool Management](#resource-pool-management)
  - [Database Migration Lock](#database-migration-lock)
  - [Scheduled Job Coordination](#scheduled-job-coordination)
- [Best Practices](#best-practices)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Overview

`go-pglock` provides a simple and reliable way to implement distributed locks using PostgreSQL's advisory lock mechanism. This is useful when you need to coordinate access to shared resources across multiple processes or servers.

### Key Features

- **Simple API**: Easy-to-use interface for acquiring and releasing locks
- **Non-blocking locks**: Try to acquire a lock without waiting
- **Blocking locks**: Wait until a lock becomes available
- **Context support**: Timeout and cancellation support for all operations
- **Lock stacking**: Same session can acquire the same lock multiple times
- **Automatic cleanup**: Locks are automatically released when connections close
- **No external dependencies**: Uses only PostgreSQL (no Redis, ZooKeeper, etc.)
- **Battle-tested**: Used in production environments

### When to Use

Use `go-pglock` when you need to:

- Prevent duplicate execution of scheduled jobs across multiple servers
- Coordinate access to shared resources
- Implement leader election
- Ensure only one instance processes a particular task
- Serialize access to critical sections in distributed systems
- Manage resource pools across multiple processes

## Installation

```bash
go get github.com/allisson/go-pglock/v3
```

Requirements:
- Go 1.17 or higher
- PostgreSQL 9.6 or higher

## Quick Start

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/allisson/go-pglock/v3"
    _ "github.com/lib/pq"
)

func main() {
    // Connect to PostgreSQL
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()

    // Create a lock with ID 1
    lock, err := pglock.NewLock(ctx, 1, db)
    if err != nil {
        log.Fatal(err)
    }
    defer lock.Close()

    // Try to acquire the lock
    acquired, err := lock.Lock(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if acquired {
        fmt.Println("Lock acquired! Doing work...")
        // Do your work here
        
        // Release the lock
        if err := lock.Unlock(ctx); err != nil {
            log.Fatal(err)
        }
        fmt.Println("Lock released!")
    } else {
        fmt.Println("Could not acquire lock - another process has it")
    }
}
```

## How It Works

PostgreSQL advisory locks are a powerful feature for implementing distributed locking:

- **Session-level locks**: Locks are held until explicitly released or the database connection closes
- **Application-defined**: You define the meaning of each lock using a numeric identifier (int64)
- **Fast and efficient**: No table bloat, faster than row-level locks
- **Automatic cleanup**: Server automatically releases locks when sessions end
- **Lock stacking**: A session can acquire the same lock multiple times (requires equal unlocks)

From the [PostgreSQL documentation](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS):

> PostgreSQL provides a means for creating locks that have application-defined meanings. These are called advisory locks, because the system does not enforce their use — it is up to the application to use them correctly. Advisory locks can be useful for locking strategies that are an awkward fit for the MVCC model.

## API Reference

### Types

#### `Locker` Interface

```go
type Locker interface {
    Lock(ctx context.Context) (bool, error)
    WaitAndLock(ctx context.Context) error
    Unlock(ctx context.Context) error
    Close() error
}
```

#### `Lock` Struct

```go
type Lock struct {
    // contains filtered or unexported fields
}
```

### Functions

#### `NewLock(ctx context.Context, id int64, db *sql.DB) (Lock, error)`

Creates a new Lock instance with a dedicated database connection.

- `ctx`: Context for managing the connection acquisition
- `id`: The lock identifier (int64)
- `db`: Database connection pool
- Returns: Lock instance and error

#### `Lock(ctx context.Context) (bool, error)`

Attempts to acquire a lock without waiting. Returns immediately with true if acquired, false otherwise.

#### `WaitAndLock(ctx context.Context) error`

Blocks until the lock is acquired. Respects context cancellation and timeouts.

#### `Unlock(ctx context.Context) error`

Releases one level of lock ownership. Must be called equal to the number of Lock/WaitAndLock calls.

#### `Close() error`

Closes the database connection and releases all locks.

## Examples

### Basic Lock Usage

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/allisson/go-pglock/v3"
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()
    lockID := int64(100)

    // Create lock
    lock, err := pglock.NewLock(ctx, lockID, db)
    if err != nil {
        log.Fatal(err)
    }
    defer lock.Close()

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
    fmt.Println("Executing critical section...")
    // Your code here

    // Release lock
    if err := lock.Unlock(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Try Lock (Non-blocking)

Perfect for scenarios where you want to skip work if another process is already doing it.

```go
func processDataIfAvailable(db *sql.DB) error {
    ctx := context.Background()
    lockID := int64(200)

    lock, err := pglock.NewLock(ctx, lockID, db)
    if err != nil {
        return err
    }
    defer lock.Close()

    // Try to acquire without waiting
    acquired, err := lock.Lock(ctx)
    if err != nil {
        return err
    }

    if !acquired {
        fmt.Println("Another process is already processing data, skipping...")
        return nil
    }
    defer lock.Unlock(ctx)

    // Process data
    fmt.Println("Processing data...")
    // Your processing logic here
    
    return nil
}
```

### Wait and Lock (Blocking)

Use when you must execute the task eventually, even if you have to wait.

```go
func processDataAndWait(db *sql.DB) error {
    ctx := context.Background()
    lockID := int64(300)

    lock, err := pglock.NewLock(ctx, lockID, db)
    if err != nil {
        return err
    }
    defer lock.Close()

    fmt.Println("Waiting for lock...")
    
    // Wait until lock is available
    if err := lock.WaitAndLock(ctx); err != nil {
        return err
    }
    defer lock.Unlock(ctx)

    fmt.Println("Lock acquired, processing data...")
    // Your processing logic here
    
    return nil
}
```

### Lock with Timeout

Implement a timeout to avoid waiting indefinitely.

```go
func processWithTimeout(db *sql.DB, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    lockID := int64(400)

    lock, err := pglock.NewLock(ctx, lockID, db)
    if err != nil {
        return err
    }
    defer lock.Close()

    // This will fail if lock is not acquired within timeout
    if err := lock.WaitAndLock(ctx); err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("could not acquire lock within %v", timeout)
        }
        return err
    }
    defer lock.Unlock(context.Background()) // Use background for cleanup

    fmt.Println("Lock acquired, processing...")
    // Your processing logic here
    
    return nil
}
```

### Concurrent Workers

Coordinate multiple workers accessing a shared resource.

```go
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
    
    fmt.Printf("Worker %d: acquired lock, processing...\n", workerID)
    
    // Simulate work
    time.Sleep(1 * time.Second)
    
    fmt.Printf("Worker %d: releasing lock\n", workerID)
    lock.Unlock(ctx)
}

func main() {
    db, _ := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    defer db.Close()

    var wg sync.WaitGroup
    for i := 1; i <= 5; i++ {
        wg.Add(1)
        go runWorker(i, db, &wg)
    }
    wg.Wait()
}
```

### Distributed Task Execution

Ensure a task runs only once across multiple servers.

```go
type TaskProcessor struct {
    db *sql.DB
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
        return fmt.Errorf("task %s is already being processed", taskID)
    }
    defer lock.Unlock(ctx)

    fmt.Printf("Processing task %s...\n", taskID)
    
    // Execute task
    if err := tp.executeTask(taskID); err != nil {
        return fmt.Errorf("failed to execute task: %w", err)
    }

    fmt.Printf("Task %s completed\n", taskID)
    return nil
}

func (tp *TaskProcessor) executeTask(taskID string) error {
    // Your task execution logic
    time.Sleep(2 * time.Second)
    return nil
}

func hashToInt64(s string) int64 {
    h := fnv.New64a()
    h.Write([]byte(s))
    return int64(h.Sum64())
}
```

### Leader Election

Implement leader election in a cluster of services.

```go
type LeaderElector struct {
    db       *sql.DB
    lockID   int64
    isLeader bool
    mu       sync.RWMutex
}

func NewLeaderElector(db *sql.DB, clusterName string) *LeaderElector {
    return &LeaderElector{
        db:     db,
        lockID: hashToInt64(clusterName),
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

        fmt.Println("✓ Became leader")
        defer func() {
            le.mu.Lock()
            le.isLeader = false
            le.mu.Unlock()
            lock.Unlock(context.Background())
            fmt.Println("✗ Lost leadership")
        }()

        // Perform leader duties
        le.performLeaderDuties(ctx)
    } else {
        fmt.Println("Another instance is the leader")
    }

    return nil
}

func (le *LeaderElector) IsLeader() bool {
    le.mu.RLock()
    defer le.mu.RUnlock()
    return le.isLeader
}

func (le *LeaderElector) performLeaderDuties(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            fmt.Println("Leader performing periodic task...")
            // Do leader work
        }
    }
}
```

### Resource Pool Management

Manage a limited pool of resources across multiple processes.

```go
type ResourcePool struct {
    db       *sql.DB
    poolSize int
}

func NewResourcePool(db *sql.DB, poolSize int) *ResourcePool {
    return &ResourcePool{
        db:       db,
        poolSize: poolSize,
    }
}

// AcquireResource tries to acquire one resource from the pool
func (rp *ResourcePool) AcquireResource(ctx context.Context) (resourceID int, release func(), err error) {
    // Try each resource slot
    for i := 1; i <= rp.poolSize; i++ {
        lockID := int64(10000 + i) // Base offset + slot number
        
        lock, err := pglock.NewLock(ctx, lockID, rp.db)
        if err != nil {
            continue
        }

        // Try to acquire this slot (non-blocking)
        acquired, err := lock.Lock(ctx)
        if err != nil {
            lock.Close()
            continue
        }

        if acquired {
            // Successfully acquired this resource slot
            release := func() {
                lock.Unlock(context.Background())
                lock.Close()
            }
            return i, release, nil
        }

        lock.Close()
    }

    return 0, nil, fmt.Errorf("no resources available in pool")
}

func main() {
    db, _ := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    defer db.Close()

    pool := NewResourcePool(db, 3) // Pool of 3 resources

    ctx := context.Background()
    resourceID, release, err := pool.AcquireResource(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer release()

    fmt.Printf("Acquired resource %d\n", resourceID)
    
    // Use the resource
    time.Sleep(2 * time.Second)
    
    fmt.Printf("Releasing resource %d\n", resourceID)
}
```

### Database Migration Lock

Ensure database migrations run only once in multi-instance deployments.

```go
type MigrationRunner struct {
    db *sql.DB
}

func (mr *MigrationRunner) RunMigrations(ctx context.Context) error {
    const migrationLockID = int64(999999)

    lock, err := pglock.NewLock(ctx, migrationLockID, mr.db)
    if err != nil {
        return fmt.Errorf("failed to create migration lock: %w", err)
    }
    defer lock.Close()

    fmt.Println("Attempting to acquire migration lock...")

    // Use a timeout to avoid waiting too long
    lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := lock.WaitAndLock(lockCtx); err != nil {
        return fmt.Errorf("failed to acquire migration lock: %w", err)
    }
    defer lock.Unlock(context.Background())

    fmt.Println("Migration lock acquired, checking migration status...")

    // Check if migrations are needed
    needsMigration, err := mr.checkMigrationStatus()
    if err != nil {
        return err
    }

    if !needsMigration {
        fmt.Println("Database is up to date")
        return nil
    }

    // Run migrations
    fmt.Println("Running migrations...")
    if err := mr.executeMigrations(); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }

    fmt.Println("Migrations completed successfully")
    return nil
}

func (mr *MigrationRunner) checkMigrationStatus() (bool, error) {
    // Check if migrations are needed
    // This is application-specific logic
    return true, nil
}

func (mr *MigrationRunner) executeMigrations() error {
    // Execute your migrations
    time.Sleep(2 * time.Second) // Simulate migration work
    return nil
}
```

### Scheduled Job Coordination

Coordinate scheduled jobs across multiple instances to prevent duplicate execution.

```go
type ScheduledJob struct {
    db     *sql.DB
    jobID  string
    lockID int64
}

func NewScheduledJob(db *sql.DB, jobID string) *ScheduledJob {
    return &ScheduledJob{
        db:     db,
        jobID:  jobID,
        lockID: hashToInt64(jobID),
    }
}

func (sj *ScheduledJob) Execute(ctx context.Context) error {
    lock, err := pglock.NewLock(ctx, sj.lockID, sj.db)
    if err != nil {
        return fmt.Errorf("failed to create lock: %w", err)
    }
    defer lock.Close()

    // Try to acquire lock (non-blocking)
    acquired, err := lock.Lock(ctx)
    if err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }

    if !acquired {
        fmt.Printf("Job %s is already running on another instance\n", sj.jobID)
        return nil
    }
    defer lock.Unlock(ctx)

    fmt.Printf("Executing job %s...\n", sj.jobID)
    
    // Execute the actual job
    if err := sj.run(ctx); err != nil {
        return fmt.Errorf("job execution failed: %w", err)
    }

    fmt.Printf("Job %s completed\n", sj.jobID)
    return nil
}

func (sj *ScheduledJob) run(ctx context.Context) error {
    // Your job logic here
    select {
    case <-time.After(5 * time.Second):
        // Job completed
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func main() {
    db, _ := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    defer db.Close()

    // Simulate a cron job running every minute
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    ctx := context.Background()
    job := NewScheduledJob(db, "cleanup-task")

    for {
        select {
        case <-ticker.C:
            if err := job.Execute(ctx); err != nil {
                log.Printf("Job execution error: %v", err)
            }
        }
    }
}
```

## Best Practices

### 1. Always Close Locks

Use `defer` to ensure locks are closed even if errors occur:

```go
lock, err := pglock.NewLock(ctx, lockID, db)
if err != nil {
    return err
}
defer lock.Close() // Always close to release the connection
```

### 2. Match Lock and Unlock Calls

Locks stack, so ensure you unlock as many times as you lock:

```go
// Acquired twice
lock.Lock(ctx)
lock.Lock(ctx)

// Must unlock twice
lock.Unlock(ctx)
lock.Unlock(ctx)
```

### 3. Use Context Timeouts

Prevent indefinite waiting with context timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := lock.WaitAndLock(ctx); err != nil {
    // Handle timeout
}
```

### 4. Choose Appropriate Lock IDs

- Use meaningful, deterministic IDs based on resource names
- Use hash functions for string-based identifiers
- Document your lock ID allocation strategy

```go
// Good: Deterministic based on resource
lockID := hashToInt64("user-" + userID)

// Avoid: Random or non-deterministic IDs
lockID := rand.Int63() // Bad!
```

### 5. Handle Lock Acquisition Failures

Always check if lock acquisition succeeded:

```go
acquired, err := lock.Lock(ctx)
if err != nil {
    // Handle error
}
if !acquired {
    // Handle case where lock is held by another process
}
```

### 6. Use Connection Pooling Wisely

Each lock holds a dedicated connection. Consider your connection pool size:

```go
// Configure appropriate pool size
db.SetMaxOpenConns(50) // Ensure enough connections for locks + queries
```

### 7. Consider Lock Granularity

- Fine-grained locks: Better concurrency, more complex
- Coarse-grained locks: Simpler, but may reduce throughput

### 8. Testing with Locks

When testing code that uses locks, consider using different lock IDs per test:

```go
func TestMyFunction(t *testing.T) {
    lockID := int64(time.Now().UnixNano()) // Unique per test run
    // ... test code
}
```

## Testing

### Running Tests Locally

The project includes a Docker Compose setup for easy local testing:

```bash
# Start PostgreSQL and run tests
make test-local

# Run tests with race detector
make test-race

# Generate coverage report
make test-coverage
```

### Manual Testing

```bash
# Start PostgreSQL
docker-compose up -d

# Set DATABASE_URL
export DATABASE_URL='postgres://test:test@localhost:5432/pglock?sslmode=disable'

# Run tests
go test -v ./...

# Clean up
docker-compose down
```

## Troubleshooting

### "pq: database \"pglock\" does not exist"

Create the database:

```sql
CREATE DATABASE pglock;
```

### "too many connections"

Increase PostgreSQL's `max_connections` or reduce your application's connection pool size:

```go
db.SetMaxOpenConns(25) // Reduce if hitting connection limits
```

### Deadlocks

Advisory locks can deadlock if acquired in different orders. Always acquire locks in a consistent order:

```go
// Good: Consistent order
lockA := getLock(1)
lockB := getLock(2)

// Bad: Inconsistent order can cause deadlocks
if someCondition {
    lockA, then lockB
} else {
    lockB, then lockA
}
```

### Lock Not Released

Locks are automatically released when:
- `Unlock()` is called
- `Close()` is called
- Database connection closes
- Database session ends

If locks aren't releasing, check for:
- Missing `Unlock()` calls
- Connection leaks
- Application crashes before cleanup

### Context Deadline Exceeded

If you see context deadline errors, either:
- Increase the timeout
- Investigate why locks are held for so long
- Use non-blocking `Lock()` instead of `WaitAndLock()`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
