package detector

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/lallihdp-web/go-query-analyzer/internal/parser"
	"github.com/lallihdp-web/go-query-analyzer/internal/patterns"
)

// RawDBIssue represents usage of raw database methods instead of dblib
type RawDBIssue struct {
	File        string
	Line        int
	Column      int
	Function    string
	Method      string
	Receiver    string
	Alternative string
	Message     string
}

// RawDBDetector detects raw database method usage that should use dblib
type RawDBDetector struct {
	rawMethods   map[string]string
	rawReceivers []string
}

// NewRawDBDetector creates a new raw DB detector
func NewRawDBDetector() *RawDBDetector {
	return &RawDBDetector{
		rawMethods:   patterns.RawDBMethodsToDblib(),
		rawReceivers: patterns.RawDBReceivers(),
	}
}

// Detect finds raw database usage issues in the parsed file
func (d *RawDBDetector) Detect(pf *parser.ParsedFile) []RawDBIssue {
	var issues []RawDBIssue

	funcs := pf.GetFunctions()
	for _, fn := range funcs {
		issues = append(issues, d.detectInFunction(pf, fn)...)
	}

	return issues
}

func (d *RawDBDetector) detectInFunction(pf *parser.ParsedFile, fn parser.FunctionInfo) []RawDBIssue {
	var issues []RawDBIssue

	calls := parser.GetCalls(fn.Body)
	for _, call := range calls {
		// Check if method is a raw DB method
		alternative, isRawMethod := d.rawMethods[call.Method]
		if !isRawMethod {
			continue
		}

		// Check if receiver looks like a database object
		if !d.isRawDBReceiver(call.Receiver) {
			continue
		}

		pos := pf.FileSet.Position(call.Pos)
		issues = append(issues, RawDBIssue{
			File:        pf.Path,
			Line:        pos.Line,
			Column:      pos.Column,
			Function:    fn.Name,
			Method:      call.Method,
			Receiver:    call.Receiver,
			Alternative: alternative,
			Message: fmt.Sprintf(
				"Raw database method '%s' used instead of dblib. Consider using %s for better consistency and error handling.",
				call.Method, alternative,
			),
		})
	}

	return issues
}

// isRawDBReceiver checks if the receiver is a database object
func (d *RawDBDetector) isRawDBReceiver(receiver string) bool {
	if receiver == "" {
		return false
	}

	for _, pattern := range d.rawReceivers {
		// Exact match
		if receiver == pattern {
			return true
		}
		// Suffix match (e.g., "r.db" ends with ".db")
		if strings.HasSuffix(receiver, pattern) {
			return true
		}
	}

	return false
}

// Position returns file position info
func (r RawDBIssue) Position() token.Position {
	return token.Position{
		Filename: r.File,
		Line:     r.Line,
		Column:   r.Column,
	}
}
