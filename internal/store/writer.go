package store

import (
	"context"
	"sync"
)

// WriterLock keeps mutation/snapshot/media serialization while allowing canceled
// HTTP waiters to leave without waiting for the current mutation to finish.
// Its zero value is ready to use, like sync.Mutex.
type WriterLock struct {
	once  sync.Once
	token chan struct{}
}

func (m *WriterLock) initialize() {
	m.once.Do(func() { m.token = make(chan struct{}, 1); m.token <- struct{}{} })
}
func (m *WriterLock) Lock() { _ = m.LockContext(context.Background()) }
func (m *WriterLock) LockContext(ctx context.Context) error {
	m.initialize()
	if e := ctx.Err(); e != nil {
		return e
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.token:
		if e := ctx.Err(); e != nil {
			m.Unlock()
			return e
		}
		return nil
	}
}
func (m *WriterLock) Unlock() { m.token <- struct{}{} }
