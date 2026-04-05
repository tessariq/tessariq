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
	// root-owned. Each entry becomes a --tmpfs mount so the directory is
	// writable regardless of Docker's intermediate directory ownership.
	// File-level bind mounts inside these directories still work (Docker
	// supports nested mounts).
	WritableDirs []string
}
