package run

// SessionName returns the deterministic tmux session name for a run.
// It matches ContainerName so the session and container share a name.
func SessionName(runID string) string {
	return ContainerName(runID)
}
