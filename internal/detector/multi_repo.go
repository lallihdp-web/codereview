package detector

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/lallihdp-web/codereview/internal/parser"
)

// MultiRepoIssue represents multiple repository calls in a single function
type MultiRepoIssue struct {
	File        string
	Line        int
	Column      int
	Function    string
	StructName  string
	ReposUsed   []RepoCall
	Message     string
	Severity    string
	IsInLoop    bool
}

// RepoCall represents a single repository call
type RepoCall struct {
	RepoField  string
	Method     string
	Line       int
	Column     int
	InLoop     bool
	LoopDepth  int
}

// MultiRepoDetector detects multiple repository calls in service/handler functions
type MultiRepoDetector struct {
	repoSuffixes []string
}

// NewMultiRepoDetector creates a new multi-repo detector
func NewMultiRepoDetector() *MultiRepoDetector {
	return &MultiRepoDetector{
		repoSuffixes: []string{
			"Repo", "Repository", "repo", "repository",
			"Store", "store", "DAO", "Dao", "dao",
		},
	}
}

// Detect finds multi-repo issues in the parsed file
func (d *MultiRepoDetector) Detect(pf *parser.ParsedFile) []MultiRepoIssue {
	var issues []MultiRepoIssue

	// Find all struct types and their repo fields
	structRepos := d.findStructRepoFields(pf)

	// Analyze methods for multi-repo calls
	ast.Inspect(pf.AST, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil {
			return true
		}

		// Get receiver type name
		receiverType := d.getReceiverTypeName(fn.Recv)
		if receiverType == "" {
			return true
		}

		// Check if this struct has repo fields
		repoFields, exists := structRepos[receiverType]
		if !exists || len(repoFields) == 0 {
			return true
		}

		// Find all repo calls in this function
		repoCalls := d.findRepoCallsInFunction(pf, fn, repoFields)

		if len(repoCalls) == 0 {
			return true
		}

		// Check for issues
		issue := d.analyzeRepoCalls(pf, fn, receiverType, repoCalls)
		if issue != nil {
			issues = append(issues, *issue)
		}

		return true
	})

	return issues
}

// findStructRepoFields finds all structs and their repository fields
func (d *MultiRepoDetector) findStructRepoFields(pf *parser.ParsedFile) map[string][]string {
	structRepos := make(map[string][]string)

	ast.Inspect(pf.AST, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		var repoFields []string
		for _, field := range st.Fields.List {
			fieldType := d.typeToString(field.Type)

			// Check if field type contains repo-related suffix
			if d.isRepoType(fieldType) {
				for _, name := range field.Names {
					repoFields = append(repoFields, name.Name)
				}
			}
		}

		if len(repoFields) > 0 {
			structRepos[ts.Name.Name] = repoFields
		}

		return true
	})

	return structRepos
}

// isRepoType checks if a type name indicates a repository
func (d *MultiRepoDetector) isRepoType(typeName string) bool {
	for _, suffix := range d.repoSuffixes {
		if strings.Contains(typeName, suffix) {
			return true
		}
	}
	return false
}

// getReceiverTypeName extracts the type name from a method receiver
func (d *MultiRepoDetector) getReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	return d.typeToString(field.Type)
}

// typeToString converts an ast.Expr type to string
func (d *MultiRepoDetector) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return d.typeToString(t.X)
	case *ast.SelectorExpr:
		return d.typeToString(t.X) + "." + t.Sel.Name
	default:
		return ""
	}
}

// findRepoCallsInFunction finds all repository field method calls
func (d *MultiRepoDetector) findRepoCallsInFunction(pf *parser.ParsedFile, fn *ast.FuncDecl, repoFields []string) []RepoCall {
	var calls []RepoCall
	repoFieldSet := make(map[string]bool)
	for _, f := range repoFields {
		repoFieldSet[f] = true
	}

	// Get receiver name
	receiverName := ""
	if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
		receiverName = fn.Recv.List[0].Names[0].Name
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for pattern: receiver.repoField.Method()
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Get the chain: e.g., os.temporalworkflow.FetchWorkFlowDetails
		chain := d.getSelectorChain(sel)
		if len(chain) < 3 {
			return true
		}

		// Check if first element is receiver and second is a repo field
		if chain[0] == receiverName && repoFieldSet[chain[1]] {
			pos := pf.FileSet.Position(call.Pos())
			inLoop, loopDepth := parser.IsInLoop(fn.Body, call.Pos())

			calls = append(calls, RepoCall{
				RepoField: chain[1],
				Method:    chain[2],
				Line:      pos.Line,
				Column:    pos.Column,
				InLoop:    inLoop,
				LoopDepth: loopDepth,
			})
		}

		return true
	})

	return calls
}

// getSelectorChain extracts the full selector chain (e.g., ["os", "repo", "Method"])
func (d *MultiRepoDetector) getSelectorChain(sel *ast.SelectorExpr) []string {
	var chain []string
	chain = append([]string{sel.Sel.Name}, chain...)

	current := sel.X
	for {
		switch x := current.(type) {
		case *ast.SelectorExpr:
			chain = append([]string{x.Sel.Name}, chain...)
			current = x.X
		case *ast.Ident:
			chain = append([]string{x.Name}, chain...)
			return chain
		default:
			return chain
		}
	}
}

// analyzeRepoCalls analyzes repo calls and creates an issue if needed
func (d *MultiRepoDetector) analyzeRepoCalls(pf *parser.ParsedFile, fn *ast.FuncDecl, structName string, calls []RepoCall) *MultiRepoIssue {
	// Group calls by repo field
	repoGroups := make(map[string][]RepoCall)
	for _, call := range calls {
		repoGroups[call.RepoField] = append(repoGroups[call.RepoField], call)
	}

	// Check for issues
	uniqueRepos := len(repoGroups)
	hasLoopCalls := false
	for _, call := range calls {
		if call.InLoop {
			hasLoopCalls = true
			break
		}
	}

	// Only report if multiple repos used OR repo calls in loops
	if uniqueRepos < 2 && !hasLoopCalls {
		return nil
	}

	pos := pf.FileSet.Position(fn.Pos())
	severity := "info"
	var messages []string

	if uniqueRepos >= 2 {
		repoNames := make([]string, 0, len(repoGroups))
		for repo := range repoGroups {
			repoNames = append(repoNames, repo)
		}
		messages = append(messages, fmt.Sprintf("%d different repositories called: %s", uniqueRepos, strings.Join(repoNames, ", ")))
		severity = "warning"
	}

	if hasLoopCalls {
		var loopCallDescs []string
		for _, call := range calls {
			if call.InLoop {
				loopCallDescs = append(loopCallDescs, fmt.Sprintf("%s.%s() at line %d", call.RepoField, call.Method, call.Line))
			}
		}
		messages = append(messages, fmt.Sprintf("Repository calls inside loop (N+1 risk): %s", strings.Join(loopCallDescs, ", ")))
		severity = "error"
	}

	return &MultiRepoIssue{
		File:       pf.Path,
		Line:       pos.Line,
		Column:     pos.Column,
		Function:   fn.Name.Name,
		StructName: structName,
		ReposUsed:  calls,
		Message:    strings.Join(messages, "; "),
		Severity:   severity,
		IsInLoop:   hasLoopCalls,
	}
}

// Position returns file position info
func (m MultiRepoIssue) Position() token.Position {
	return token.Position{
		Filename: m.File,
		Line:     m.Line,
		Column:   m.Column,
	}
}
