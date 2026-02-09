# go-pglock Quick Reference

## Installation

```bash
go get github.com/allisson/go-pglock/v3
```

## Basic Usage

```go
import "github.com/allisson/go-pglock/v3"

// Connect to database
db, _ := sql.Open("postgres", "postgres://user:pass@host/db?sslmode=disable")

// Create lock
ctx := context.Background()
lock, err := pglock.NewLock(ctx, lockID, db)
defer lock.Close()

// Acquire exclusive lock (non-blocking)
acquired, err := lock.Lock(ctx)

// Acquire shared lock (non-blocking)
acquired, err := lock.RLock(ctx)

// Wait for exclusive lock (blocking)
err := lock.WaitAndLock(ctx)

// Wait for shared lock (blocking)
err := lock.WaitAndRLock(ctx)

// Release exclusive lock
err := lock.Unlock(ctx)

// Release shared lock
err := lock.RUnlock(ctx)
```

## API Quick Reference

### Exclusive Locks (Write Locks)

| Method | Behavior | Returns | Use Case |
|--------|----------|---------|----------|
| `Lock(ctx)` | Try acquire (non-blocking) | `bool, error` | Skip if busy |
| `WaitAndLock(ctx)` | Wait for lock (blocking) | `error` | Must execute |
| `Unlock(ctx)` | Release one lock level | `error` | After work |

### Shared Locks (Read Locks)

| Method | Behavior | Returns | Use Case |
|--------|----------|---------|----------|
| `RLock(ctx)` | Try acquire shared (non-blocking) | `bool, error` | Multiple readers |
| `WaitAndRLock(ctx)` | Wait for shared (blocking) | `error` | Must read |
| `RUnlock(ctx)` | Release one shared lock level | `error` | After read |

### General

| Method | Behavior | Returns | Use Case |
|--------|----------|---------|----------|
| `NewLock(ctx, id, db)` | Create lock instance | `Lock, error` | Initialize lock |
| `Close()` | Release all & cleanup | `error` | Shutdown |

## Common Patterns

### Pattern: Try Exclusive Lock

```go
acquired, _ := lock.Lock(ctx)
if !acquired {
    return // Skip work
}
defer lock.Unlock(ctx)
// Do work
```

### Pattern: Try Shared Lock (Multiple Readers)

```go
acquired, _ := lock.RLock(ctx)
if !acquired {
    return // Writer is working
}
defer lock.RUnlock(ctx)
// Read data (multiple readers can do this concurrently)
```

### Pattern: Wait for Exclusive Lock

```go
if err := lock.WaitAndLock(ctx); err != nil {
    return err
}
defer lock.Unlock(ctx)
// Do work
```

### Pattern: Wait for Shared Lock

```go
if err := lock.WaitAndRLock(ctx); err != nil {
    return err
}
defer lock.RUnlock(ctx)
// Read data
```

### Pattern: Lock with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := lock.WaitAndLock(ctx); err != nil {
    return err // Timeout or other error
}
defer lock.Unlock(context.Background())
// Do work
```

### Pattern: Unique Lock per Resource

```go
import "hash/fnv"

func lockIDFromString(s string) int64 {
    h := fnv.New64a()
    h.Write([]byte(s))
    return int64(h.Sum64())
}

lockID := lockIDFromString("user-" + userID)
```

## Lock Behavior

### Lock Types

| Lock Type | Symbol | Conflicts With | Use Case |
|-----------|--------|----------------|----------|
| Exclusive | `Lock()` | All locks | Write/modify data |
| Shared | `RLock()` | Exclusive only | Read data |

### Lock Compatibility

| Current Lock | Lock() | RLock() |
|--------------|--------|----------|
| None | ✅ Yes | ✅ Yes |
| Exclusive | ❌ No | ❌ No |
| Shared | ❌ No | ✅ Yes |

### Blocking vs Non-blocking

| Method | Blocks? | Use When |
|--------|---------|----------|
| `Lock()` | No | Can skip if locked (exclusive) |
| `RLock()` | No | Can skip if locked (shared) |
| `WaitAndLock()` | Yes | Must execute eventually (exclusive) |
| `WaitAndRLock()` | Yes | Must read eventually (shared) |

### Lock Stacking

Locks **stack** within the same session (applies to both exclusive and shared locks):

```go
// Exclusive lock stacking
lock.Lock(ctx)    // Acquired (count: 1)
lock.Lock(ctx)    // Acquired (count: 2)
lock.Unlock(ctx)  // Released (count: 1) - still locked!
lock.Unlock(ctx)  // Released (count: 0) - now free

