package util

import "time"

type Timer struct {
	impl    *time.Timer
	drained bool
}

func NewTimer(d time.Duration) *Timer {
	impl := time.NewTimer(d)
	return &Timer{impl, false}
}

func (t *Timer) C() <-chan time.Time {
	return t.impl.C
}

func (t *Timer) Drain() {
	t.drained = true
}

func (t *Timer) Stop() {
	t.impl.Stop()
}

func (t *Timer) Reset(d time.Duration) {
	if !t.drained && !t.impl.Stop() {
		<-t.impl.C
	}
	t.impl.Reset(d)
	t.drained = false
}
