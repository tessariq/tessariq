package container

// Config holds everything needed to create and run a Docker container.
type Config struct {
	Name        string            // deterministic container name: tessariq-<run_id>
	Image       string            // resolved container image
	Command     []string          // e.g. ["claude", "--print", ...]
	WorkDir     string            // container working directory, always "/work"
	User        string            // always "tessariq"
	Env         map[string]string // env vars injected via docker create --env
	Mounts      []Mount           // all bind mounts
	Interactive bool              // when true, docker create uses -i -t flags for TTY
	NetworkName string            // Docker network to attach; empty = default bridge

	// WritableDirs are container paths that must be writable by the
	// container user. Docker creates intermediate directories as root for
	// file-level bind mounts, so directories like ~/.claude/ end up
	// root-owned. When non-empty, the container command is wrapped with
	// mkdir -p to create these directories before exec'ing the agent.
	WritableDirs []string
}
