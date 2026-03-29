package run

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var entropy = &ulid.LockedMonotonicReader{
	MonotonicReader: ulid.Monotonic(rand.Reader, 0),
}

func NewRunID(now time.Time) (string, error) {
	id, err := ulid.New(ulid.Timestamp(now), entropy)
	if err != nil {
		return "", fmt.Errorf("generate run ID: %w", err)
	}
	return id.String(), nil
}

func IsValidRunID(id string) bool {
	if len(id) != 26 {
		return false
	}

	upper := strings.ToUpper(id)
	for _, c := range upper {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			return false
		}
	}

	_, err := ulid.Parse(id)
	return err == nil
}
