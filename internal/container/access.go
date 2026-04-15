package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// HardenWritablePath restricts a writable host path to the invoking user and,
// on Linux when needed, grants the container user exact access via POSIX ACLs.
// This avoids relying on host group membership, which can accidentally grant
// unrelated local users access when numeric gids overlap.
func HardenWritablePath(ctx context.Context, path string, identity RuntimeIdentity) error {
	for _, args := range buildHardenPathCommands(runtime.GOOS, path, os.Getuid(), identity) {
		if len(args) == 0 {
			continue
		}
		if args[0] == "setfacl" {
			if _, err := exec.LookPath("setfacl"); err != nil {
				return fmt.Errorf("setfacl is required to harden writable path %s for runtime uid %d: %w", path, identity.UID, err)
			}
		}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s %s: %s: %w", args[0], path, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func buildHardenPathCommands(goos, path string, hostUID int, identity RuntimeIdentity) [][]string {
	cmds := [][]string{{"chmod", "-R", "u=rwX,go=", path}}
	if goos != "linux" || identity.UID == hostUID {
		return cmds
	}
	cmds = append(cmds,
		[]string{"setfacl", "-R", "-m", fmt.Sprintf("u:%d:rwX", identity.UID), path},
		[]string{"find", path, "-type", "d", "-exec", "setfacl", "-m", fmt.Sprintf("d:u:%d:rwX,d:u:%d:rwX", hostUID, identity.UID), "{}", "+"},
	)
	return cmds
}
