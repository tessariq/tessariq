package container

import (
	"os"
	"sort"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCreateArgs_FullCommand(t *testing.T) {
	t.Parallel()
	p := New(Config{
		Name:    "tessariq-abc123",
		Image:   "ghcr.io/tessariq/claude-code:latest",
		Command: []string{"claude", "--print", "do stuff"},
		WorkDir: "/work",
		User:    "tessariq",
		Mounts: []Mount{
			{Source: "/host/wt", Target: "/work", ReadOnly: false},
			{Source: "/host/ev", Target: "/evidence", ReadOnly: false},
			{Source: "/host/cred", Target: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
		},
		Env: map[string]string{"CLAUDE_CONFIG_DIR": "/home/tessariq/.claude"},
	})

	args := p.buildCreateArgs()

	require.Equal(t, "create", args[0])
	require.Contains(t, args, "--name")
	require.Contains(t, args, "tessariq-abc123")
	require.Contains(t, args, "--user")
	require.Contains(t, args, "tessariq")
	require.Contains(t, args, "--workdir")
	require.Contains(t, args, "/work")
	require.Contains(t, args, "ghcr.io/tessariq/claude-code:latest")
	require.Contains(t, args, "claude")
	require.Contains(t, args, "--print")
	require.Contains(t, args, "do stuff")
}

func TestBuildCreateArgs_Name(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "tessariq-run42", Image: "img"})
	args := p.buildCreateArgs()

	nameIdx := indexOf(args, "--name")
	require.GreaterOrEqual(t, nameIdx, 0)
	require.Equal(t, "tessariq-run42", args[nameIdx+1])
}

func TestBuildCreateArgs_User(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img", User: "tessariq"})
	args := p.buildCreateArgs()

	userIdx := indexOf(args, "--user")
	require.GreaterOrEqual(t, userIdx, 0)
	require.Equal(t, "tessariq", args[userIdx+1])
}

func TestBuildCreateArgs_NoUser(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	require.Equal(t, -1, indexOf(args, "--user"))
}

func TestBuildCreateArgs_WorkDir(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img", WorkDir: "/work"})
	args := p.buildCreateArgs()

	wdIdx := indexOf(args, "--workdir")
	require.GreaterOrEqual(t, wdIdx, 0)
	require.Equal(t, "/work", args[wdIdx+1])
}

func TestBuildCreateArgs_NoWorkDir(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	require.Equal(t, -1, indexOf(args, "--workdir"))
}

func TestBuildCreateArgs_MountFlags(t *testing.T) {
	t.Parallel()
	p := New(Config{
		Name:  "c",
		Image: "img",
		Mounts: []Mount{
			{Source: "/a", Target: "/b", ReadOnly: false},
			{Source: "/c", Target: "/d", ReadOnly: true},
		},
	})
	args := p.buildCreateArgs()

	vFlags := collectAfter(args, "-v")
	require.Len(t, vFlags, 2)
	require.Contains(t, vFlags, "/a:/b")
	require.Contains(t, vFlags, "/c:/d:ro")
}

func TestBuildCreateArgs_EnvFlags(t *testing.T) {
	t.Parallel()
	p := New(Config{
		Name:  "c",
		Image: "img",
		Env:   map[string]string{"KEY1": "val1", "KEY2": "val2"},
	})
	args := p.buildCreateArgs()

	envFlags := collectAfter(args, "--env")
	sort.Strings(envFlags)
	require.Len(t, envFlags, 2)
	require.Contains(t, envFlags, "KEY1=val1")
	require.Contains(t, envFlags, "KEY2=val2")
}

func TestBuildCreateArgs_NoEnvFlags(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	require.Equal(t, -1, indexOf(args, "--env"))
}

func TestBuildCreateArgs_CommandAtEnd(t *testing.T) {
	t.Parallel()
	p := New(Config{
		Name:    "c",
		Image:   "myimg",
		Command: []string{"claude", "--print", "task"},
	})
	args := p.buildCreateArgs()

	// Image followed by command must be at the end.
	imgIdx := indexOf(args, "myimg")
	require.GreaterOrEqual(t, imgIdx, 0)
	require.Equal(t, []string{"myimg", "claude", "--print", "task"}, args[imgIdx:])
}

func TestSignalCommand_SIGTERM(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test-container"})
	cmd := p.signalCommand(syscall.SIGTERM)
	require.Equal(t, []string{"docker", "stop", "--time=10", "test-container"}, cmd)
}

func TestSignalCommand_SIGKILL(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test-container"})
	cmd := p.signalCommand(syscall.SIGKILL)
	require.Equal(t, []string{"docker", "kill", "test-container"}, cmd)
}

