package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yebiguo/proofrun/internal/report"
)

var reportJSON bool

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print a full report of every check's status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		evals, required, err := evaluateAll(dir)
		if err != nil {
			return err
		}

		rep := report.Build(evals, required)

		if reportJSON {
			data, err := rep.JSON()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		fmt.Fprint(cmd.OutOrStdout(), rep.Terminal())
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "output as JSON instead of a terminal table")
	rootCmd.AddCommand(reportCmd)
}
