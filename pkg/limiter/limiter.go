package limiter

import (
	"sync"
)

const (
	// DefaultLimit is the default concurrency limit
	DefaultLimit = 100
)

type Limiter struct {
	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []error

	sem chan struct{}
}

func NewLimiter(limit int) *Limiter {
	w := Limiter{sem: make(chan struct{}, limit)}
	return &w
}

func (l *Limiter) Error(err error) {
	l.mu.Lock()
	l.errs = append(l.errs, err)
	l.mu.Unlock()
}

func (l *Limiter) Errors() []error {
	return l.errs
}

func (l *Limiter) Add(delta int) {
	l.wg.Add(delta)
}

func (l *Limiter) Done() {
	l.wg.Done()
}

func (l *Limiter) Lock() {
	l.sem <- struct{}{}
}

func (l *Limiter) Unlock() {
	<-l.sem
}

func (l *Limiter) Wait() {
	l.wg.Wait()
}
