package detector

import (
	"fmt"
	"go/token"

	"github.com/lallihdp-web/go-query-analyzer/internal/parser"
	"github.com/lallihdp-web/go-query-analyzer/internal/patterns"
)

// MultiTripIssue represents sequential queries that could be combined
type MultiTripIssue struct {
	File       string
	Line       int
	Column     int
	Function   string
	Queries    []QueryLocation
	Message    string
	Suggestion string
}

// MultipleBatchIssue represents multiple SendBatch calls in a single function
type MultipleBatchIssue struct {
	File         string
	Line         int
	Column       int
	Function     string
	BatchCalls   []QueryLocation
	Message      string
	Suggestion   string
}

// QueryLocation holds location info for a query
type QueryLocation struct {
	Line     int
	Column   int
	Method   string
	Receiver string
	SQLHint  string // Extracted SQL if available
}

// MultiTripDetector detects sequential database queries
type MultiTripDetector struct {
	execMethods       map[string]bool
	batchExecMethods  map[string]bool
	maxDistance       int // Max lines between queries to consider them sequential
}

// NewMultiTripDetector creates a new multi-trip detector
func NewMultiTripDetector() *MultiTripDetector {
	return &MultiTripDetector{
		execMethods:      patterns.QueryExecutionMethods(),
		batchExecMethods: patterns.BatchExecutionMethods(),
		maxDistance:      10, // Queries within 10 lines
	}
}

// Detect finds multi-trip query issues in the parsed file
func (d *MultiTripDetector) Detect(pf *parser.ParsedFile) []MultiTripIssue {
	var issues []MultiTripIssue

	funcs := pf.GetFunctions()
	for _, fn := range funcs {
		issues = append(issues, d.detectInFunction(pf, fn)...)
	}

	return issues
}

// DetectMultipleBatch finds functions with multiple SendBatch calls
func (d *MultiTripDetector) DetectMultipleBatch(pf *parser.ParsedFile) []MultipleBatchIssue {
	var issues []MultipleBatchIssue

	funcs := pf.GetFunctions()
	for _, fn := range funcs {
		if issue := d.detectMultipleBatchInFunction(pf, fn); issue != nil {
			issues = append(issues, *issue)
		}
	}

	return issues
}

func (d *MultiTripDetector) detectMultipleBatchInFunction(pf *parser.ParsedFile, fn parser.FunctionInfo) *MultipleBatchIssue {
	calls := parser.GetCalls(fn.Body)

	// Find all SendBatch calls
	var batchCalls []parser.CallInfo
	for _, call := range calls {
		if d.batchExecMethods[call.Method] {
			batchCalls = append(batchCalls, call)
		}
	}

	// Only report if more than one SendBatch call
	if len(batchCalls) <= 1 {
		return nil
	}

	// Build issue
	var batchLocs []QueryLocation
	for _, call := range batchCalls {
		pos := pf.FileSet.Position(call.Pos)
		batchLocs = append(batchLocs, QueryLocation{
			Line:     pos.Line,
			Column:   pos.Column,
			Method:   call.Method,
			Receiver: call.Receiver,
		})
	}

	firstPos := pf.FileSet.Position(batchCalls[0].Pos)

	return &MultipleBatchIssue{
		File:       pf.Path,
		Line:       firstPos.Line,
		Column:     firstPos.Column,
		Function:   fn.Name,
		BatchCalls: batchLocs,
		Message: fmt.Sprintf(
			"Multiple SendBatch calls detected: %d batch executions in function '%s'. Each SendBatch is a database round trip - consider combining into a single batch.",
			len(batchCalls), fn.Name,
		),
		Suggestion: "Combine all batch operations into a single pgx.Batch and execute with one SendBatch call to minimize round trips.",
	}
}

func (d *MultiTripDetector) detectInFunction(pf *parser.ParsedFile, fn parser.FunctionInfo) []MultiTripIssue {
	var issues []MultiTripIssue

	calls := parser.GetCalls(fn.Body)

	// Filter to only DB calls
	var dbCalls []parser.CallInfo
	for _, call := range calls {
		if d.isDBExecCall(call, pf.Imports) {
			dbCalls = append(dbCalls, call)
		}
	}

	if len(dbCalls) < 2 {
		return issues
	}

	// Find sequences of queries that could be combined
	var currentSequence []parser.CallInfo
	var lastLine int

	for _, call := range dbCalls {
		pos := pf.FileSet.Position(call.Pos)

		// Check if this call is in a loop - skip those as they're handled by N+1 detector
		inLoop, _ := parser.IsInLoop(fn.Body, call.Pos)
		if inLoop {
			// Flush current sequence
			if len(currentSequence) >= 2 {
				issues = append(issues, d.buildIssue(pf, fn.Name, currentSequence))
			}
			currentSequence = nil
			continue
		}

		// Check if within distance threshold
		if len(currentSequence) == 0 || pos.Line-lastLine <= d.maxDistance {
			currentSequence = append(currentSequence, call)
			lastLine = pos.Line
		} else {
			// Sequence broken - check if we have an issue
			if len(currentSequence) >= 2 {
				issues = append(issues, d.buildIssue(pf, fn.Name, currentSequence))
			}
			currentSequence = []parser.CallInfo{call}
			lastLine = pos.Line
		}
	}

	// Check remaining sequence
	if len(currentSequence) >= 2 {
		issues = append(issues, d.buildIssue(pf, fn.Name, currentSequence))
	}

	return issues
}

