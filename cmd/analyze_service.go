package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lallihdp-web/go-query-analyzer/internal/analyzer"
	"github.com/lallihdp-web/go-query-analyzer/internal/output"
	"github.com/spf13/cobra"
)

var analyzeServiceCmd = &cobra.Command{
	Use:   "analyze-service [path]",
	Short: "Analyze service/handler files for multi-repository calls",
	Long: `Analyze Go service or handler files for database repository issues.

This command specifically looks for:
  - Multiple repository calls in a single function
  - Repository calls inside loops (N+1 at service layer)
  - Functions that could benefit from transactions or batching

Examples:
  go-query-analyzer analyze-service ./core/service
  go-query-analyzer analyze-service ./handler
  go-query-analyzer analyze-service ./internal/service -f json`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAnalyzeService,
}

func init() {
	rootCmd.AddCommand(analyzeServiceCmd)
}

func runAnalyzeService(cmd *cobra.Command, args []string) error {
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
		// Only process Go files (include test files for services)
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

	// Create analyzer with repo analysis enabled
	a := analyzer.New(analyzer.Config{
		Verbose:      verbose,
		SchemaFile:   schemaFile,
		AnalyzeRepos: true, // Enable multi-repo detection
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

	// Filter to only show multi-repo issues (since this is a service-specific command)
	var serviceIssues []analyzer.Issue
	for _, issue := range allIssues {
		if issue.Type == "multi-repo" {
			serviceIssues = append(serviceIssues, issue)
		}
	}

	// Output results
	formatter := output.NewFormatter(outputFormat)
	return formatter.Format(os.Stdout, serviceIssues)
}
