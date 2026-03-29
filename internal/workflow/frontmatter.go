package workflow

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func parseFrontmatter[T any](data []byte) (T, string, error) {
	var zero T
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return zero, "", fmt.Errorf("missing frontmatter prefix")
	}

	rest := data[len("---\n"):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return zero, "", fmt.Errorf("missing frontmatter terminator")
	}

	frontmatter := rest[:idx]
	body := rest[idx+len("\n---\n"):]

	var value T
	if err := yaml.Unmarshal(frontmatter, &value); err != nil {
		return zero, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	return value, strings.TrimLeft(string(body), "\n"), nil
}

func marshalFrontmatter[T any](value T, body string) ([]byte, error) {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var out strings.Builder
	out.WriteString("---\n")
	out.Write(encoded)
	out.WriteString("---\n\n")
	out.WriteString(strings.TrimSpace(body))
	out.WriteString("\n")

	return []byte(out.String()), nil
}
