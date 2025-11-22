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
  codereview analyze .
  codereview analyze ./internal/repository
  codereview analyze main.go
  codereview analyze . -f json
  codereview analyze . -f reviewdog
  codereview analyze ./core/service --repos`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeRepos, "repos", false, "Enable multi-repository call detection for services/handlers")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Convert to absolute path for Windows compatibility
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	if verbose {
		fmt.Printf("Analyzing path: %s\n", absPath)
	}

	// Collect all Go files
	var files []string

	if !info.IsDir() {
		// Single file
		if strings.HasSuffix(absPath, ".go") {
			files = append(files, absPath)
		}
	} else {
		// Directory - walk it
		err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: error accessing %s: %v\n", p, err)
				}
				return nil // Continue walking
			}
			// Skip vendor and hidden directories
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			// Only process Go files (not test files)
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				files = append(files, p)
				if verbose {
					fmt.Printf("Found: %s\n", p)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk path: %w", err)
		}
	}

	if len(files) == 0 {
		fmt.Printf("No Go files found in: %s\n", absPath)
		return nil
	}

	if verbose {
		fmt.Printf("Found %d Go files to analyze\n", len(files))
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
