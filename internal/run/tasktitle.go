package run

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
)

func ExtractTaskTitle(content []byte, filename string) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}

		title := trimmed[2:]
		title = strings.TrimRight(title, " \t#")
		title = strings.TrimSpace(title)

		if title == "" {
			continue
		}

		return title
	}

	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}
