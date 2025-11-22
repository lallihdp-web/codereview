package detector

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/lallihdp-web/go-query-analyzer/internal/parser"
	"github.com/lallihdp-web/go-query-analyzer/internal/patterns"
)

// NPlusOneIssue represents an N+1 query problem
type NPlusOneIssue struct {
	File      string
	Line      int
	Column    int
	Function  string
	Method    string
	Receiver  string
	LoopType  string
	LoopDepth int
	Message   string
}

// NPlusOneDetector detects N+1 query patterns
type NPlusOneDetector struct {
	sensitiveMeths map[string]bool
}

// NewNPlusOneDetector creates a new N+1 detector
func NewNPlusOneDetector() *NPlusOneDetector {
	return &NPlusOneDetector{
		sensitiveMeths: patterns.LoopSensitiveMethods(),
	}
}

// Detect finds N+1 query issues in the parsed file
func (d *NPlusOneDetector) Detect(pf *parser.ParsedFile) []NPlusOneIssue {
	var issues []NPlusOneIssue

	funcs := pf.GetFunctions()
	for _, fn := range funcs {
		issues = append(issues, d.detectInFunction(pf, fn)...)
	}

	return issues
}

func (d *NPlusOneDetector) detectInFunction(pf *parser.ParsedFile, fn parser.FunctionInfo) []NPlusOneIssue {
	var issues []NPlusOneIssue

	loops := parser.GetLoops(fn.Body)
	for _, loop := range loops {
		issues = append(issues, d.detectInLoop(pf, fn, loop, 1)...)
	}

	return issues
}

func (d *NPlusOneDetector) detectInLoop(pf *parser.ParsedFile, fn parser.FunctionInfo, loop parser.LoopInfo, depth int) []NPlusOneIssue {
	var issues []NPlusOneIssue

	calls := parser.GetCalls(loop.Body)
	for _, call := range calls {
		if d.isDBCall(call, pf.Imports) {
			pos := pf.FileSet.Position(call.Pos)
			issues = append(issues, NPlusOneIssue{
				File:      pf.Path,
				Line:      pos.Line,
				Column:    pos.Column,
				Function:  fn.Name,
				Method:    call.Method,
				Receiver:  call.Receiver,
				LoopType:  loop.Type,
				LoopDepth: depth,
				Message:   d.buildMessage(call, loop, depth),
			})
		}
	}

	// Check nested loops
	nestedLoops := parser.GetLoops(loop.Body)
	for _, nested := range nestedLoops {
		// Skip if it's the same loop
		if nested.StartPos == loop.StartPos {
			continue
		}
		issues = append(issues, d.detectInLoop(pf, fn, nested, depth+1)...)
	}

	return issues
}

func (d *NPlusOneDetector) isDBCall(call parser.CallInfo, imports map[string]string) bool {
	// Check if method is a known database operation
	if !d.sensitiveMeths[call.Method] {
		return false
	}

	// Check receiver against known patterns
	receiver := call.Receiver
	if receiver == "" {
		return false
	}

	// Check for common database receiver patterns
	// These can be exact matches or suffixes (e.g., "r.db" ends with ".db")
	dbReceivers := []string{
		"db", "DB", "tx", "Tx", "conn", "Conn", "pool", "Pool",
		"repo", "repository", "store", "dao",
		"Psql", "psql", // api-db Squirrel builder
		"rows", "Rows", "orderRows", "productRows", "categories", "products",
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

	// Check against imported packages
	for alias, path := range imports {
		if receiver == alias {
			// Check if it's a known database package
			dbPackages := []string{
				"database/sql",
				"github.com/jackc/pgx",
				"github.com/stephenafamo/bob",
				"github.com/Masterminds/squirrel",
				"github.com/lallihdp-web/api-db",
			}
			for _, dbPkg := range dbPackages {
				if path == dbPkg || containsSubstring(path, dbPkg) {
					return true
				}
			}
		}
	}

	return false
}

func (d *NPlusOneDetector) buildMessage(call parser.CallInfo, loop parser.LoopInfo, depth int) string {
	severity := "N+1"
	if depth > 1 {
		severity = fmt.Sprintf("N+%d (nested loop depth: %d)", depth, depth)
	}

	return fmt.Sprintf(
		"%s query detected: '%s.%s()' called inside %s loop. Consider batching queries or using JOINs.",
		severity, call.Receiver, call.Method, loop.Type,
	)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)
}

// DetectInBlock detects N+1 issues in a specific block with loop context
func (d *NPlusOneDetector) DetectInBlock(pf *parser.ParsedFile, block *ast.BlockStmt, fnName string) []NPlusOneIssue {
	var issues []NPlusOneIssue

	// Get all calls in the block
	calls := parser.GetCalls(block)

	for _, call := range calls {
		// Check if call is inside a loop
		inLoop, loopDepth := parser.IsInLoop(block, call.Pos)
		if !inLoop {
			continue
		}

		if d.isDBCall(call, pf.Imports) {
			pos := pf.FileSet.Position(call.Pos)
			loopType := "for" // Default
			issues = append(issues, NPlusOneIssue{
				File:      pf.Path,
				Line:      pos.Line,
				Column:    pos.Column,
				Function:  fnName,
				Method:    call.Method,
				Receiver:  call.Receiver,
				LoopType:  loopType,
				LoopDepth: loopDepth,
				Message: fmt.Sprintf(
					"N+1 query detected: '%s.%s()' called inside loop (depth: %d). Consider batching queries or using JOINs.",
					call.Receiver, call.Method, loopDepth,
				),
			})
		}
	}

	return issues
}

// Position returns file position info
func (n NPlusOneIssue) Position() token.Position {
	return token.Position{
		Filename: n.File,
		Line:     n.Line,
		Column:   n.Column,
	}
}
