package workspace

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// RepoID derives a stable, filesystem-safe identifier from the repository root
// path. It combines a slugified basename with the first 8 hex characters of the
// SHA-256 hash of the full path.
func RepoID(repoRoot string) string {
	base := filepath.Base(repoRoot)
	return slug(base) + "-" + shortHash(repoRoot)
}

func slug(s string) string {
	var b strings.Builder
	prev := true // treat start as a hyphen to suppress leading hyphens
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = false
		} else if !prev {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:4])
}
