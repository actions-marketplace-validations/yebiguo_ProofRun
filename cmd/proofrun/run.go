package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yebiguo/proofrun/internal/git"
	"github.com/yebiguo/proofrun/internal/receipt"
	"github.com/yebiguo/proofrun/internal/runner"
)

var runTimeoutSeconds int

var runCmd = &cobra.Command{
	Use:   "run <check-name> -- <cmd> [args...]",
	Short: "Run a check command and record its result bound to the current git state",
	Long: `Run executes <cmd> as a real subprocess — it never accepts a
pre-computed result — and records its exit code and duration in
.proofrun/receipt.json, bound to the current git HEAD and working-tree
diff. Any later code change makes that record STALE.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dashAt := cmd.ArgsLenAtDash()
		if dashAt == -1 {
			return fmt.Errorf("expected `--` before the command, e.g. proofrun run test -- pytest")
		}
		checkName := args[dashAt-1]
		command := args[dashAt:]
		if checkName == "" {
			return fmt.Errorf("check name must not be empty")
		}
		if len(command) == 0 {
			return fmt.Errorf("no command given after `--`")
		}

		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		head, err := git.Head(dir)
		if err != nil {
			return err
		}
		diff, err := git.DiffFingerprint(dir, receipt.DirName)
		if err != nil {
			return err
		}

		var timeout time.Duration
		if runTimeoutSeconds > 0 {
			timeout = time.Duration(runTimeoutSeconds) * time.Second
		}

		result, err := runner.Run(context.Background(), dir, command, timeout, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if result.TimedOut {
			return fmt.Errorf("check %q timed out after %ds", checkName, runTimeoutSeconds)
		}

		r, err := receipt.Load(dir)
		if err != nil {
			return fmt.Errorf("loading receipt: %w", err)
		}
		fp := receipt.Fingerprint{Head: head, DiffSHA256: diff}
		cr := receipt.NewResult(command, result.ExitCode, result.StartedAt, result.Duration, fp)
		r.Set(checkName, cr)
		if err := r.Save(dir); err != nil {
			return fmt.Errorf("saving receipt: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s (exit %d, %dms)\n", checkName, cr.Status, cr.ExitCode, cr.DurationMS)

		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}
		return nil
	},
}

func init() {
	runCmd.Flags().IntVar(&runTimeoutSeconds, "timeout", 0, "kill the check after this many seconds (0 = no timeout)")
	rootCmd.AddCommand(runCmd)
}
