package proc

import (
	"context"
	"errors"
	"sync"
)

func MakeChannel[T any](buffer int) *Channel[T] {
	return &Channel[T]{ch: make(chan T, buffer)}
}

// Channel is channel that you can tell if it closed
type Channel[T any] struct {
	closed Atomic[bool]
	ch     chan T
}

func (c *Channel[T]) TrySend(v T) bool {
	if c.closed.Load() {
		return false
	}

	select {
	case c.ch <- v:
		return true
	default:
		return false
	}
}

func (c *Channel[T]) Send(v T) {
	if c.closed.Load() {
		return
	}

	c.ch <- v
}

func (c *Channel[T]) TryRecv() (T, bool) {
	var v T
	if c.closed.Load() {
		return v, false
	}

	select {
	case rVal := <-c.ch:
		return rVal, true
	default:
	}
	return v, false
}

func (c *Channel[T]) Recv() (T, bool) {
	var v T
	if c.closed.Load() {
		return v, false
	}

	rVal, ok := <-c.ch
	if !ok {
		return v, false
	}
	return rVal, true
}

func (c *Channel[T]) Closed() bool {
	return c.closed.Load()
}

func (c *Channel[T]) Close() {
	if c.closed.Load() {
		return
	}

	c.closed.Locker.Lock()
	close(c.ch)
	c.closed.Locker.Unlock()
	c.closed.Store(true)
}

// Atomic protect a val Type T with a mutex
type Atomic[T any] struct {
	Locker sync.Mutex
	val    T
}

func (v *Atomic[T]) Load() T {
	v.Locker.Lock()
	defer v.Locker.Unlock()

	return v.val
}

func (v *Atomic[T]) Store(val T) {
	v.Locker.Lock()
	defer v.Locker.Unlock()

	v.val = val
}

// isCtxDone return true if ctx has done
func isCtxDone(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return true, nil
		}
		return true, ctx.Err()
	default:
		return false, nil
	}
}
