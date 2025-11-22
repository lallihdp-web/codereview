package analyzer

import (
	"github.com/lallihdp-web/go-query-analyzer/internal/detector"
	"github.com/lallihdp-web/go-query-analyzer/internal/parser"
)

// Config holds analyzer configuration
type Config struct {
	Verbose       bool
	SchemaFile    string
	AnalyzeRepos  bool // Enable multi-repository call detection for services/handlers
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
	multiRepo       *detector.MultiRepoDetector
	rawDB           *detector.RawDBDetector
}

// New creates a new analyzer
func New(config Config) *Analyzer {
	return &Analyzer{
		config:     config,
		nPlusOne:   detector.NewNPlusOneDetector(),
		multiTrip:  detector.NewMultiTripDetector(),
		sqlPattern: detector.NewSQLPatternDetector(),
		multiRepo:  detector.NewMultiRepoDetector(),
		rawDB:      detector.NewRawDBDetector(),
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

	// Run multiple batch detector (multiple SendBatch in one function)
	batchIssues := a.multiTrip.DetectMultipleBatch(pf)
	for _, bi := range batchIssues {
		issues = append(issues, Issue{
			Type:       "multiple-batch",
			Severity:   "warning",
			File:       bi.File,
			Line:       bi.Line,
			Column:     bi.Column,
			Function:   bi.Function,
			Message:    bi.Message,
			Suggestion: bi.Suggestion,
			Details: map[string]any{
				"batch_count": len(bi.BatchCalls),
				"batch_calls": bi.BatchCalls,
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

	// Run raw DB detector (suggest using dblib instead of raw methods)
	rawDBIssues := a.rawDB.Detect(pf)
	for _, ri := range rawDBIssues {
		issues = append(issues, Issue{
			Type:       "raw-db",
			Severity:   "info",
			File:       ri.File,
			Line:       ri.Line,
			Column:     ri.Column,
			Function:   ri.Function,
			Message:    ri.Message,
			Suggestion: "Use dblib functions for consistent error handling and logging.",
			Details: map[string]any{
				"method":      ri.Method,
				"receiver":    ri.Receiver,
				"alternative": ri.Alternative,
			},
		})
	}

	// Run multiple raw DB detector (summary when function has many raw DB calls)
	multiRawDBIssues := a.rawDB.DetectMultiple(pf)
	for _, mr := range multiRawDBIssues {
		issues = append(issues, Issue{
			Type:       "multiple-raw-db",
			Severity:   "warning",
			File:       mr.File,
			Line:       mr.Line,
			Column:     mr.Column,
			Function:   mr.Function,
			Message:    mr.Message,
			Suggestion: mr.Suggestion,
			Details: map[string]any{
				"call_count": mr.CallCount,
				"raw_calls":  mr.RawCalls,
			},
		})
	}

	// Run multi-repo detector (for services/handlers)
	if a.config.AnalyzeRepos {
		multiRepoIssues := a.multiRepo.Detect(pf)
		for _, mr := range multiRepoIssues {
			suggestion := "Consider:\n" +
				"  1. Combine repository calls into a single transaction\n" +
				"  2. Use batch operations to reduce database round trips\n" +
				"  3. Pre-fetch related data before loops\n" +
				"  4. Consider creating a dedicated repository method for this use case"

			issues = append(issues, Issue{
				Type:       "multi-repo",
				Severity:   mr.Severity,
				File:       mr.File,
				Line:       mr.Line,
				Column:     mr.Column,
				Function:   mr.Function,
				Message:    mr.Message,
				Suggestion: suggestion,
				Details: map[string]any{
					"struct":     mr.StructName,
					"repos_used": mr.ReposUsed,
					"in_loop":    mr.IsInLoop,
				},
			})
		}
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
