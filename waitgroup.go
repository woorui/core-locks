package core_locks

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrUnlockTimeout = errors.New("wait unlock timeout")

// WithLockWait lets the current call wait if its locked by others.
// timeout is the waiting timeout, do not perform timeout if timeout is less than 0.
func WithLockWait(timeout time.Duration) func(c *LockManager) {
	return func(c *LockManager) {
		if timeout > 0 {
			c.waitLock = true
		}
		c.waitgroup = group{timeout: timeout}
	}
}

type group struct {
	mu      sync.Mutex
	m       map[string]*call
	timeout time.Duration
}

type call struct {
	wg   sync.WaitGroup
	dups int
}

func (g *group) addWithContext(ctx context.Context, key string) (bool, error) {
	sharedch := make(chan bool, 1)

	go func() {
		sharedch <- g.add(key)
	}()

	defer func() {
		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(g.timeout):
		return false, ErrUnlockTimeout
	case shared := <-sharedch:
		return shared, nil
	}
}

func (g *group) add(key string) (shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	c, ok := g.m[key]

	if ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return
	}
	c = new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	shared = c.dups > 0
	return
}
