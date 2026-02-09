# 💡 go-pglock Examples

This directory contains practical examples demonstrating various use cases for `go-pglock`.

## 📋 Prerequisites

1. 🐘 PostgreSQL running and accessible
2. 🗄️ Database created (default: `pglock`)
3. 🔧 Set `DATABASE_URL` environment variable (optional)

```bash
# Using docker-compose from the project root
docker-compose up -d

# Or set custom database URL
export DATABASE_URL='postgres://user:pass@localhost:5432/dbname?sslmode=disable'
```

## 🚀 Running Examples

### 🔰 Basic Lock Usage

Demonstrates the fundamental lock acquire and release pattern.

```bash
cd examples/basic
go run main.go
```

**What it demonstrates:**
- 🔒 Creating a lock
- ✅ Acquiring a lock with `Lock()` (non-blocking)
- ⚙️ Performing work in a critical section
- 🔓 Releasing the lock

**Try it:**
Run two instances in different terminals to see one acquire the lock while the other is blocked.

### 👷 Concurrent Workers

Shows how multiple workers coordinate access to a shared resource.

```bash
cd examples/workers
go run main.go
```

**What it demonstrates:**
- 🏃 Multiple goroutines competing for the same lock
- ⏳ `WaitAndLock()` blocking behavior
- 📊 Sequential execution enforced by the lock
- 🧹 Proper cleanup in concurrent scenarios

**Expected output:**
- ⏸️ Workers start and wait for the lock
- 🔄 Workers execute one at a time (serialized)
- ⏱️ Each worker holds the lock for ~1 second

### 👑 Leader Election

Implements a simple leader election mechanism.

```bash
cd examples/leader-election
go run main.go
```

**What it demonstrates:**
- 🗳️ Leader election pattern
- ✅ Only one instance becomes the leader
- 📅 Leader performing periodic tasks
- 👥 Followers waiting/skipping work

**Use cases:**
- 🌐 Distributed systems where only one instance should perform certain tasks
- 🔄 Active-passive failover scenarios
- 🤝 Coordinating cluster-wide operations

### ⏱️ Timeout Handling

Shows how to use context timeouts with lock operations.

```bash
cd examples/timeout
go run main.go
```

**What it demonstrates:**
- ⏰ Creating locks with timeout contexts
- ⚠️ Handling `context.DeadlineExceeded` errors
- 🎚️ Different timeout values and their effects
- 🧹 Proper cleanup when timeouts occur

**Expected output:**
- ❌ First attempt fails due to short timeout
- ✅ Second attempt succeeds with longer timeout

### ⚙️ Task Processing

Demonstrates distributed task processing with lock-based coordination.

```bash
cd examples/task-processing
go run main.go
```

**What it demonstrates:**
- 🏷️ Using task IDs to generate lock IDs
- 🚫 Preventing duplicate task execution
- #️⃣ Hash-based lock ID generation
- ⚠️ Handling already-processing scenarios

**Use cases:**
- 📬 Job queues in distributed systems
- ♻️ Idempotent task processing
- 🔒 Preventing race conditions in task execution

### 📖 Read-Write Locks

Demonstrates shared (read) and exclusive (write) locks for concurrent reader scenarios.

```bash
cd examples/read-write-locks
go run main.go
```

**What it demonstrates:**
- 👥 Shared locks (`RLock()`) allowing multiple concurrent readers
- 🔒 Exclusive locks (`Lock()`) blocking all other operations
- 🔄 How readers and writers interact
- 📊 Three scenarios: multiple readers, writer blocking readers, readers blocking writer

**Use cases:**
- 📚 Read-heavy workloads with occasional writes
- 💾 Cache management with multiple readers
- ⚙️ Configuration management
- 📊 Report viewing with generation locks
- 🗄️ Database record access coordination

**Expected output:**
- ✅ Scenario 1: Multiple readers acquire locks simultaneously
- ⏳ Scenario 2: Writer blocks readers until write completes
- ⏸️ Scenario 3: Readers block writer until all reads complete

## 🔄 Running Multiple Instances

To see the distributed locking in action, run the same example in multiple terminal windows:

```bash
# 💻 Terminal 1
cd examples/basic
go run main.go

# 💻 Terminal 2 (run simultaneously)
cd examples/basic
go run main.go
```

You'll see that only one instance acquires the lock while others wait or skip.

## 🛠️ Customizing Examples

All examples use the same database connection pattern:

```go
dsn := os.Getenv("DATABASE_URL")
if dsn == "" {
    dsn = "postgres://test:test@localhost:5432/pglock?sslmode=disable"
}
```

You can customize the connection by:
1. 🔧 Setting the `DATABASE_URL` environment variable
2. ✏️ Modifying the default DSN in the code
3. 🔌 Using connection parameters in the DSN string

## 💡 Common Patterns

### Pattern 1: Try Exclusive Lock (Non-blocking)

Use when you want to skip work if another instance is already doing it:

```go
acquired, err := lock.Lock(ctx)
if !acquired {
    return // ⏭️ Skip work
}
defer lock.Unlock(ctx)
// ✅ Do work
```

### Pattern 2: Try Shared Lock (Non-blocking)

Use when you want to read data but skip if a writer is active:

```go
acquired, err := lock.RLock(ctx)
if !acquired {
    return // ⏭️ Skip reading, writer is active
}
defer lock.RUnlock(ctx)
// 📖 Read data (multiple readers can do this concurrently)
```

### Pattern 3: Wait for Exclusive Lock (Blocking)

Use when work must be done eventually:

```go
if err := lock.WaitAndLock(ctx); err != nil {
    return err
}
defer lock.Unlock(ctx)
// ✅ Do work
```

### Pattern 4: Wait for Shared Lock (Blocking)

Use when you must read data eventually:

```go
if err := lock.WaitAndRLock(ctx); err != nil {
    return err
}
defer lock.RUnlock(ctx)
// 📖 Read data
```

### Pattern 5: Lock with Timeout

Use when you want to wait but not indefinitely:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := lock.WaitAndLock(ctx); err != nil {
    // ⚠️ Handle timeout or other errors
}
defer lock.Unlock(context.Background())
// ✅ Do work
```

## 🏗️ Building Examples

To build all examples:

```bash
# From the examples directory
for dir in */; do
    echo "Building ${dir%/}..."
    (cd "$dir" && go build -o "../../bin/${dir%/}" .)
done
```

This creates binaries in the `bin/` directory.

## 🔧 Troubleshooting

### ❌ "DATABASE_URL environment variable not set"

Either set the environment variable or use the default connection string in the code.

### ⚠️ "pq: database does not exist"

Create the database:
```sql
CREATE DATABASE pglock;
```

### 🔌 "connection refused"

Make sure PostgreSQL is running:
```bash
docker-compose up -d
```

### 🔓 Lock Never Released

Check that:
- ✅ `defer lock.Close()` is present
- 🚫 Program doesn't panic before cleanup
- ⏱️ Context isn't cancelled prematurely

## 📚 Further Reading

- 🐘 [PostgreSQL Advisory Locks Documentation](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS)
- 📖 [Main README](../README.md) for more examples and patterns
- 🔍 [API Documentation](https://pkg.go.dev/github.com/allisson/go-pglock/v3)
