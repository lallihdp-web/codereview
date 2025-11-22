package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/lallihdp-web/codereview/internal/analyzer"
)

// Formatter formats analysis results
type Formatter struct {
	format string
}

// NewFormatter creates a new formatter
func NewFormatter(format string) *Formatter {
	return &Formatter{format: format}
}

// Format writes the issues in the specified format
func (f *Formatter) Format(w io.Writer, issues []analyzer.Issue) error {
	switch f.format {
	case "json":
		return f.formatJSON(w, issues)
	case "reviewdog":
		return f.formatReviewdog(w, issues)
	default:
		return f.formatText(w, issues)
	}
}

func (f *Formatter) formatText(w io.Writer, issues []analyzer.Issue) error {
	if len(issues) == 0 {
		fmt.Fprintln(w, "✓ No issues found")
		return nil
	}

	// Group by severity
	errors := filterBySeverity(issues, "error")
	warnings := filterBySeverity(issues, "warning")
	infos := filterBySeverity(issues, "info")

	// Print summary
	fmt.Fprintf(w, "\n=== Analysis Results ===\n")
	fmt.Fprintf(w, "Found %d issue(s): %d error(s), %d warning(s), %d info(s)\n\n",
		len(issues), len(errors), len(warnings), len(infos))

	// Print errors first
	if len(errors) > 0 {
		fmt.Fprintln(w, "ERRORS:")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, issue := range errors {
			f.printIssue(w, issue, "❌")
		}
		fmt.Fprintln(w)
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Fprintln(w, "WARNINGS:")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, issue := range warnings {
			f.printIssue(w, issue, "⚠️")
		}
		fmt.Fprintln(w)
	}

	// Print info
	if len(infos) > 0 {
		fmt.Fprintln(w, "INFO:")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, issue := range infos {
			f.printIssue(w, issue, "ℹ️")
		}
		fmt.Fprintln(w)
	}

	return nil
}

func (f *Formatter) printIssue(w io.Writer, issue analyzer.Issue, icon string) {
	fmt.Fprintf(w, "%s [%s] %s:%d:%d\n", icon, issue.Type, issue.File, issue.Line, issue.Column)
	fmt.Fprintf(w, "   Function: %s\n", issue.Function)
	fmt.Fprintf(w, "   Message: %s\n", issue.Message)
	if issue.Suggestion != "" {
		fmt.Fprintf(w, "   Suggestion: %s\n", issue.Suggestion)
	}
	fmt.Fprintln(w)
}

func (f *Formatter) formatJSON(w io.Writer, issues []analyzer.Issue) error {
	output := struct {
		TotalCount   int              `json:"total_count"`
		ErrorCount   int              `json:"error_count"`
		WarningCount int              `json:"warning_count"`
		InfoCount    int              `json:"info_count"`
		Issues       []analyzer.Issue `json:"issues"`
	}{
		TotalCount:   len(issues),
		ErrorCount:   len(filterBySeverity(issues, "error")),
		WarningCount: len(filterBySeverity(issues, "warning")),
		InfoCount:    len(filterBySeverity(issues, "info")),
		Issues:       issues,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// ReviewdogDiagnostic represents a reviewdog diagnostic
type ReviewdogDiagnostic struct {
	Message  string              `json:"message"`
	Location ReviewdogLocation   `json:"location"`
	Severity string              `json:"severity"`
	Source   ReviewdogSource     `json:"source"`
	Code     *ReviewdogCode      `json:"code,omitempty"`
}

// ReviewdogLocation represents a location in reviewdog format
type ReviewdogLocation struct {
	Path  string              `json:"path"`
	Range ReviewdogRange      `json:"range"`
}

// ReviewdogRange represents a range in reviewdog format
type ReviewdogRange struct {
	Start ReviewdogPosition `json:"start"`
	End   ReviewdogPosition `json:"end,omitempty"`
}

// ReviewdogPosition represents a position in reviewdog format
type ReviewdogPosition struct {
	Line   int `json:"line"`
	Column int `json:"column,omitempty"`
}

// ReviewdogSource represents the source of a diagnostic
type ReviewdogSource struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// ReviewdogCode represents a diagnostic code
type ReviewdogCode struct {
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

func (f *Formatter) formatReviewdog(w io.Writer, issues []analyzer.Issue) error {
	// Reviewdog uses JSON Lines format (rdjsonl)
	for _, issue := range issues {
		diag := ReviewdogDiagnostic{
			Message: issue.Message,
			Location: ReviewdogLocation{
				Path: issue.File,
				Range: ReviewdogRange{
					Start: ReviewdogPosition{
						Line:   issue.Line,
						Column: issue.Column,
					},
				},
			},
			Severity: mapSeverity(issue.Severity),
			Source: ReviewdogSource{
				Name: "codereview",
				URL:  "https://github.com/lallihdp-web/codereview",
			},
			Code: &ReviewdogCode{
				Value: issue.Type,
			},
		}

		data, err := json.Marshal(diag)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
	}

	return nil
}

func filterBySeverity(issues []analyzer.Issue, severity string) []analyzer.Issue {
	var filtered []analyzer.Issue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func mapSeverity(severity string) string {
	switch severity {
	case "error":
		return "ERROR"
	case "warning":
		return "WARNING"
	default:
		return "INFO"
	}
}