func TestSignalCommand_SIGINT(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test-container"})
	cmd := p.signalCommand(syscall.SIGINT)
	require.Equal(t, []string{"docker", "stop", "--time=10", "test-container"}, cmd)
}

func TestSignalCommand_Other(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test-container"})
	cmd := p.signalCommand(syscall.SIGUSR1)
	require.Equal(t, []string{"docker", "kill", "--signal=user defined signal 1", "test-container"}, cmd)
}

func TestNew_SetsDockerBinary(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test"})
	require.Equal(t, "docker", p.docker)
}

func TestRemove_SkipsWhenNotCreated(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "test"})
	// remove should be a no-op when container was never created.
	require.False(t, p.created)
	err := p.remove(nil)
	require.NoError(t, err)
}

func TestBuildCreateArgs_NetworkName(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img", NetworkName: "tessariq-net-abc123"})
	args := p.buildCreateArgs()

	netIdx := indexOf(args, "--net")
	require.GreaterOrEqual(t, netIdx, 0, "--net must be present")
	require.Equal(t, "tessariq-net-abc123", args[netIdx+1])

	imgIdx := indexOf(args, "img")
	require.Less(t, netIdx, imgIdx, "--net must precede image")
}

func TestBuildCreateArgs_NoNetworkName(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	require.Equal(t, -1, indexOf(args, "--net"))
}

func TestBuildCreateArgs_InteractiveFlags(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img", Interactive: true})
	args := p.buildCreateArgs()

	require.Contains(t, args, "-i")
	require.Contains(t, args, "-t")

	// -i and -t must appear before the image name.
	imgIdx := indexOf(args, "img")
	iIdx := indexOf(args, "-i")
	tIdx := indexOf(args, "-t")
	require.Less(t, iIdx, imgIdx, "-i must precede image")
	require.Less(t, tIdx, imgIdx, "-t must precede image")
}

func TestBuildCreateArgs_NonInteractiveNoTTYFlags(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	require.Equal(t, -1, indexOf(args, "-i"))
	require.Equal(t, -1, indexOf(args, "-t"))
}

func TestBuildCreateArgs_CapDropAll(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	capIdx := indexOf(args, "--cap-drop")
	require.GreaterOrEqual(t, capIdx, 0, "--cap-drop must be present")
	require.Equal(t, "ALL", args[capIdx+1])

	imgIdx := indexOf(args, "img")
	require.Less(t, capIdx, imgIdx, "--cap-drop must precede image")
}

func TestBuildCreateArgs_NoNewPrivileges(t *testing.T) {
	t.Parallel()
	p := New(Config{Name: "c", Image: "img"})
	args := p.buildCreateArgs()

	secIdx := indexOf(args, "--security-opt")
	require.GreaterOrEqual(t, secIdx, 0, "--security-opt must be present")
	require.Equal(t, "no-new-privileges", args[secIdx+1])

	imgIdx := indexOf(args, "img")
	require.Less(t, secIdx, imgIdx, "--security-opt must precede image")
}

func TestPrepareWritableMounts_ChmodsRWMounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create a file with restrictive permissions.
	testFile := dir + "/test.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0o600))

	p := New(Config{
		Name: "test",
		Mounts: []Mount{
			{Source: dir, Target: "/work", ReadOnly: false},
		},
	})

	require.NoError(t, p.prepareWritableMounts())

	// Verify the file is now world-readable+writable.
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	perm := info.Mode().Perm()
	require.True(t, perm&0o006 == 0o006,
		"file should be world-readable+writable after prepare, got %o", perm)
}

func TestPrepareWritableMounts_SkipsReadOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testFile := dir + "/test.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0o600))

	p := New(Config{
		Name: "test",
		Mounts: []Mount{
			{Source: dir, Target: "/evidence", ReadOnly: true},
		},
	})

	require.NoError(t, p.prepareWritableMounts())

	// Permissions should be unchanged for read-only mounts.
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	perm := info.Mode().Perm()
	require.Equal(t, os.FileMode(0o600), perm,
		"read-only mount source should not be chmod'd")
}

// indexOf returns the first index of needle in args, or -1.
func indexOf(args []string, needle string) int {
	for i, a := range args {
		if a == needle {
			return i
		}
	}
	return -1
}

// collectAfter returns all values that follow the given flag in args.
// For example, collectAfter(["--env", "A", "--env", "B"], "--env") returns ["A", "B"].
func collectAfter(args []string, flag string) []string {
	var result []string
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			result = append(result, args[i+1])
		}
	}
	return result
}
