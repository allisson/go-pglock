package pglock

import (
	"context"
	"database/sql"
	"log"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func assertEqual(t *testing.T, expected, current interface{}) {
	t.Helper()

	if !reflect.DeepEqual(expected, current) {
		t.Fatalf("assertion_type=Equal, expected_value=%#v, expected_type=%T, current_value=%#v, current_type=%T", expected, expected, current, current)
	}
}

func newConn() (*sql.Conn, error) {
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

func waitAndLock(id int64, l *Lock, wg *sync.WaitGroup) {
	if err := l.WaitAndLock(id); err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Duration(500) * time.Millisecond)
	if err := l.Unlock(id); err != nil {
		log.Fatal(err)
	}
	wg.Done()
}

func TestNewLock(t *testing.T) {
	conn, err := newConn()
	assertEqual(t, nil, err)
	defer closeConn(conn)

	lock := NewLock(conn)
	assertEqual(t, conn, lock.conn)
}

func TestLockUnlock(t *testing.T) {
	conn1, err := newConn()
	assertEqual(t, nil, err)
	defer closeConn(conn1)
	conn2, err := newConn()
	assertEqual(t, nil, err)
	defer closeConn(conn2)

	id := int64(1)
	lock1 := NewLock(conn1)
	lock2 := NewLock(conn2)

	ok, err := lock1.Lock(id)
	assertEqual(t, true, ok)
	assertEqual(t, nil, err)

	ok, err = lock2.Lock(id)
	assertEqual(t, false, ok)
	assertEqual(t, nil, err)

	err = lock1.Unlock(id)
	assertEqual(t, nil, err)

	ok, err = lock2.Lock(id)
	assertEqual(t, true, ok)
	assertEqual(t, nil, err)

	err = lock2.Unlock(id)
	assertEqual(t, nil, err)
}

func TestWaitAndLock(t *testing.T) {
	conn1, err := newConn()
	assertEqual(t, nil, err)
	defer closeConn(conn1)
	conn2, err := newConn()
	assertEqual(t, nil, err)
	defer closeConn(conn2)

	wg := sync.WaitGroup{}
	id := int64(1)
	lock1 := NewLock(conn1)
	lock2 := NewLock(conn2)

	start := time.Now()
	wg.Add(1)
	go waitAndLock(id, &lock1, &wg) // wait for 500 milliseconds
	wg.Add(1)
	go waitAndLock(id, &lock2, &wg) // wait for 500 milliseconds
	wg.Wait()
	stop := time.Since(start)
	assertEqual(t, true, stop.Milliseconds() >= 1000)
}
