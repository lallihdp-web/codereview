package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	verbose      bool
	schemaFile   string
)

var rootCmd = &cobra.Command{
	Use:   "codereview",
	Short: "Analyze Go code for database query issues",
	Long: `A CLI tool to analyze Go code for database query patterns and issues.

Detects:
  - N+1 query problems (queries inside loops)
  - Multiple database trips that could be combined
  - SQL anti-patterns (SELECT *, missing LIMIT, etc.)
  - Inefficient ORM usage patterns

Supports:
  - Bob ORM (github.com/stephenafamo/bob)
  - Squirrel query builder
  - api-db patterns (WithTx, ReadTx, Psql)
  - Standard database/sql and pgx`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "text", "Output format: text, json, reviewdog")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&schemaFile, "schema", "s", "", "Path to schema file for deeper analysis")
}

func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
