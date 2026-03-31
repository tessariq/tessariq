package container

import (
	"github.com/tessariq/tessariq/internal/authmount"
)

// Mount represents a single Docker bind mount.
type Mount struct {
	Source   string // host path
	Target   string // container path
	ReadOnly bool
}

// DockerFlag returns the -v flag value for this mount.
func (m Mount) DockerFlag() string {
	flag := m.Source + ":" + m.Target
	if m.ReadOnly {
		flag += ":ro"
	}
	return flag
}

// AssembleMounts builds the complete mount list for a container run.
// The worktree is mounted read-write at /work so the agent can modify code.
// Evidence is mounted read-only at /evidence because only the host-side runner
// writes evidence artifacts; the agent has no need to write there.
// Auth/config mounts are appended with their existing ReadOnly settings.
func AssembleMounts(worktreePath, evidencePath string, authMounts, configMounts []authmount.MountSpec) []Mount {
	mounts := []Mount{
		{Source: worktreePath, Target: "/work", ReadOnly: false},
		{Source: evidencePath, Target: "/evidence", ReadOnly: true},
	}
	for _, am := range authMounts {
		mounts = append(mounts, Mount{
			Source:   am.HostPath,
			Target:   am.ContainerPath,
			ReadOnly: am.ReadOnly,
		})
	}
	for _, cm := range configMounts {
		mounts = append(mounts, Mount{
			Source:   cm.HostPath,
			Target:   cm.ContainerPath,
			ReadOnly: cm.ReadOnly,
		})
	}
	return mounts
}
