package pglock

import (
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

func newSQLDB() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func closeSQLDB(db *sql.DB) {
	if err := db.Close(); err != nil {
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
	sqlDB, err := newSQLDB()
	assertEqual(t, nil, err)
	defer closeSQLDB(sqlDB)

	lock := NewLock(sqlDB)
	assertEqual(t, sqlDB, lock.db)
}

func TestLockUnlock(t *testing.T) {
	db1, err := newSQLDB()
	assertEqual(t, nil, err)
	defer closeSQLDB(db1)
	db2, err := newSQLDB()
	assertEqual(t, nil, err)
	defer closeSQLDB(db2)

	id := int64(1)
	lock1 := NewLock(db1)
	lock2 := NewLock(db2)

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
	db1, err := newSQLDB()
	assertEqual(t, nil, err)
	defer closeSQLDB(db1)
	db2, err := newSQLDB()
	assertEqual(t, nil, err)
	defer closeSQLDB(db2)

	wg := sync.WaitGroup{}
	id := int64(1)
	lock1 := NewLock(db1)
	lock2 := NewLock(db2)

	start := time.Now()
	wg.Add(1)
	go waitAndLock(id, &lock1, &wg) // wait for 500 milliseconds
	wg.Add(1)
	go waitAndLock(id, &lock2, &wg) // wait for 500 milliseconds
	wg.Wait()
	stop := time.Since(start)
	assertEqual(t, true, stop.Milliseconds() >= 1000)
}
