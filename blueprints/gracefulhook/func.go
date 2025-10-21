package gracefulhook

import (
	"errors"
	"time"
)

// gracefulFn is a struct for registering shutdown functions with timeout and gracefully executing them during shutdown
type gracefulFn struct {
	f       func()
	timeout time.Duration
}

// shutDown runs f with defined timer or without it
func (f *gracefulFn) shutDown() error {
	if f.timeout == 0 {
		f.f()
		return nil
	}
	return f.timedShutDown()
}

// timedShutDown runs f with defined timer
func (f *gracefulFn) timedShutDown() error {
	finished := make(chan struct{})
	go func() {
		f.f()
		close(finished)
	}()

	t := time.NewTimer(f.timeout)
	defer t.Stop()
	select {
	case <-t.C:
		return errors.New("Graceful shutdown failed within: " + f.timeout.String())
	case <-finished:
		return nil
	}
}
