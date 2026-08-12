// Command proofrun binds check results to the exact git state they ran
// against, so an AI coding agent's "tests pass" claim can be checked
// against reality instead of taken on faith.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version, commit, and date are set via -ldflags at build time (see
// .goreleaser.yml). They default to "dev" for `go run`/`go build` without
// those flags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "proofrun",
	Short: "A local verification receipt for AI coding agents",
	Long: `ProofRun is a local verification receipt for AI coding agents.

It does not judge whether your code is correct — it only proves which
checks actually ran against the exact code you have right now. Run a
check, and ProofRun binds its exit code to your current git commit and
working-tree diff. Change one byte of code afterward, and that result
is immediately STALE.`,
	Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