// Shared lock stacking
lock.RLock(ctx)    // Acquired (count: 1)
lock.RLock(ctx)    // Acquired (count: 2)
lock.RUnlock(ctx)  // Released (count: 1) - still locked!
lock.RUnlock(ctx)  // Released (count: 0) - now free
```

### Lock Release

Locks are released when:
- ✅ `Unlock()` called (decrements count)
- ✅ `Close()` called (releases all)
- ✅ Connection closes
- ✅ Database session ends

## Error Handling

### Exclusive Locks

```go
acquired, err := lock.Lock(ctx)
if err != nil {
    // Database or connection error
}
if !acquired {
    // Lock held by another process
}
```

### Shared Locks

```go
acquired, err := lock.RLock(ctx)
if err != nil {
    // Database or connection error
}
if !acquired {
    // Exclusive lock held by another process
}
```

### Blocking Locks

```go
err := lock.WaitAndLock(ctx)  // or WaitAndRLock(ctx)
if errors.Is(err, context.DeadlineExceeded) {
    // Timeout occurred
}
if errors.Is(err, context.Canceled) {
    // Context was cancelled
}
```

## Best Practices

### ✅ DO

- Always `defer lock.Close()`
- Use context with timeouts for `WaitAndLock()` and `WaitAndRLock()`
- Match lock and unlock calls (stacking)
- Match lock types: `Lock()`→`Unlock()`, `RLock()`→`RUnlock()`
- Use deterministic lock IDs
- Check `acquired` return value
- Use `RLock()` for read-heavy workloads
- Use `Lock()` when modifying data

### ❌ DON'T

- Don't use random lock IDs
- Don't forget to unlock
- Don't mix `Lock()` with `RUnlock()` or vice versa
- Don't acquire in inconsistent order (deadlock)
- Don't share Lock instances across goroutines
- Don't rely on lock after `Close()`

## Testing

```bash
# Start PostgreSQL
docker-compose up -d

# Run tests
make test-local

# With race detector
make test-race
```

## Use Cases at a Glance

| Use Case | Lock Type | Pattern | Method |
|----------|-----------|---------|--------|
| Scheduled jobs | Exclusive | Try Lock | `Lock()` |
| Database migrations | Exclusive | Wait + Timeout | `WaitAndLock()` |
| Leader election | Exclusive | Try Lock | `Lock()` |
| Task processing | Exclusive | Try Lock | `Lock()` |
| Resource pools | Exclusive | Try each slot | `Lock()` |
| Critical sections | Exclusive | Wait Lock | `WaitAndLock()` |
| Read cached data | Shared | Try/Wait | `RLock()` / `WaitAndRLock()` |
| View reports | Shared | Try Lock | `RLock()` |
| Read config | Shared | Try Lock | `RLock()` |
| Update config | Exclusive | Wait Lock | `WaitAndLock()` |
| Generate reports | Exclusive | Try Lock | `Lock()` |

## Connection Management

```go
// Configure pool for locks
db.SetMaxOpenConns(50)  // Each lock uses one connection
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(time.Hour)
```

## Lock ID Strategies

### Strategy 1: Sequential IDs

```go
const (
    LockIDMigration = 1
    LockIDBackup    = 2
    LockIDCleanup   = 3
)
```

### Strategy 2: Hash-based

```go
lockID := hashToInt64("resource-name")
```

### Strategy 3: Composite

```go
lockID := (resourceType << 32) | resourceID
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Lock never released | Add `defer lock.Close()` |
| Too many connections | Reduce pool size or close locks |
| Deadlocks | Acquire locks in consistent order |
| Timeouts | Increase timeout or investigate blocking |
| Tests skip | Set `DATABASE_URL` environment variable |

## Resources

- [Full Documentation](README.md)
- [Examples](examples/)
- [API Reference](https://pkg.go.dev/github.com/allisson/go-pglock/v3)
- [PostgreSQL Advisory Locks](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS)
