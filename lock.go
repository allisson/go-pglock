// Package pglock provides distributed locks using PostgreSQL session level advisory locks.
//
// PostgreSQL advisory locks are application-defined locks that can be useful for locking
// strategies that are an awkward fit for the MVCC model. They are faster than table-based
// locking mechanisms, avoid table bloat, and are automatically cleaned up by the server
// at the end of the session.
//
// This package uses session-level advisory locks, which are held until explicitly released
// or the session ends. Unlike transaction-level locks, session-level advisory locks do not
// honor transaction semantics: a lock acquired during a transaction that is later rolled
// back will still be held following the rollback.
package pglock

import (
	"context"
	"database/sql"
)

// Locker is an interface for PostgreSQL advisory locks.
//
// All methods use PostgreSQL session-level advisory locks, which persist until
// explicitly released or the database connection is closed. These locks are based
// on a numeric identifier and can be used to coordinate access to shared resources
// across multiple database connections.
type Locker interface {
	Lock(ctx context.Context) (bool, error)
	RLock(ctx context.Context) (bool, error)
	WaitAndLock(ctx context.Context) error
	WaitAndRLock(ctx context.Context) error
	Unlock(ctx context.Context) error
	RUnlock(ctx context.Context) error
	Close() error
}

// Lock implements the Locker interface using PostgreSQL advisory locks.
//
// A Lock holds a dedicated database connection and a lock identifier.
// The connection is obtained from a connection pool and is held for the
// lifetime of the Lock instance to maintain the session-level advisory lock.
type Lock struct {
	id   int64
	conn *sql.Conn
}

// Lock attempts to obtain an exclusive session level advisory lock without waiting.
//
// This method uses PostgreSQL's pg_try_advisory_lock function, which is non-blocking.
// It will either obtain the lock immediately and return true, or return false if the
// lock is already held by another session. This is similar to WaitAndLock, except it
// will not wait for the lock to become available.
//
// Multiple lock requests stack within the same session, meaning if a resource is
// locked three times, it must be unlocked three times to be fully released.
//
// Returns true if the lock was successfully acquired, false if it's already held
// by another session, and an error if the database operation fails.
func (l *Lock) Lock(ctx context.Context) (bool, error) {
	result := false
	sqlQuery := "SELECT pg_try_advisory_lock($1)"
	err := l.conn.QueryRowContext(ctx, sqlQuery, l.id).Scan(&result)
	return result, err
}

// RLock attempts to obtain a shared session level advisory lock without waiting.
//
// This method uses PostgreSQL's pg_try_advisory_lock_shared function, which is non-blocking.
// It will either obtain the shared lock immediately and return true, or return false if an
// exclusive lock is already held by another session. Multiple sessions can hold shared locks
// simultaneously, but shared locks conflict with exclusive locks.
//
// Shared locks are ideal for read operations where multiple readers can safely access a
// resource concurrently, but writers need to be prevented from modifying it during the read.
//
// Multiple lock requests stack within the same session, meaning if a shared lock is acquired
// three times, it must be released three times to be fully released.
//
// Returns true if the shared lock was successfully acquired, false if an exclusive lock is
// held by another session, and an error if the database operation fails.
func (l *Lock) RLock(ctx context.Context) (bool, error) {
	result := false
	sqlQuery := "SELECT pg_try_advisory_lock_shared($1)"
	err := l.conn.QueryRowContext(ctx, sqlQuery, l.id).Scan(&result)
	return result, err
}

// WaitAndLock obtains an exclusive session level advisory lock, waiting if necessary.
//
// This method uses PostgreSQL's pg_advisory_lock function, which will block until
// the lock becomes available. If another session already holds a lock on the same
// resource identifier, this function will wait until the resource becomes available.
//
// Multiple lock requests stack within the same session, meaning if a resource is
// locked three times, it must be unlocked three times to be fully released. If the
// session already holds the given advisory lock, additional requests will always
// succeed immediately.
//
// The lock persists until explicitly released via Unlock or until the session ends.
// Returns an error if the database operation fails or if the context is cancelled
// while waiting for the lock.
func (l *Lock) WaitAndLock(ctx context.Context) error {
	sqlQuery := "SELECT pg_advisory_lock($1)"
	_, err := l.conn.ExecContext(ctx, sqlQuery, l.id)
	return err
}

