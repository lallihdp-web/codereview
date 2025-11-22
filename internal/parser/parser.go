package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// ParsedFile represents a parsed Go source file
type ParsedFile struct {
	Path    string
	FileSet *token.FileSet
	AST     *ast.File
	Imports map[string]string // alias -> import path
}

// ParseFile parses a Go source file and returns its AST
func ParseFile(path string) (*ParsedFile, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Extract imports
	imports := make(map[string]string)
	for _, imp := range f.Imports {
		path := imp.Path.Value[1 : len(imp.Path.Value)-1] // Remove quotes
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			// Use last segment of path as alias
			alias = lastSegment(path)
		}
		imports[alias] = path
	}

	return &ParsedFile{
		Path:    path,
		FileSet: fset,
		AST:     f,
		Imports: imports,
	}, nil
}

func lastSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// FunctionInfo holds information about a function
type FunctionInfo struct {
	Name     string
	StartPos token.Pos
	EndPos   token.Pos
	Body     *ast.BlockStmt
}

// GetFunctions extracts all functions from the parsed file
func (p *ParsedFile) GetFunctions() []FunctionInfo {
	var funcs []FunctionInfo

	ast.Inspect(p.AST, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				funcs = append(funcs, FunctionInfo{
					Name:     fn.Name.Name,
					StartPos: fn.Pos(),
					EndPos:   fn.End(),
					Body:     fn.Body,
				})
			}
		}
		return true
	})

	return funcs
}

// LoopInfo holds information about a loop
type LoopInfo struct {
	Type     string // "for", "range"
	StartPos token.Pos
	EndPos   token.Pos
	Body     *ast.BlockStmt
	Parent   ast.Node
}

// GetLoops extracts all loops from a block statement
func GetLoops(block *ast.BlockStmt) []LoopInfo {
	var loops []LoopInfo

	ast.Inspect(block, func(n ast.Node) bool {
		switch loop := n.(type) {
		case *ast.ForStmt:
			if loop.Body != nil {
				loops = append(loops, LoopInfo{
					Type:     "for",
					StartPos: loop.Pos(),
					EndPos:   loop.End(),
					Body:     loop.Body,
					Parent:   loop,
				})
			}
		case *ast.RangeStmt:
			if loop.Body != nil {
				loops = append(loops, LoopInfo{
					Type:     "range",
					StartPos: loop.Pos(),
					EndPos:   loop.End(),
					Body:     loop.Body,
					Parent:   loop,
				})
			}
		}
		return true
	})

	return loops
}

// CallInfo holds information about a function/method call
type CallInfo struct {
	Receiver   string // Empty for function calls
	Method     string
	Args       []ast.Expr
	Pos        token.Pos
	EndPos     token.Pos
	InLoop     bool
	LoopDepth  int
	RawExpr    *ast.CallExpr
	ParentStmt ast.Node
}

// GetCalls extracts all function/method calls from a block
func GetCalls(block *ast.BlockStmt) []CallInfo {
	var calls []CallInfo

	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := CallInfo{
			Pos:     call.Pos(),
			EndPos:  call.End(),
			Args:    call.Args,
			RawExpr: call,
		}

		switch fn := call.Fun.(type) {
		case *ast.Ident:
			// Simple function call
			info.Method = fn.Name
		case *ast.SelectorExpr:
			// Method call or package.Function
			info.Method = fn.Sel.Name
			if ident, ok := fn.X.(*ast.Ident); ok {
				info.Receiver = ident.Name
			} else if sel, ok := fn.X.(*ast.SelectorExpr); ok {
				// Chained call like db.Pool.Query()
				info.Receiver = exprToString(sel)
			} else {
				info.Receiver = exprToString(fn.X)
			}
		}

		if info.Method != "" {
			calls = append(calls, info)
		}

		return true
	})

	return calls
}

// exprToString converts an expression to a string representation
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return exprToString(e.Fun) + "()"
	case *ast.IndexExpr:
		return exprToString(e.X) + "[...]"
	default:
		return "<expr>"
	}
}

// StringLiteralValue extracts string literal value from an expression
func StringLiteralValue(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Remove quotes
			val := e.Value
			if len(val) >= 2 {
				if val[0] == '"' && val[len(val)-1] == '"' {
					return val[1 : len(val)-1], true
				}
				if val[0] == '`' && val[len(val)-1] == '`' {
					return val[1 : len(val)-1], true
				}
			}
		}
	}
	return "", false
}

// IsInLoop checks if a position is within any loop in the block
func IsInLoop(block *ast.BlockStmt, pos token.Pos) (bool, int) {
	depth := 0
	maxDepth := 0

	var checkNode func(n ast.Node) bool
	checkNode = func(n ast.Node) bool {
		switch loop := n.(type) {
		case *ast.ForStmt:
			if loop.Pos() <= pos && pos <= loop.End() {
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
				ast.Inspect(loop.Body, checkNode)
				depth--
				return false
			}
		case *ast.RangeStmt:
			if loop.Pos() <= pos && pos <= loop.End() {
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
				ast.Inspect(loop.Body, checkNode)
				depth--
				return false
			}
		}
		return true
	}

	ast.Inspect(block, checkNode)
	return maxDepth > 0, maxDepth
}
