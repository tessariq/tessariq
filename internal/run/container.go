package run

// ContainerName returns the deterministic Docker container name for a run.
func ContainerName(runID string) string {
	return "tessariq-" + runID
}
