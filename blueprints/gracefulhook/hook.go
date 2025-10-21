package gracefulhook

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// New creates a new GracefulHook, needs a shutdown signal channel
func New(ch <-chan os.Signal) *GracefulHook {
	return &GracefulHook{sigch: ch, errch: make(chan error)}
}

// GracefulHook is a struct for registering shutdown functions and gracefully executing
// them during shutdown, caused by signal or error
type GracefulHook struct {
	wg sync.WaitGroup

	sigch       <-chan os.Signal
	errch       chan error
	errOnce     sync.Once
	shutdownFns []*gracefulFn
	hookerr     error
}

// RegShutdown registers a shutdown function to be executed during graceful shutdown without timeout
func (h *GracefulHook) RegShutdown(shutdownFn func()) {
	h.regShutdownTime(shutdownFn, 0)
}

// RegShutdownTime registers a shutdown function to be executed during graceful shutdown with timeout
func (h *GracefulHook) RegShutdownTime(shutdownFn func(), timeout time.Duration) {
	h.regShutdownTime(shutdownFn, timeout)
}

// RegShutdownTimeWithStartup registers a shutdown function to be executed during graceful shutdown with timeout
// and starts a function that can fail and cause shutdown
func (h *GracefulHook) RegShutdownTimeWithStartup(startFn func() error, shutdownFn func(), timeout time.Duration) {
	// prevents any new component start, due to shutdown has been already triggered
	if h.hookerr != nil {
		return
	}
	h.regShutdownTime(shutdownFn, timeout)
	go func(errch chan<- error) {
		if err := startFn(); err != nil {
			h.errOnce.Do(func() {
				h.hookerr = err
				errch <- h.hookerr
			})
			return
		}
	}(h.errch)
}

// regShutdownTime registers a shutdown function to be executed during graceful shutdown with timeout
func (h *GracefulHook) regShutdownTime(shutdownFn func(), timeout time.Duration) {
	h.shutdownFns = append(h.shutdownFns, &gracefulFn{shutdownFn, timeout})
	h.wg.Add(1)
}

// cancellAll runs all registered shutdown functions
func (h *GracefulHook) cancellAll() {
	for i := len(h.shutdownFns) - 1; i >= 0; i-- {
		if err := h.shutdownFns[i].shutDown(); err != nil {
			slog.Error(err.Error())
		}
		h.wg.Done()
	}
}

// Wait waits for a signal or error and runs all registered shutdown functions
func (h *GracefulHook) Wait() {
	select {
	case <-h.sigch:
		slog.Info("SIGINT shutdown")
	case err := <-h.errch:
		slog.Error("Graceful startup error: " + err.Error())
	}
	h.cancellAll()
	h.wg.Wait()
}
