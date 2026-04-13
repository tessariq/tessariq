package runner

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// SignalError preserves the originating OS signal in a cancellable context.
type SignalError struct {
	Signal os.Signal
}

func (e SignalError) Error() string {
	if e.Signal == nil {
		return "received signal"
	}
	return fmt.Sprintf("received signal %s", e.Signal)
}

// SignalCause wraps an OS signal in an error so it can be used with
// context.WithCancelCause and later mapped back to a terminal run state.
func SignalCause(sig os.Signal) error {
	return SignalError{Signal: sig}
}

// SignalState maps an OS signal to the corresponding terminal state.
func SignalState(sig os.Signal) State {
	if sig == syscall.SIGINT {
		return StateInterrupted
	}
	return StateKilled
}

// SignalStateFromCause maps a cancellation cause back to a terminal state.
// Unknown causes conservatively become killed.
func SignalStateFromCause(err error) State {
	var sigErr SignalError
	if errors.As(err, &sigErr) {
		return SignalState(sigErr.Signal)
	}
	return StateKilled
}