func (d *MultiTripDetector) isDBExecCall(call parser.CallInfo, imports map[string]string) bool {
	if !d.execMethods[call.Method] {
		return false
	}

	receiver := call.Receiver

	// Check for package-level DB function calls (e.g., dblib.SelectOne)
	pkgFuncs := patterns.PackageLevelDBFunctions()
	if pkgFuncs[call.Method] {
		// Check if receiver is a known DB library alias
		dbLibAliases := []string{"dblib", "db", "apidb", "sqldb"}
		for _, alias := range dbLibAliases {
			if receiver == alias {
				return true
			}
		}
		// Also check imports for api-db or database packages
		for alias, path := range imports {
			if receiver == alias {
				if containsAny(path, []string{"api-db", "database", "pgx", "sql"}) {
					return true
				}
			}
		}
	}

	if receiver == "" {
		return false
	}

	// Check for common database receiver patterns
	dbReceivers := []string{
		"db", "DB", "tx", "Tx", "conn", "Conn", "pool", "Pool",
		"repo", "repository", "store", "dao",
		"Psql", "psql",
	}

	for _, dbr := range dbReceivers {
		// Exact match
		if receiver == dbr {
			return true
		}
		// Suffix match (e.g., "r.db" contains ".db")
		if len(receiver) > len(dbr)+1 && receiver[len(receiver)-len(dbr)-1] == '.' && receiver[len(receiver)-len(dbr):] == dbr {
			return true
		}
	}

	// Also check if receiver ends with common DB field names
	dbSuffixes := []string{".db", ".DB", ".tx", ".Tx", ".conn", ".Conn", ".pool", ".Pool", ".Psql"}
	for _, suffix := range dbSuffixes {
		if len(receiver) >= len(suffix) && receiver[len(receiver)-len(suffix):] == suffix {
			return true
		}
	}

	return false
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

func (d *MultiTripDetector) buildIssue(pf *parser.ParsedFile, fnName string, calls []parser.CallInfo) MultiTripIssue {
	var queries []QueryLocation
	for _, call := range calls {
		pos := pf.FileSet.Position(call.Pos)

		// Try to extract SQL hint from arguments
		sqlHint := ""
		if len(call.Args) > 0 {
			if sql, ok := parser.StringLiteralValue(call.Args[0]); ok {
				if len(sql) > 50 {
					sqlHint = sql[:50] + "..."
				} else {
					sqlHint = sql
				}
			}
		}

		queries = append(queries, QueryLocation{
			Line:     pos.Line,
			Column:   pos.Column,
			Method:   call.Method,
			Receiver: call.Receiver,
			SQLHint:  sqlHint,
		})
	}

	firstPos := pf.FileSet.Position(calls[0].Pos)

	return MultiTripIssue{
		File:     pf.Path,
		Line:     firstPos.Line,
		Column:   firstPos.Column,
		Function: fnName,
		Queries:  queries,
		Message: fmt.Sprintf(
			"Multiple database trips detected: %d sequential queries in function '%s'. Consider combining with JOINs or batch operations.",
			len(calls), fnName,
		),
		Suggestion: d.buildSuggestion(queries),
	}
}

func (d *MultiTripDetector) buildSuggestion(queries []QueryLocation) string {
	if len(queries) < 2 {
		return ""
	}

	// Try to suggest based on query patterns
	hasSelect := false
	for _, q := range queries {
		if q.Method == "Query" || q.Method == "QueryRow" || q.Method == "QueryContext" || q.Method == "QueryRowContext" {
			hasSelect = true
		}
	}

	if hasSelect {
		return "Consider using JOINs to fetch related data in a single query, or use batch SELECT with IN clause."
	}

	return "Consider combining queries or using batch operations to reduce database round trips."
}

// Position returns file position info
func (m MultiTripIssue) Position() token.Position {
	return token.Position{
		Filename: m.File,
		Line:     m.Line,
		Column:   m.Column,
	}
}

// Position returns file position info
func (m MultipleBatchIssue) Position() token.Position {
	return token.Position{
		Filename: m.File,
		Line:     m.Line,
		Column:   m.Column,
	}
}
