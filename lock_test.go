package pglock

import (
	"context"
	"database/sql"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func newDB() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func closeDB(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
}

func waitAndLock(id int64, l *Lock, wg *sync.WaitGroup) {
	ctx := context.Background()
	if err := l.WaitAndLock(ctx); err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Duration(500) * time.Millisecond)
	if err := l.Unlock(ctx); err != nil {
		log.Fatal(err)
	}
	wg.Done()
}

func TestNewLock(t *testing.T) {
	db, err := newDB()
	assert.Equal(t, nil, err)
	defer closeDB(db)
	id := int64(10)
	lock, err := NewLock(context.Background(), id, db)
	assert.Nil(t, err)
	assert.Equal(t, id, lock.id)
	assert.NotNil(t, lock.conn)
}

func TestLockUnlock(t *testing.T) {
	db1, err := newDB()
	assert.Nil(t, err)
	defer closeDB(db1)
	db2, err := newDB()
	assert.Nil(t, err)
	defer closeDB(db2)

	ctx := context.Background()
	id := int64(1)
	lock1, err := NewLock(ctx, id, db1)
	assert.Nil(t, err)
	lock2, err := NewLock(ctx, id, db2)
	assert.Nil(t, err)

	ok, err := lock1.Lock(ctx)
	assert.True(t, ok)
	assert.Nil(t, err)

	ok, err = lock2.Lock(ctx)
	assert.False(t, ok)
	assert.Nil(t, err)

	err = lock1.Unlock(ctx)
	assert.Nil(t, err)

	ok, err = lock2.Lock(ctx)
	assert.True(t, ok)
	assert.Nil(t, err)

	err = lock2.Unlock(ctx)
	assert.Nil(t, err)
}

func TestWaitAndLock(t *testing.T) {
	db1, err := newDB()
	assert.Nil(t, err)
	defer closeDB(db1)
	db2, err := newDB()
	assert.Nil(t, err)
	defer closeDB(db2)

	ctx := context.Background()
	wg := sync.WaitGroup{}
	id := int64(1)
	lock1, err := NewLock(ctx, id, db1)
	assert.Nil(t, err)
	lock2, err := NewLock(ctx, id, db2)
	assert.Nil(t, err)

	start := time.Now()
	wg.Add(1)
	go waitAndLock(id, &lock1, &wg) // wait for 500 milliseconds
	wg.Add(1)
	go waitAndLock(id, &lock2, &wg) // wait for 500 milliseconds
	wg.Wait()
	stop := time.Since(start)
	assert.True(t, stop.Milliseconds() >= 1000)
}
