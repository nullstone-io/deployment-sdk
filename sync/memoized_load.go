package sync

import "sync"

type MemoizedLoad[T any] struct {
	once sync.Once
	val  T
	err  error
}

// Load executes the given function exactly once and stores its result or error.
// Subsequent calls return the stored result or error without re-executing the function.
func (o *MemoizedLoad[T]) Load(fn func() (T, error)) (T, error) {
	var zero T // Default zero value for type T
	o.once.Do(func() {
		o.val, o.err = fn()
	})
	if o.err != nil {
		return zero, o.err
	}
	return o.val, nil
}

func (o *MemoizedLoad[T]) Value() T {
	return o.val
}

func (o *MemoizedLoad[T]) Error() error {
	return o.err
}
