package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/adapter/opencode"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

func TestPrintRunOutput_ContainsAllFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EvidencePath:  "/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV",
		WorkspacePath: "/home/user/.tessariq/worktrees/abc/01ARZ3NDEKTSV4RRFFQ69G5FAV",
		ContainerName: "tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
	})

	output := buf.String()
	require.Contains(t, output, "run_id: 01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "evidence_path: /repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "workspace_path: /home/user/.tessariq/worktrees/abc/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "container_name: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "attach: tessariq attach 01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "promote: tessariq promote 01ARZ3NDEKTSV4RRFFQ69G5FAV")
}

func TestPrintRunOutput_ScriptFriendlyFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "TESTID",
		EvidencePath:  "/evidence",
		WorkspacePath: "/workspace",
		ContainerName: "tessariq-TESTID",
	})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Equal(t, 6, len(lines), "expected exactly 6 output lines")
	for _, line := range lines {
		require.Contains(t, line, ": ", "each line must be key: value format")
	}
}

func TestPrintRunOutput_AttachCommandUsesRunID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "MYRUNID",
		EvidencePath:  "/e",
		WorkspacePath: "/w",
		ContainerName: "tessariq-MYRUNID",
	})

	output := buf.String()
	require.Contains(t, output, "tessariq attach MYRUNID")
	require.Contains(t, output, "tessariq promote MYRUNID")
}

func TestPrintInteractiveNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interactive bool
		attach      bool
		wantNote    bool
	}{
		{
			name:        "default_run_no_note",
			interactive: false,
			attach:      false,
			wantNote:    false,
		},
		{
			name:        "explicit_interactive_without_attach_emits_note",
			interactive: true,
			attach:      false,
			wantNote:    true,
		},
		{
			name:        "interactive_with_attach_no_note",
			interactive: true,
			attach:      true,
			wantNote:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			printInteractiveNote(&buf, tt.interactive, tt.attach, "test-session")
			if tt.wantNote {
				require.Contains(t, buf.String(), "note: interactive mode without --attach")
				require.Contains(t, buf.String(), "test-session")
			} else {
				require.Empty(t, buf.String())
			}
		})
	}
}

