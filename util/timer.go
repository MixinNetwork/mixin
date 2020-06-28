package util

import "time"

type Timer struct {
	impl *time.Timer
}

func NewTimer(d time.Duration) *Timer {
	impl := time.NewTimer(d)
	return &Timer{impl}
}

func (t *Timer) C() <-chan time.Time {
	return t.impl.C
}

func (t *Timer) Stop() {
	t.impl.Stop()
}

func (t *Timer) Reset(d time.Duration) {
	if !t.impl.Stop() {
		<-t.impl.C
	}
	t.impl.Reset(d)
}
