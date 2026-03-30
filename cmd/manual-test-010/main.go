package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tessariq/tessariq/internal/adapter/opencode"
	"github.com/tessariq/tessariq/internal/run"
)

func main() {
	allPass := true

	// MT-002: Requested options forwarded (task content as positional arg)
	fmt.Println("=== MT-002: Requested options forwarded ===")
	{
		cfg := run.DefaultConfig()
		cfg.Model = "sonnet"
		cfg.Interactive = true
		p := opencode.New(cfg, "implement feature X")

		// Verify args are just the task — we can't access buildArgs directly,
		// but we can verify via the public metadata that options are handled.
		req := p.Requested()
		if req["interactive"] != true {
			fmt.Println("FAIL: expected requested[interactive]=true")
			allPass = false
		} else {
			fmt.Println("PASS: interactive request recorded")
		}
		if req["model"] != "sonnet" {
			fmt.Println("FAIL: expected requested[model]=sonnet")
			allPass = false
		} else {
			fmt.Println("PASS: model request recorded")
		}
	}

	// MT-003: Unsupported options recorded in adapter.json
	fmt.Println("\n=== MT-003: Unsupported options recorded ===")
	{
		cfg := run.DefaultConfig()
		cfg.Model = "opus"
		cfg.Interactive = true
		p := opencode.New(cfg, "task")

		req := p.Requested()
		app := p.Applied()

		if req["model"] != "opus" {
			fmt.Println("FAIL: expected requested[model]=opus")
			allPass = false
		} else {
			fmt.Println("PASS: requested[model]=opus")
		}
		if req["interactive"] != true {
			fmt.Println("FAIL: expected requested[interactive]=true")
			allPass = false
		} else {
			fmt.Println("PASS: requested[interactive]=true")
		}
		if app["model"] != false {
			fmt.Println("FAIL: expected applied[model]=false")
			allPass = false
		} else {
			fmt.Println("PASS: applied[model]=false (unsupported)")
		}
		if app["interactive"] != false {
			fmt.Println("FAIL: expected applied[interactive]=false")
			allPass = false
		} else {
			fmt.Println("PASS: applied[interactive]=false (unsupported)")
		}

		// Also verify without model
		cfg2 := run.DefaultConfig()
		p2 := opencode.New(cfg2, "task")
		_, hasModel := p2.Requested()["model"]
		_, hasModelApplied := p2.Applied()["model"]
		if hasModel {
			fmt.Println("FAIL: model should be absent from requested when empty")
			allPass = false
		} else {
			fmt.Println("PASS: model absent from requested when not set")
		}
		if hasModelApplied {
			fmt.Println("FAIL: model should be absent from applied when empty")
			allPass = false
		} else {
			fmt.Println("PASS: model absent from applied when not set")
		}
	}

	// MT-005: Missing binary produces actionable guidance
	fmt.Println("\n=== MT-005: Missing binary actionable guidance ===")
	{
		// Override PATH to an empty dir so opencode is not found.
		emptyDir := os.TempDir() + "/empty-path-mt005"
		os.MkdirAll(emptyDir, 0o755)
		defer os.RemoveAll(emptyDir)
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", emptyDir)
		defer os.Setenv("PATH", origPath)

		cfg := run.DefaultConfig()
		p := opencode.New(cfg, "task")
		err := p.Start(context.Background())
		if err == nil {
			fmt.Println("FAIL: expected error when binary not found")
			allPass = false
		} else {
			msg := err.Error()
			checks := []struct {
				substr string
				desc   string
			}{
				{`adapter binary "opencode"`, "mentions adapter binary name"},
				{"container image", "mentions container image"},
				{"--image", "mentions --image flag"},
			}
			for _, c := range checks {
				if contains(msg, c.substr) {
					fmt.Printf("PASS: error %s\n", c.desc)
				} else {
					fmt.Printf("FAIL: error does not contain %q (%s)\n", c.substr, c.desc)
					allPass = false
				}
			}
		}
	}

	fmt.Println()
	if allPass {
		fmt.Println("ALL MANUAL TESTS PASSED")
	} else {
		fmt.Println("SOME MANUAL TESTS FAILED")
		os.Exit(1)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