// fakeReadFile returns a readFile func that serves canned content keyed by
// suffix match on the path, and tracks which paths were actually read.
func fakeReadFile(files map[string]string) (func(string) ([]byte, error), *[]string) {
	var called []string
	fn := func(path string) ([]byte, error) {
		called = append(called, path)
		for suffix, content := range files {
			if strings.HasSuffix(path, suffix) {
				return []byte(content), nil
			}
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return fn, &called
}

func TestResolveAllowlistCore_OpenCode(t *testing.T) {
	t.Parallel()

	validAuth := `{"provider":"https://api.example.com/v1"}`
	noProviderAuth := `{"token":"fake"}`

	tests := []struct {
		name            string
		agent           string
		egress          string
		cliAllow        []string
		noDefaults      bool
		configDirExists bool // if true, dirExists returns true so user config is loaded
		files           map[string]string
		wantSource      string
		wantErr         string
		wantErrType     any
		wantProvSkipped bool // true if auth.json should NOT be read
	}{
		{
			name:            "cli_bypasses_unresolvable_provider",
			agent:           "opencode",
			egress:          "proxy",
			cliAllow:        []string{"custom.host:443"},
			files:           map[string]string{}, // no auth.json at all
			wantSource:      "cli",
			wantProvSkipped: true,
		},
		{
			name:        "no_cli_unresolvable_provider_errors",
			agent:       "opencode",
			egress:      "proxy",
			files:       map[string]string{"auth.json": noProviderAuth},
			wantErrType: &opencode.ProviderUnresolvableError{},
		},
		{
			name:            "cli_wins_even_when_provider_resolvable",
			agent:           "opencode",
			egress:          "proxy",
			cliAllow:        []string{"override.host:8443"},
			files:           map[string]string{"auth.json": validAuth},
			wantSource:      "cli",
			wantProvSkipped: true,
		},
		{
			name:       "claude_code_cli",
			agent:      "claude-code",
			egress:     "proxy",
			cliAllow:   []string{"my.api:443"},
			files:      map[string]string{},
			wantSource: "cli",
		},
		{
			name:            "opencode_non_proxy_skips_resolution",
			agent:           "opencode",
			egress:          "open",
			files:           map[string]string{},
			wantSource:      "built_in",
			wantProvSkipped: true,
		},
		{
			name:       "opencode_proxy_provider_resolvable_uses_built_in",
			agent:      "opencode",
			egress:     "proxy",
			files:      map[string]string{"auth.json": validAuth},
			wantSource: "built_in",
		},
		{
			name:       "no_defaults_no_cli_proxy_errors",
			agent:      "opencode",
			egress:     "proxy",
			noDefaults: true,
			files:      map[string]string{},
			wantErr:    "proxy mode requires at least one",
		},
		{
			name:            "user_config_allowlist_bypasses_unresolvable_provider",
			agent:           "opencode",
			egress:          "proxy",
			configDirExists: true,
			files: map[string]string{
				"config.yaml": "egress_allow:\n  - api.example.com:443\n",
			},
			wantSource:      "user_config",
			wantProvSkipped: true,
		},
		{
			name:    "no_user_config_no_cli_missing_auth_errors",
			agent:   "opencode",
			egress:  "proxy",
			files:   map[string]string{}, // no auth.json, no user config
			wantErr: "authenticate opencode locally first",
		},
		{
			name:            "cli_wins_over_user_config_and_skips_provider",
			agent:           "opencode",
			egress:          "proxy",
			cliAllow:        []string{"override.host:443"},
			configDirExists: true,
			files: map[string]string{
				"config.yaml": "egress_allow:\n  - api.example.com:443\n",
			},
			wantSource:      "cli",
			wantProvSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			readFile, called := fakeReadFile(tt.files)
			deps := resolveAllowlistDeps{
				xdgConfigHome: "",
				dirExists:     func(string) bool { return tt.configDirExists },
				readFile:      readFile,
			}

			cfg := run.Config{
				Agent:            tt.agent,
				EgressAllow:      tt.cliAllow,
				EgressNoDefaults: tt.noDefaults,
			}

			result, err := resolveAllowlistCore(cfg, "/fakehome", tt.egress, deps)

			if tt.wantErrType != nil {
				require.Error(t, err)
				require.ErrorAs(t, err, &tt.wantErrType)
				return
			}
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantSource, result.Source)

			if tt.wantSource == "cli" {
				// CLI destinations should match input.
				for _, entry := range tt.cliAllow {
					host, _, _ := run.ParseDestination(entry)
					found := false
					for _, d := range result.Destinations {
						if strings.HasPrefix(d, host) {
							found = true
							break
						}
					}
					require.True(t, found, "expected CLI destination %q in result", entry)
				}
			}

			if tt.wantProvSkipped {
				authPath := filepath.Join("/fakehome", ".local", "share", "opencode", "auth.json")
				for _, path := range *called {
					require.NotEqual(t, authPath, path, "provider resolution should have been skipped")
				}
			}
		})
	}
}

func TestResolveAllowlistCore_OpenCode_PermissionDenied(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "auth.json") {
			return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	deps := resolveAllowlistDeps{
		xdgConfigHome: "",
		dirExists:     func(string) bool { return false },
		readFile:      readFile,
	}

	cfg := run.Config{
		Agent: "opencode",
	}

	_, err := resolveAllowlistCore(cfg, "/fakehome", "proxy", deps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read auth file")

	var authMissing *authmount.AuthMissingError
	require.False(t, errors.As(err, &authMissing),
		"permission denied should not be mapped to AuthMissingError")
}

// writeManifestFixture writes a minimal valid manifest.json to dir.
func writeManifestFixture(t *testing.T, dir string) {
	t.Helper()
	m := run.Manifest{
		SchemaVersion: 1,
		RunID:         "TEST01",
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Sample",
		Agent:         "claude-code",
		BaseSHA:       "abc123",
		WorkspaceMode: "worktree",
		CreatedAt:     "2026-01-01T00:00:00Z",
	}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600))
}

