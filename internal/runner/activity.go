package runner

import (
	"io"
	"sync"
	"time"
)

// ActivityTimer tracks accumulated active time and fires when the total
// active duration exceeds the configured timeout. It pauses counting
// when no output activity is detected for longer than the idle threshold.
type ActivityTimer struct {
	timeout       time.Duration
	idleThreshold time.Duration
	tickInterval  time.Duration

	mu             sync.Mutex
	elapsed        time.Duration
	lastActive     time.Time
	lastCheckpoint time.Time
	fired          bool
	expired        chan struct{}
	stopped        chan struct{}
	clock          func() time.Time
}

// ActivityTimerOption configures an ActivityTimer.
type ActivityTimerOption func(*ActivityTimer)

// WithIdleThreshold sets the duration after which the timer considers the
// agent idle if no output has been received.
func WithIdleThreshold(d time.Duration) ActivityTimerOption {
	return func(t *ActivityTimer) { t.idleThreshold = d }
}

// WithTickInterval sets how often the timer checks for activity.
func WithTickInterval(d time.Duration) ActivityTimerOption {
	return func(t *ActivityTimer) { t.tickInterval = d }
}

// WithClock injects a clock function for testing.
func WithClock(clock func() time.Time) ActivityTimerOption {
	return func(t *ActivityTimer) { t.clock = clock }
}

// NewActivityTimer creates an ActivityTimer that fires after the given timeout
// of accumulated active time. Idle periods (no output for longer than the
// idle threshold) do not count toward the timeout.
func NewActivityTimer(timeout time.Duration, opts ...ActivityTimerOption) *ActivityTimer {
	t := &ActivityTimer{
		timeout:       timeout,
		idleThreshold: 30 * time.Second,
		tickInterval:  500 * time.Millisecond,
		expired:       make(chan struct{}),
		stopped:       make(chan struct{}),
		clock:         time.Now,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Start begins the timer loop. Call Stop() to clean up.
func (t *ActivityTimer) Start() {
	t.mu.Lock()
	now := t.clock()
	t.lastActive = now
	t.lastCheckpoint = now
	t.mu.Unlock()
	go t.loop()
}

// Stop terminates the timer loop without firing expiry.
func (t *ActivityTimer) Stop() {
	t.mu.Lock()
	t.fired = true
	t.mu.Unlock()
	select {
	case <-t.stopped:
	default:
		close(t.stopped)
	}
}

// Expired returns a channel that is closed when accumulated active time
// exceeds the timeout.
func (t *ActivityTimer) Expired() <-chan struct{} {
	return t.expired
}

// RecordActivity notifies the timer that the agent produced output.
// It also advances the elapsed counter and fires expiry if the
// accumulated active time exceeds the timeout.
func (t *ActivityTimer) RecordActivity() {
	now := t.clock()
	t.mu.Lock()
	t.lastActive = now
	expired := t.advanceElapsedLocked(now)
	if expired {
		t.fired = true
		close(t.expired)
	}
	t.mu.Unlock()
}

// Elapsed returns the current accumulated active time.
func (t *ActivityTimer) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.elapsed
}

// advanceElapsedLocked updates elapsed time if the timer is active.
// Returns true if the timer has expired. Caller must hold t.mu.
func (t *ActivityTimer) advanceElapsedLocked(now time.Time) bool {
	if t.fired {
		return false
	}
	sinceLastActive := now.Sub(t.lastActive)
	if sinceLastActive <= t.idleThreshold {
		t.elapsed += now.Sub(t.lastCheckpoint)
	}
	t.lastCheckpoint = now
	return t.elapsed >= t.timeout
}

func (t *ActivityTimer) loop() {
	ticker := time.NewTicker(t.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopped:
			return
		case <-ticker.C:
			now := t.clock()
			t.mu.Lock()
			if t.fired {
				t.mu.Unlock()
				return
			}
			expired := t.advanceElapsedLocked(now)
			if expired {
				t.fired = true
				close(t.expired)
				t.mu.Unlock()
				return
			}
			t.mu.Unlock()
		}
	}
}

// ActivityWriter wraps an io.Writer and notifies an ActivityTimer on every
// successful write. This tracks container output activity for timeout purposes.
type ActivityWriter struct {
	inner io.Writer
	timer *ActivityTimer
}

// NewActivityWriter creates a writer that delegates to inner and calls
// timer.RecordActivity() on each successful write with n > 0.
func NewActivityWriter(inner io.Writer, timer *ActivityTimer) *ActivityWriter {
	return &ActivityWriter{inner: inner, timer: timer}
}

func (w *ActivityWriter) Write(p []byte) (int, error) {
	n, err := w.inner.Write(p)
	if n > 0 {
		w.timer.RecordActivity()
	}
	return n, err
}
