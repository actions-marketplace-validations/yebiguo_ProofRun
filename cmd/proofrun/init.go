package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yebiguo/proofrun/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a .proofrun.yml in the current directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}

		if config.Exists(dir) {
			return fmt.Errorf("%s already exists in %s", config.FileName, dir)
		}

		cfg := config.Default()
		if err := cfg.Save(dir); err != nil {
			return fmt.Errorf("writing %s: %w", config.FileName, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", config.Path(dir))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