// writeStatusFixture writes a minimal valid status.json to dir.
func writeStatusFixture(t *testing.T, dir string) {
	t.Helper()
	s := runner.Status{
		SchemaVersion: 1,
		State:         runner.StateSuccess,
		StartedAt:     "2026-01-01T00:00:00Z",
		FinishedAt:    "2026-01-01T00:01:00Z",
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "status.json"), data, 0o600))
}

func TestAppendIndexEntry_Warnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T, evidenceDir, repoRoot string)
		wantWarning string
	}{
		{
			name:        "manifest_read_failure",
			setup:       func(t *testing.T, evidenceDir, repoRoot string) {},
			wantWarning: "read manifest",
		},
		{
			name: "status_read_failure",
			setup: func(t *testing.T, evidenceDir, repoRoot string) {
				writeManifestFixture(t, evidenceDir)
			},
			wantWarning: "read status",
		},
		{
			name: "append_failure",
			setup: func(t *testing.T, evidenceDir, repoRoot string) {
				writeManifestFixture(t, evidenceDir)
				writeStatusFixture(t, evidenceDir)
				runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
				require.NoError(t, os.MkdirAll(runsDir, 0o700))
				require.NoError(t, os.Chmod(runsDir, 0o444))
				t.Cleanup(func() { os.Chmod(runsDir, 0o700) })
			},
			wantWarning: "open index file",
		},
		{
			name: "success_no_warning",
			setup: func(t *testing.T, evidenceDir, repoRoot string) {
				writeManifestFixture(t, evidenceDir)
				writeStatusFixture(t, evidenceDir)
				runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
				require.NoError(t, os.MkdirAll(runsDir, 0o700))
			},
			wantWarning: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evidenceDir := t.TempDir()
			repoRoot := t.TempDir()
			tt.setup(t, evidenceDir, repoRoot)

			var buf bytes.Buffer
			appendIndexEntry(&buf, repoRoot, evidenceDir)

			if tt.wantWarning != "" {
				require.Contains(t, buf.String(), "warning:")
				require.Contains(t, buf.String(), tt.wantWarning)
			} else {
				require.Empty(t, buf.String())
			}
		})
	}
}

func TestWarnDiffArtifacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantWarning bool
	}{
		{
			name:        "error_emits_warning",
			err:         errors.New("generate diff: exec failed"),
			wantWarning: true,
		},
		{
			name:        "nil_error_no_output",
			err:         nil,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			warnDiffArtifacts(&buf, tt.err)
			if tt.wantWarning {
				require.Contains(t, buf.String(), "warning:")
				require.Contains(t, buf.String(), "diff artifacts skipped")
				require.Contains(t, buf.String(), tt.err.Error())
			} else {
				require.Empty(t, buf.String())
			}
		})
	}
}

func TestAppendRunningIndexEntry_Warnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T, evidenceDir, repoRoot string)
		wantWarning string
	}{
		{
			name:        "manifest_read_failure",
			setup:       func(t *testing.T, evidenceDir, repoRoot string) {},
			wantWarning: "read manifest",
		},
		{
			name: "append_failure",
			setup: func(t *testing.T, evidenceDir, repoRoot string) {
				writeManifestFixture(t, evidenceDir)
				runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
				require.NoError(t, os.MkdirAll(runsDir, 0o700))
				require.NoError(t, os.Chmod(runsDir, 0o444))
				t.Cleanup(func() { os.Chmod(runsDir, 0o700) })
			},
			wantWarning: "open index file",
		},
		{
			name: "success_no_warning",
			setup: func(t *testing.T, evidenceDir, repoRoot string) {
				writeManifestFixture(t, evidenceDir)
				runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
				require.NoError(t, os.MkdirAll(runsDir, 0o700))
			},
			wantWarning: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evidenceDir := t.TempDir()
			repoRoot := t.TempDir()
			tt.setup(t, evidenceDir, repoRoot)

			var buf bytes.Buffer
			appendRunningIndexEntry(&buf, repoRoot, evidenceDir)

			if tt.wantWarning != "" {
				require.Contains(t, buf.String(), "warning:")
				require.Contains(t, buf.String(), tt.wantWarning)
			} else {
				require.Empty(t, buf.String())
			}
		})
	}
}
