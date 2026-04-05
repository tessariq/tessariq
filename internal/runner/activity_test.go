package runner

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActivityTimer_ExpiresAfterContinuousActivity(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	timer := NewActivityTimer(1*time.Second,
		WithIdleThreshold(500*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)
	timer.Start()
	defer timer.Stop()

	// Continuously record activity while advancing time.
	for i := 0; i < 120; i++ {
		advance(10 * time.Millisecond)
		timer.RecordActivity()
	}

	select {
	case <-timer.Expired():
		// Expected: timer expired after ~1s of active time.
	case <-time.After(5 * time.Second):
		t.Fatal("timer should have expired")
	}
}

func TestActivityTimer_PausesDuringIdle(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	timer := NewActivityTimer(500*time.Millisecond,
		WithIdleThreshold(100*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)
	timer.Start()
	defer timer.Stop()

	// Active for 200ms.
	for i := 0; i < 20; i++ {
		advance(10 * time.Millisecond)
		timer.RecordActivity()
	}

	// Go idle for 2s (well past idle threshold).
	advance(2 * time.Second)

	// Timer should NOT have expired (only ~200ms of active time, timeout=500ms).
	select {
	case <-timer.Expired():
		t.Fatal("timer should not have expired during idle period")
	default:
	}

	elapsed := timer.Elapsed()
	require.Less(t, elapsed, 400*time.Millisecond,
		"elapsed should reflect only active time (~200ms), got %s", elapsed)

	// Resume activity for another 400ms to push past timeout.
	for i := 0; i < 40; i++ {
		advance(10 * time.Millisecond)
		timer.RecordActivity()
	}

	select {
	case <-timer.Expired():
		// Expected.
	case <-time.After(5 * time.Second):
		t.Fatal("timer should have expired after resumed activity")
	}
}

func TestActivityTimer_StopPreventsExpiry(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	timer := NewActivityTimer(100*time.Millisecond,
		WithIdleThreshold(50*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)
	timer.Start()

	// Stop before timeout.
	timer.Stop()

	// Advance time well past timeout.
	mu.Lock()
	now = now.Add(10 * time.Second)
	mu.Unlock()
	time.Sleep(30 * time.Millisecond)

	select {
	case <-timer.Expired():
		t.Fatal("expired channel should not be closed after Stop()")
	default:
		// Expected: not expired.
	}
}

func TestActivityTimer_ElapsedAccuracy(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	timer := NewActivityTimer(10*time.Second,
		WithIdleThreshold(100*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)
	timer.Start()
	defer timer.Stop()

	// Active for 300ms.
	for i := 0; i < 30; i++ {
		advance(10 * time.Millisecond)
		timer.RecordActivity()
	}

	// Go idle for 1s.
	advance(1 * time.Second)

	elapsed := timer.Elapsed()
	// Should be approximately 300ms (active time only), not 1.3s.
	require.Greater(t, elapsed, 200*time.Millisecond)
	require.Less(t, elapsed, 500*time.Millisecond)
}

func TestActivityWriter_DelegatesAndRecords(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	timer := NewActivityTimer(10*time.Second,
		WithClock(clock),
	)
	var buf bytes.Buffer
	aw := NewActivityWriter(&buf, timer)

	n, err := aw.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", buf.String())
}

func TestActivityTimer_RecordActivityBeforeStartDoesNotOverflow(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	timer := NewActivityTimer(1*time.Second,
		WithIdleThreshold(500*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)

	// RecordActivity before Start must not fire the timer.
	timer.RecordActivity()

	select {
	case <-timer.Expired():
		t.Fatal("timer should not have expired from pre-Start RecordActivity")
	default:
	}

	require.Less(t, timer.Elapsed(), 100*time.Millisecond,
		"elapsed should be near zero, not overflowed")
}

func TestActivityWriter_NoActivityOnZeroBytes(t *testing.T) {
	t.Parallel()

	mu := sync.Mutex{}
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	timer := NewActivityTimer(10*time.Second,
		WithIdleThreshold(50*time.Millisecond),
		WithTickInterval(10*time.Millisecond),
		WithClock(clock),
	)

	before := timer.lastActive

	var buf bytes.Buffer
	aw := NewActivityWriter(&buf, timer)

	n, err := aw.Write([]byte{})
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// lastActive should not have changed.
	require.Equal(t, before, timer.lastActive)
}
