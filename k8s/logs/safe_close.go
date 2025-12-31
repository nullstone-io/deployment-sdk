package logs

import "sync"

func SafeClose[T any](ch chan T) func() {
	if ch == nil {
		return func() {}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			close(ch)
		})
	}
}
