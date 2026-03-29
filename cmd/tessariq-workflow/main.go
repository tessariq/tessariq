package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/workflow"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "tessariq-workflow",
		Short:         "Tracked-work helper CLI for Tessariq repository planning",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newValidateStateCmd(),
		newNextCmd(),
		newStartCmd(),
		newFinishCmd(),
		newRefreshStateCmd(),
		newVerifyCmd(),
		newFollowupsCmd(),
		newCheckSkillsCmd(),
	)

	return cmd
}

type jsonOption struct {
	json bool
}

func printResult(cmd *cobra.Command, asJSON bool, value any, fallback string) error {
	if !asJSON {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), fallback)
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json result: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func serviceFromCmd(cmd *cobra.Command) (*workflow.Service, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	svc, err := workflow.NewService(wd)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func newValidateStateCmd() *cobra.Command {
	var opt jsonOption

	cmd := &cobra.Command{
		Use:   "validate-state",
		Short: "Validate tracked-work state and tasks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			result, err := svc.ValidateState()
			if err != nil {
				return err
			}

			fallback := "state valid"
			if !result.Valid {
				fallback = "state invalid"
			}

			return printResult(cmd, opt.json, result, fallback)
		},
	}

	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

func newNextCmd() *cobra.Command {
	var opt jsonOption

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Select the next eligible task and apply deterministic recovery",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			result, err := svc.Next()
			if err != nil {
				return err
			}

			fallback := "no eligible task"
			if result.SelectedTask != "" {
				fallback = result.SelectedTask
			}

			return printResult(cmd, opt.json, result, fallback)
		},
	}

	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

func newStartCmd() *cobra.Command {
	var (
		mode    string
		agentID string
		model   string
		opt     jsonOption
	)

	cmd := &cobra.Command{
		Use:   "start <task-id>",
		Short: "Start a tracked task through workflow automation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			input := workflow.StartInput{
				TaskID:  args[0],
				Mode:    mode,
				AgentID: agentID,
				Model:   model,
			}
			result, err := svc.Start(input)
			if err != nil {
				return err
			}

			return printResult(cmd, opt.json, result, result.TaskID)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "user_request", "workflow mode")
	cmd.Flags().StringVar(&agentID, "agent-id", "human", "agent identity")
	cmd.Flags().StringVar(&model, "model", "n/a", "model identifier")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

func newFinishCmd() *cobra.Command {
	var (
		status string
		note   string
		opt    jsonOption
	)

	cmd := &cobra.Command{
		Use:   "finish [task-id]",
		Short: "Finish the active tracked task with evidence-bearing notes",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			taskID := ""
			if len(args) == 1 {
				taskID = args[0]
			}

			input := workflow.FinishInput{
				TaskID: taskID,
				Status: status,
				Note:   note,
			}
			result, err := svc.Finish(input)
			if err != nil {
				return err
			}

			return printResult(cmd, opt.json, result, result.TaskID)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "terminal status: done|blocked|cancelled")
	cmd.Flags().StringVar(&note, "note", "", "evidence-bearing completion note")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("status")
	_ = cmd.MarkFlagRequired("note")
	return cmd
}

func newRefreshStateCmd() *cobra.Command {
	var opt jsonOption

	cmd := &cobra.Command{
		Use:   "refresh-state",
		Short: "Refresh derived state and machine snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			result, err := svc.RefreshState()
			if err != nil {
				return err
			}

			return printResult(cmd, opt.json, result, "state refreshed")
		},
	}

	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	var (
		profile     string
		taskID      string
		disposition string
		opt         jsonOption
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run tracked-work verification and emit artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			input := workflow.VerifyInput{
				Profile:     profile,
				TaskID:      taskID,
				Disposition: disposition,
			}
			result, err := svc.Verify(input)
			if err != nil {
				return err
			}

			return printResult(cmd, opt.json, result, result.ArtifactDir)
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "verification profile: task|implemented|spec")
	cmd.Flags().StringVar(&taskID, "task", "", "task identifier for task-scoped verification")
	cmd.Flags().StringVar(&disposition, "disposition", "report", "verification disposition: report|hybrid")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newFollowupsCmd() *cobra.Command {
	var (
		mode        string
		minSeverity string
		opt         jsonOption
	)

	cmd := &cobra.Command{
		Use:   "followups",
		Short: "Create follow-up tasks from unresolved verification findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			input := workflow.FollowupsInput{
				Mode:        mode,
				MinSeverity: minSeverity,
			}
			result, err := svc.CreateFollowups(input)
			if err != nil {
				return err
			}

			fallback := "no follow-up tasks created"
			if len(result.CreatedTaskIDs) > 0 {
				fallback = result.CreatedTaskIDs[0]
			}

			return printResult(cmd, opt.json, result, fallback)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "follow-up mode")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "minimum severity: low|medium|high")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("mode")
	return cmd
}

func newCheckSkillsCmd() *cobra.Command {
	var opt jsonOption

	cmd := &cobra.Command{
		Use:   "check-skills",
		Short: "Verify mirrored agent skill directories stay identical",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			result, err := svc.CheckSkills()
			if err != nil {
				return err
			}

			if !result.Match {
				if opt.json {
					return printResult(cmd, true, result, "skills mismatch")
				}
				return errors.New("skills mismatch")
			}

			return printResult(cmd, opt.json, result, "skills match")
		},
	}

	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}
