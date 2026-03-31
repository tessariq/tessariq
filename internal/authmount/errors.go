package authmount

import "fmt"

// AuthMissingError indicates that required auth files for the agent were not found.
type AuthMissingError struct {
	Agent string
}

func (e *AuthMissingError) Error() string {
	return fmt.Sprintf("supported auth files or directories for %s were not found; authenticate %s locally first", e.Agent, e.Agent)
}

// KeychainOnlyError indicates that macOS Claude Code has only Keychain-backed
// auth with no file-backed credential mirror.
type KeychainOnlyError struct{}

func (e *KeychainOnlyError) Error() string {
	return "v0.1.0 supports Claude Code auth reuse on macOS only when ~/.claude/.credentials.json exists; use a compatible file-backed setup"
}

// WritableAuthRequiredError indicates that the agent needs writable auth
// refresh, which is incompatible with the v0.1.0 read-only mount contract.
type WritableAuthRequiredError struct {
	Agent string
}

func (e *WritableAuthRequiredError) Error() string {
	return "v0.1.0 supports only read-only auth and config mounts; use a compatible pre-authenticated setup"
}
