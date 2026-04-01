package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type requiredSpecCoverage struct {
	Ref         string
	Title       string
	SpecVersion string
}

type specScope struct {
	Milestone string
	Version   string
	Path      string
}

type specDocument struct {
	Scope   specScope
	Anchors map[string]string
}

// specRefAliases maps historical spec-ref anchors to their normative replacements.
// Completed tasks that reference the alias are treated as covering the normative anchor.
var specRefAliases = map[string]map[string]string{
	"v0.1.0": {
		"adapter-contract": "agent-and-runtime-contract",
	},
}

var requiredSpecCoverageByVersion = map[string][]requiredSpecCoverage{
	"v0.1.0": {
		{Ref: "specs/tessariq-v0.1.0.md#release-intent", Title: "release intent", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#product-intent", Title: "product intent", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#repository-model", Title: "repository model", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#workspace-guarantees", Title: "workspace guarantees", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#host-prerequisites", Title: "host prerequisites", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#tessariq-init", Title: "tessariq init", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#tessariq-version", Title: "tessariq version", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#tessariq-run-task-path", Title: "tessariq run", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#tessariq-attach-run-ref", Title: "tessariq attach", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#tessariq-promote-run-ref", Title: "tessariq promote", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#lifecycle-rules", Title: "lifecycle rules", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#agent-and-runtime-contract", Title: "agent and runtime contract", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#networking-and-egress", Title: "networking and egress", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#evidence-contract", Title: "evidence contract", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#compatibility-rules", Title: "compatibility rules", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#acceptance-scenarios", Title: "acceptance scenarios", SpecVersion: "v0.1.0"},
		{Ref: "specs/tessariq-v0.1.0.md#failure-ux", Title: "failure UX", SpecVersion: "v0.1.0"},
	},
}

func resolveSpecScope(state *State) (specScope, []string) {
	scope := specScope{
		Milestone: strings.TrimSpace(state.Frontmatter.MilestoneFocus),
		Version:   strings.TrimSpace(state.Frontmatter.ActiveSpecVersion),
		Path:      filepath.Clean(strings.TrimSpace(state.Frontmatter.ActiveSpecPath)),
	}

	var violations []string
	if scope.Milestone == "" {
		violations = append(violations, "state milestone_focus must not be empty")
	}
	if scope.Version == "" {
		violations = append(violations, "state active_spec_version must not be empty")
	}
	if scope.Path == "." || scope.Path == "" {
		violations = append(violations, "state active_spec_path must not be empty")
	}
	if scope.Version != "" && scope.Path != "" && !strings.Contains(scope.Path, scope.Version) {
		violations = append(violations, fmt.Sprintf("state active_spec_path %q does not match active_spec_version %q", scope.Path, scope.Version))
	}

	return scope, violations
}

func loadSpecDocument(repoRoot string, scope specScope) (*specDocument, error) {
	return loadSpecDocumentAtPath(repoRoot, scope.Path)
}

func loadSpecDocumentAtPath(repoRoot, specPath string) (*specDocument, error) {
	filename := filepath.Join(repoRoot, specPath)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", specPath, err)
	}

	return &specDocument{
		Scope: specScope{
			Version: specVersionFromPath(specPath),
			Path:    specPath,
		},
		Anchors: extractHeadingAnchors(string(data)),
	}, nil
}

func extractHeadingAnchors(markdown string) map[string]string {
	anchors := make(map[string]string)
	duplicateCounts := make(map[string]int)

	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}

		title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if title == "" {
			continue
		}

		base := markdownAnchor(title)
		if base == "" {
			continue
		}

		anchor := base
		if count := duplicateCounts[base]; count > 0 {
			anchor = fmt.Sprintf("%s-%d", base, count)
		}
		duplicateCounts[base]++
		anchors[anchor] = title
	}

	return anchors
}

func markdownAnchor(title string) string {
	var out strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
			prevHyphen = false
		case r >= '0' && r <= '9':
			out.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-':
			if out.Len() == 0 || prevHyphen {
				continue
			}
			out.WriteByte('-')
			prevHyphen = true
		}
	}

	return strings.Trim(out.String(), "-")
}

func splitSpecRef(ref string) (string, string, error) {
	path, anchor, ok := strings.Cut(ref, "#")
	if !ok {
		return "", "", fmt.Errorf("spec ref %q is missing an anchor", ref)
	}
	path = filepath.Clean(strings.TrimSpace(path))
	anchor = strings.TrimSpace(anchor)
	if path == "." || path == "" {
		return "", "", fmt.Errorf("spec ref %q is missing a path", ref)
	}
	if anchor == "" {
		return "", "", fmt.Errorf("spec ref %q is missing an anchor", ref)
	}
	return path, anchor, nil
}

func specVersionFromPath(specPath string) string {
	base := filepath.Base(specPath)
	if strings.HasPrefix(base, "tessariq-") && strings.HasSuffix(base, ".md") {
		return strings.TrimSuffix(strings.TrimPrefix(base, "tessariq-"), ".md")
	}
	return ""
}

func resolveSpecRefAlias(ref string, specVersion string) string {
	aliases, ok := specRefAliases[specVersion]
	if !ok {
		return ref
	}
	path, anchor, err := splitSpecRef(ref)
	if err != nil {
		return ref
	}
	if replacement, ok := aliases[anchor]; ok {
		return path + "#" + replacement
	}
	return ref
}
