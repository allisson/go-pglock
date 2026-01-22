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

// Acquire lock (non-blocking)
acquired, err := lock.Lock(ctx)

// Wait for lock (blocking)
err := lock.WaitAndLock(ctx)

// Release lock
err := lock.Unlock(ctx)
```

## API Quick Reference

| Method | Behavior | Returns | Use Case |
|--------|----------|---------|----------|
| `NewLock(ctx, id, db)` | Create lock instance | `Lock, error` | Initialize lock |
| `Lock(ctx)` | Try acquire (non-blocking) | `bool, error` | Skip if busy |
| `WaitAndLock(ctx)` | Wait for lock (blocking) | `error` | Must execute |
| `Unlock(ctx)` | Release one lock level | `error` | After work |
| `Close()` | Release all & cleanup | `error` | Shutdown |

## Common Patterns

### Pattern: Try Lock

```go
acquired, _ := lock.Lock(ctx)
if !acquired {
    return // Skip work
}
defer lock.Unlock(ctx)
// Do work
```

### Pattern: Wait for Lock

```go
if err := lock.WaitAndLock(ctx); err != nil {
    return err
}
defer lock.Unlock(ctx)
// Do work
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

### Blocking vs Non-blocking

| Method | Blocks? | Use When |
|--------|---------|----------|
| `Lock()` | No | Can skip if locked |
| `WaitAndLock()` | Yes | Must execute eventually |

### Lock Stacking

Locks **stack** within the same session:

```go
lock.Lock(ctx)    // Acquired (count: 1)
lock.Lock(ctx)    // Acquired (count: 2)
lock.Unlock(ctx)  // Released (count: 1) - still locked!
lock.Unlock(ctx)  // Released (count: 0) - now free
```

### Lock Release

Locks are released when:
- ✅ `Unlock()` called (decrements count)
- ✅ `Close()` called (releases all)
- ✅ Connection closes
- ✅ Database session ends

## Error Handling

```go
acquired, err := lock.Lock(ctx)
if err != nil {
    // Database or connection error
}
if !acquired {
    // Lock held by another process
}
```

```go
err := lock.WaitAndLock(ctx)
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
- Use context with timeouts for `WaitAndLock()`
- Match lock and unlock calls (stacking)
- Use deterministic lock IDs
- Check `acquired` return value

### ❌ DON'T

- Don't use random lock IDs
- Don't forget to unlock
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

| Use Case | Pattern | Lock Type |
|----------|---------|-----------|
| Scheduled jobs | Try Lock | Non-blocking |
| Database migrations | Wait + Timeout | Blocking |
| Leader election | Try Lock | Non-blocking |
| Task processing | Try Lock | Non-blocking |
| Resource pools | Try each slot | Non-blocking |
| Critical sections | Wait Lock | Blocking |

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
