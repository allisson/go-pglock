# go-pglock
[![Build Status](https://github.com/allisson/go-pglock/workflows/tests/badge.svg)](https://github.com/allisson/go-pglock/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/allisson/go-pglock)](https://goreportcard.com/report/github.com/allisson/go-pglock)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/allisson/go-pglock)

Distributed locks using PostgreSQL session level advisory locks.

## About PostgreSQL Advisory Locks

From https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS:

PostgreSQL provides a means for creating locks that have application-defined meanings. These are called advisory locks, because the system does not enforce their use — it is up to the application to use them correctly. Advisory locks can be useful for locking strategies that are an awkward fit for the MVCC model. For example, a common use of advisory locks is to emulate pessimistic locking strategies typical of so-called "flat file" data management systems. While a flag stored in a table could be used for the same purpose, advisory locks are faster, avoid table bloat, and are automatically cleaned up by the server at the end of the session.

There are two ways to acquire an advisory lock in PostgreSQL: at session level or at transaction level. Once acquired at session level, an advisory lock is held until explicitly released or the session ends. Unlike standard lock requests, session-level advisory lock requests do not honor transaction semantics: a lock acquired during a transaction that is later rolled back will still be held following the rollback, and likewise an unlock is effective even if the calling transaction fails later. A lock can be acquired multiple times by its owning process; for each completed lock request there must be a corresponding unlock request before the lock is actually released. Transaction-level lock requests, on the other hand, behave more like regular lock requests: they are automatically released at the end of the transaction, and there is no explicit unlock operation. This behavior is often more convenient than the session-level behavior for short-term usage of an advisory lock. Session-level and transaction-level lock requests for the same advisory lock identifier will block each other in the expected way. If a session already holds a given advisory lock, additional requests by it will always succeed, even if other sessions are awaiting the lock; this statement is true regardless of whether the existing lock hold and new request are at session level or transaction level.

## Example

```golang
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/allisson/go-pglock"
	_ "github.com/lib/pq"
)

func newConn() (*sql.Conn, error) {
	// export DATABASE_URL='postgres://user:pass@localhost:5432/pglock?sslmode=disable'
	dsn := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db.Conn(context.Background())
}

func closeConn(conn *sql.Conn) {
	if err := conn.Close(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// Create two postgresql sessions
	conn1, err := newConn()
	if err != nil {
		log.Fatal(err)
	}
	defer closeConn(conn1)
	conn2, err := newConn()
	if err != nil {
		log.Fatal(err)
	}
	defer closeConn(conn2)

	// Set id and create locks
	id := int64(1)
	lock1 := pglock.NewLock(conn1)
	lock2 := pglock.NewLock(conn2)

	// lock1 get the lock
	ok, err := lock1.Lock(id)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("lock1.Lock(%v)==%v\n", id, ok)

	// lock2 try to get the lock
	ok, err = lock2.Lock(id)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("lock2.Lock(%v)==%v\n", id, ok)

	// lock1 release the lock
	if err := lock1.Unlock(id); err != nil {
		log.Fatal(err)
	}

	// lock2 try to get the lock again
	ok, err = lock2.Lock(id)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("lock2.Lock(%v)==%v\n", id, ok)
}
```

```go run main.go
lock1.Lock(1)==true
lock2.Lock(1)==false
lock2.Lock(1)==true
```
