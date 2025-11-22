package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lallihdp-web/codereview/internal/analyzer"
	"github.com/lallihdp-web/codereview/internal/output"
	"github.com/spf13/cobra"
)

var (
	analyzeRepos bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze Go files for database query issues",
	Long: `Analyze Go source files or directories for database query issues.

Examples:
  go-query-analyzer analyze .
  go-query-analyzer analyze ./internal/repository
  go-query-analyzer analyze main.go
  go-query-analyzer analyze . -f json
  go-query-analyzer analyze . -f reviewdog
  go-query-analyzer analyze ./core/service --repos`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeRepos, "repos", false, "Enable multi-repository call detection for services/handlers")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Collect all Go files
	var files []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip vendor and hidden directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// Only process Go files
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk path: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No Go files found to analyze")
		return nil
	}

	// Create analyzer
	a := analyzer.New(analyzer.Config{
		Verbose:      verbose,
		SchemaFile:   schemaFile,
		AnalyzeRepos: analyzeRepos,
	})

	// Analyze files
	var allIssues []analyzer.Issue
	for _, file := range files {
		issues, err := a.AnalyzeFile(file)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", file, err)
			}
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	// Output results
	formatter := output.NewFormatter(outputFormat)
	return formatter.Format(os.Stdout, allIssues)
}
