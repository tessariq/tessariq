package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteTimeoutFlag creates the timeout.flag marker in the evidence directory.
func WriteTimeoutFlag(evidenceDir string) error {
	path := filepath.Join(evidenceDir, "timeout.flag")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return fmt.Errorf("write timeout flag: %w", err)
	}
	return nil
}
