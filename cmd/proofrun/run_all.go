package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yebiguo/proofrun/internal/ci"
)

var (
	runAllTimeoutSeconds int
	runAllOnly           []string
)

var runAllCmd = &cobra.Command{
	Use:   "run-all",
	Short: "Run every check declared in .proofrun.yml",
	Long: `run-all reads .proofrun.yml and executes every declared check's
command directly — there is no separate CLI-supplied command to compare
against, since each check's command comes from exactly one place. Every
declared check is attempted even if an earlier one fails, and each
result is saved to the receipt immediately after it finishes, so a
mid-run interruption doesn't lose already-completed results.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		var timeout time.Duration
		if runAllTimeoutSeconds > 0 {
			timeout = time.Duration(runAllTimeoutSeconds) * time.Second
		}

		only := map[string]bool{}
		for _, n := range runAllOnly {
			only[n] = true
		}

		outcomes, err := ci.RunAll(context.Background(), dir, timeout, only, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		failed := false
		for _, o := range outcomes {
			if o.Failed() {
				failed = true
			}
		}
		if failed {
			fmt.Fprintln(cmd.OutOrStdout())
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	runAllCmd.Flags().IntVar(&runAllTimeoutSeconds, "timeout", 0, "kill each check after this many seconds (0 = no timeout)")
	runAllCmd.Flags().StringSliceVar(&runAllOnly, "only", nil, "only run checks with these names (repeatable, default: all)")
	rootCmd.AddCommand(runAllCmd)
}
