package runner

import (
	"os"
	"syscall"
)

// SignalState maps an OS signal to the corresponding terminal state.
func SignalState(sig os.Signal) State {
	if sig == syscall.SIGINT {
		return StateInterrupted
	}
	return StateKilled
}
