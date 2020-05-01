package pglock

import (
	"database/sql"
	"sync"
)

// Locker is an interface for postgresql advisory locks.
type Locker interface {
	Lock(id int64) (bool, error)
	WaitAndLock(id int64) error
	Unlock(id int64) error
}

// Lock implements the Locker interface.
type Lock struct {
	db *sql.DB
	mu sync.Mutex
}

// Lock obtains exclusive session level advisory lock if available.
// It’s similar to WaitAndLock, except it will not wait for the lock to become available.
// It will either obtain the lock and return true, or return false if the lock cannot be acquired immediately.
func (l *Lock) Lock(id int64) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := false
	sqlQuery := "SELECT pg_try_advisory_lock($1)"
	err := l.db.QueryRow(sqlQuery, id).Scan(&result)
	return result, err
}

// WaitAndLock obtains exclusive session level advisory lock.
// If another session already holds a lock on the same resource identifier, this function will wait until the resource becomes available.
// Multiple lock requests stack, so that if the resource is locked three times it must then be unlocked three times.
func (l *Lock) WaitAndLock(id int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sqlQuery := "SELECT pg_advisory_lock($1)"
	_, err := l.db.Exec(sqlQuery, id)
	return err
}

// Unlock releases the lock.
func (l *Lock) Unlock(id int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sqlQuery := "SELECT pg_advisory_unlock($1)"
	_, err := l.db.Exec(sqlQuery, id)
	return err
}

// NewLock returns a Lock with *sql.DB
func NewLock(db *sql.DB) Lock {
	return Lock{db: db}
}
