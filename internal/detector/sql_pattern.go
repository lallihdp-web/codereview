package detector

import (
	"go/token"
	"regexp"
	"strings"

	"github.com/lallihdp-web/codereview/internal/parser"
	"github.com/lallihdp-web/codereview/internal/patterns"
)

// SQLPatternIssue represents an SQL anti-pattern
type SQLPatternIssue struct {
	File        string
	Line        int
	Column      int
	Function    string
	PatternName string
	SQL         string
	Message     string
	Severity    string
}

// SQLPatternDetector detects SQL anti-patterns
type SQLPatternDetector struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	name        string
	regex       *regexp.Regexp
	description string
	severity    string
}

// NewSQLPatternDetector creates a new SQL pattern detector
func NewSQLPatternDetector() *SQLPatternDetector {
	antiPatterns := patterns.GetSQLAntiPatterns()
	compiled := make([]compiledPattern, 0, len(antiPatterns))

	for _, ap := range antiPatterns {
		re, err := regexp.Compile(ap.Pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		compiled = append(compiled, compiledPattern{
			name:        ap.Name,
			regex:       re,
			description: ap.Description,
			severity:    ap.Severity,
		})
	}

	return &SQLPatternDetector{patterns: compiled}
}

// Detect finds SQL anti-patterns in the parsed file
func (d *SQLPatternDetector) Detect(pf *parser.ParsedFile) []SQLPatternIssue {
	var issues []SQLPatternIssue

	funcs := pf.GetFunctions()
	for _, fn := range funcs {
		issues = append(issues, d.detectInFunction(pf, fn)...)
	}

	return issues
}

func (d *SQLPatternDetector) detectInFunction(pf *parser.ParsedFile, fn parser.FunctionInfo) []SQLPatternIssue {
	var issues []SQLPatternIssue

	calls := parser.GetCalls(fn.Body)

	for _, call := range calls {
		// Look for SQL in call arguments
		for _, arg := range call.Args {
			sql, ok := parser.StringLiteralValue(arg)
			if !ok {
				continue
			}

			// Check if it looks like SQL
			if !looksLikeSQL(sql) {
				continue
			}

			// Check against patterns
			for _, pattern := range d.patterns {
				if pattern.regex.MatchString(sql) {
					// Special handling for missing LIMIT - only for SELECT
					if pattern.name == "missing_limit" && !strings.Contains(strings.ToUpper(sql), "SELECT") {
						continue
					}

					// Special handling for UPDATE/DELETE without WHERE
					if pattern.name == "update_without_where" {
						if strings.Contains(strings.ToUpper(sql), "WHERE") {
							continue
						}
					}
					if pattern.name == "delete_without_where" {
						if strings.Contains(strings.ToUpper(sql), "WHERE") {
							continue
						}
					}

					pos := pf.FileSet.Position(call.Pos)
					issues = append(issues, SQLPatternIssue{
						File:        pf.Path,
						Line:        pos.Line,
						Column:      pos.Column,
						Function:    fn.Name,
						PatternName: pattern.name,
						SQL:         truncateSQL(sql),
						Message:     pattern.description,
						Severity:    pattern.severity,
					})
				}
			}
		}
	}

	return issues
}

func looksLikeSQL(s string) bool {
	upper := strings.ToUpper(strings.TrimSpace(s))
	sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "FROM", "WHERE", "JOIN"}
	for _, kw := range sqlKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

func truncateSQL(sql string) string {
	// Clean up whitespace
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) > 100 {
		return sql[:100] + "..."
	}
	return sql
}

// Position returns file position info
func (s SQLPatternIssue) Position() token.Position {
	return token.Position{
		Filename: s.File,
		Line:     s.Line,
		Column:   s.Column,
	}
}
