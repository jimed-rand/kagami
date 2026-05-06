package workspace

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// CleanupHandler runs registered cleanup callbacks in LIFO order on SIGINT or
// SIGTERM. It guarantees each callback runs at most once even when both a
// signal and the regular exit path race.
type CleanupHandler struct {
	mu       sync.Mutex
	cbs      []func()
	done     bool
	signalCh chan os.Signal
}

// NewCleanupHandler installs SIGINT/SIGTERM handlers tied to ctx. Cancellation
// of ctx (typically by the signal handler) lets the caller short-circuit
// in-flight work.
func NewCleanupHandler(ctx context.Context, cancel context.CancelFunc) *CleanupHandler {
	h := &CleanupHandler{
		signalCh: make(chan os.Signal, 1),
	}
	signal.Notify(h.signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-h.signalCh:
			h.RunAll()
			cancel()
		case <-ctx.Done():
			return
		}
	}()
	return h
}

// Add registers cb to run on abort or via RunAll.
func (h *CleanupHandler) Add(cb func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cbs = append(h.cbs, cb)
}

// RunAll executes registered callbacks in LIFO order. Subsequent calls are
// no-ops.
func (h *CleanupHandler) RunAll() {
	h.mu.Lock()
	if h.done {
		h.mu.Unlock()
		return
	}
	h.done = true
	cbs := h.cbs
	h.cbs = nil
	h.mu.Unlock()

	for i := len(cbs) - 1; i >= 0; i-- {
		safeRun(cbs[i])
	}
}

// Stop unregisters the OS signal handler. Cleanup callbacks are NOT executed.
func (h *CleanupHandler) Stop() {
	signal.Stop(h.signalCh)
}

func safeRun(fn func()) {
	defer func() { _ = recover() }()
	fn()
}
