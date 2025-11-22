package analyzer

import (
	"github.com/lallihdp-web/go-query-analyzer/internal/detector"
	"github.com/lallihdp-web/go-query-analyzer/internal/parser"
)

// Config holds analyzer configuration
type Config struct {
	Verbose    bool
	SchemaFile string
}

// Issue represents a detected issue
type Issue struct {
	Type       string `json:"type"`       // "n+1", "multi-trip", "sql-pattern"
	Severity   string `json:"severity"`   // "error", "warning", "info"
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Function   string `json:"function"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Details    any    `json:"details,omitempty"`
}

// Analyzer is the main analyzer that coordinates all detectors
type Analyzer struct {
	config          Config
	nPlusOne        *detector.NPlusOneDetector
	multiTrip       *detector.MultiTripDetector
	sqlPattern      *detector.SQLPatternDetector
}

// New creates a new analyzer
func New(config Config) *Analyzer {
	return &Analyzer{
		config:     config,
		nPlusOne:   detector.NewNPlusOneDetector(),
		multiTrip:  detector.NewMultiTripDetector(),
		sqlPattern: detector.NewSQLPatternDetector(),
	}
}

// AnalyzeFile analyzes a single Go file
func (a *Analyzer) AnalyzeFile(path string) ([]Issue, error) {
	pf, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}

	var issues []Issue

	// Run N+1 detector
	nPlusOneIssues := a.nPlusOne.Detect(pf)
	for _, ni := range nPlusOneIssues {
		issues = append(issues, Issue{
			Type:     "n+1",
			Severity: a.getNPlusOneSeverity(ni.LoopDepth),
			File:     ni.File,
			Line:     ni.Line,
			Column:   ni.Column,
			Function: ni.Function,
			Message:  ni.Message,
			Suggestion: "Consider:\n" +
				"  1. Use batch queries with IN clause\n" +
				"  2. Use JOINs to fetch related data\n" +
				"  3. Pre-fetch data before the loop\n" +
				"  4. Use eager loading if using an ORM",
			Details: map[string]any{
				"method":     ni.Method,
				"receiver":   ni.Receiver,
				"loop_type":  ni.LoopType,
				"loop_depth": ni.LoopDepth,
			},
		})
	}

	// Run multi-trip detector
	multiTripIssues := a.multiTrip.Detect(pf)
	for _, mi := range multiTripIssues {
		issues = append(issues, Issue{
			Type:       "multi-trip",
			Severity:   a.getMultiTripSeverity(len(mi.Queries)),
			File:       mi.File,
			Line:       mi.Line,
			Column:     mi.Column,
			Function:   mi.Function,
			Message:    mi.Message,
			Suggestion: mi.Suggestion,
			Details: map[string]any{
				"query_count": len(mi.Queries),
				"queries":     mi.Queries,
			},
		})
	}

	// Run SQL pattern detector
	sqlPatternIssues := a.sqlPattern.Detect(pf)
	for _, si := range sqlPatternIssues {
		issues = append(issues, Issue{
			Type:     "sql-pattern",
			Severity: si.Severity,
			File:     si.File,
			Line:     si.Line,
			Column:   si.Column,
			Function: si.Function,
			Message:  si.Message,
			Details: map[string]any{
				"pattern": si.PatternName,
				"sql":     si.SQL,
			},
		})
	}

	return issues, nil
}

func (a *Analyzer) getNPlusOneSeverity(loopDepth int) string {
	if loopDepth > 1 {
		return "error" // Nested loops are more severe
	}
	return "warning"
}

func (a *Analyzer) getMultiTripSeverity(queryCount int) string {
	if queryCount >= 5 {
		return "error"
	}
	if queryCount >= 3 {
		return "warning"
	}
	return "info"
}
