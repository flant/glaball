package limiter

import (
	"sync"

	"github.com/flant/glaball/pkg/client"
)

const (
	// DefaultLimit is the default concurrency limit
	DefaultLimit = 100
)

type Error struct {
	Host *client.Host
	Err  error
}

type Limiter struct {
	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []Error

	sem chan struct{}
}

func NewLimiter(limit int) *Limiter {
	w := Limiter{sem: make(chan struct{}, limit)}
	return &w
}

func (l *Limiter) Error(host *client.Host, err error) {
	l.mu.Lock()
	l.errs = append(l.errs, Error{host, err})
	l.mu.Unlock()
}

func (l *Limiter) Errors() []Error {
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