// WaitAndRLock obtains a shared session level advisory lock, waiting if necessary.
//
// This method uses PostgreSQL's pg_advisory_lock_shared function, which will block until
// the lock becomes available. If another session holds an exclusive lock on the same
// resource identifier, this function will wait until the exclusive lock is released.
// Multiple sessions can hold shared locks concurrently.
//
// Shared locks are ideal for read operations where multiple readers can safely access a
// resource at the same time, but need to prevent writers from modifying it during reads.
//
// Multiple lock requests stack within the same session, meaning if a shared lock is acquired
// three times, it must be released three times to be fully released. If the session already
// holds the given shared advisory lock, additional requests will always succeed immediately.
//
// The lock persists until explicitly released via RUnlock or until the session ends.
// Returns an error if the database operation fails or if the context is cancelled
// while waiting for the lock.
func (l *Lock) WaitAndRLock(ctx context.Context) error {
	sqlQuery := "SELECT pg_advisory_lock_shared($1)"
	_, err := l.conn.ExecContext(ctx, sqlQuery, l.id)
	return err
}

// Unlock releases a previously acquired advisory lock.
//
// This method uses PostgreSQL's pg_advisory_unlock function to release one level
// of lock ownership. Because lock requests stack within a session, each Unlock call
// only decrements the lock count by one. If the same lock was acquired multiple times,
// it must be unlocked the same number of times to be fully released.
//
// Note that unlocking a lock that is not currently held will not return an error,
// but may have unexpected consequences in PostgreSQL. It's the caller's responsibility
// to ensure locks and unlocks are properly paired.
//
// Returns an error if the database operation fails.
func (l *Lock) Unlock(ctx context.Context) error {
	sqlQuery := "SELECT pg_advisory_unlock($1)"
	_, err := l.conn.ExecContext(ctx, sqlQuery, l.id)
	return err
}

// RUnlock releases a previously acquired shared advisory lock.
//
// This method uses PostgreSQL's pg_advisory_unlock_shared function to release one level
// of shared lock ownership. Because shared lock requests stack within a session, each
// RUnlock call only decrements the shared lock count by one. If the same shared lock was
// acquired multiple times, it must be unlocked the same number of times to be fully released.
//
// Note that unlocking a shared lock that is not currently held will not return an error,
// but may have unexpected consequences in PostgreSQL. It's the caller's responsibility
// to ensure shared locks and unlocks are properly paired, and to use RUnlock only for
// locks acquired with RLock or WaitAndRLock (not with Lock or WaitAndLock).
//
// Returns an error if the database operation fails.
func (l *Lock) RUnlock(ctx context.Context) error {
	sqlQuery := "SELECT pg_advisory_unlock_shared($1)"
	_, err := l.conn.ExecContext(ctx, sqlQuery, l.id)
	return err
}

// Close closes the database connection, releasing all advisory locks held by this Lock.
//
// Since advisory locks are automatically cleaned up when a database session ends,
// closing the connection will release all locks held on this connection, regardless
// of how many times they were acquired. This provides a reliable way to ensure all
// locks are released when the Lock instance is no longer needed.
//
// After calling Close, the Lock instance should not be used for any further operations.
// Returns an error if closing the connection fails.
func (l *Lock) Close() error {
	return l.conn.Close()
}

// NewLock creates a new Lock instance with a dedicated database connection.
//
// This function obtains a connection from the provided database connection pool
// and stores it for use in lock and unlock operations. The connection is held for
// the lifetime of the Lock instance to maintain session-level advisory locks.
//
// Parameters:
//   - ctx: Context for managing the connection acquisition
//   - id: The lock identifier (a 64-bit integer used as the PostgreSQL advisory lock key)
//   - db: A database connection pool from which to obtain a dedicated connection
//
// The caller is responsible for calling Close on the returned Lock to release
// the connection back to the pool and clean up any held advisory locks.
//
// Returns a Lock instance and an error if the connection cannot be obtained.
func NewLock(ctx context.Context, id int64, db *sql.DB) (Lock, error) {
	// Obtain a connection from the DB connection pool and store it and use it for lock and unlock operations
	conn, err := db.Conn(ctx)
	if err != nil {
		return Lock{}, err
	}
	return Lock{id: id, conn: conn}, nil
}
