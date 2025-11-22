package patterns

// DatabasePattern represents a pattern that indicates database access
type DatabasePattern struct {
	Package     string   // Import path
	Type        string   // Struct/interface name (empty for package-level functions)
	Methods     []string // Method names that execute queries
	Description string   // Human-readable description
	Category    string   // Category: "orm", "builder", "driver", "transaction"
}

// GetAllPatterns returns all known database access patterns
func GetAllPatterns() []DatabasePattern {
	return []DatabasePattern{
		// Bob ORM patterns
		{
			Package:     "github.com/stephenafamo/bob",
			Methods:     []string{"One", "All", "Cursor", "Count", "Exists"},
			Description: "Bob ORM query execution",
			Category:    "orm",
		},
		{
			Package:     "github.com/stephenafamo/bob/dialect/psql",
			Methods:     []string{"Select", "Insert", "Update", "Delete", "Raw"},
			Description: "Bob PostgreSQL dialect",
			Category:    "orm",
		},
		{
			Package:     "github.com/stephenafamo/bob/dialect/psql/sm",
			Methods:     []string{"From", "Where", "Join", "LeftJoin", "InnerJoin"},
			Description: "Bob select mods",
			Category:    "orm",
		},

		// Squirrel patterns
		{
			Package:     "github.com/Masterminds/squirrel",
			Methods:     []string{"Select", "Insert", "Update", "Delete", "RunWith", "ToSql"},
			Description: "Squirrel query builder",
			Category:    "builder",
		},

		// api-db patterns
		{
			Package:     "github.com/lallihdp-web/api-db",
			Type:        "DB",
			Methods:     []string{"WithTx", "ReadTx", "Ping", "PingContext"},
			Description: "api-db database operations",
			Category:    "driver",
		},

		// Standard database/sql
		{
			Package:     "database/sql",
			Type:        "DB",
			Methods:     []string{"Query", "QueryRow", "QueryContext", "QueryRowContext", "Exec", "ExecContext"},
			Description: "Standard database/sql",
			Category:    "driver",
		},
		{
			Package:     "database/sql",
			Type:        "Tx",
			Methods:     []string{"Query", "QueryRow", "QueryContext", "QueryRowContext", "Exec", "ExecContext"},
			Description: "Standard database/sql transaction",
			Category:    "transaction",
		},
		{
			Package:     "database/sql",
			Type:        "Rows",
			Methods:     []string{"Scan", "Next"},
			Description: "Standard database/sql rows",
			Category:    "driver",
		},

		// pgx patterns
		{
			Package:     "github.com/jackc/pgx/v5",
			Type:        "Conn",
			Methods:     []string{"Query", "QueryRow", "Exec", "Begin"},
			Description: "pgx connection",
			Category:    "driver",
		},
		{
			Package:     "github.com/jackc/pgx/v5/pgxpool",
			Type:        "Pool",
			Methods:     []string{"Query", "QueryRow", "Exec", "Begin", "Acquire"},
			Description: "pgx connection pool",
			Category:    "driver",
		},
		{
			Package:     "github.com/jackc/pgx/v5",
			Type:        "Tx",
			Methods:     []string{"Query", "QueryRow", "Exec", "Commit", "Rollback"},
			Description: "pgx transaction",
			Category:    "transaction",
		},
		{
			Package:     "github.com/jackc/pgx/v5",
			Type:        "Rows",
			Methods:     []string{"Scan", "Next", "Close"},
			Description: "pgx rows",
			Category:    "driver",
		},
	}
}

// QueryExecutionMethods returns methods that actually execute database queries
func QueryExecutionMethods() map[string]bool {
	return map[string]bool{
		// database/sql
		"Query":           true,
		"QueryRow":        true,
		"QueryContext":    true,
		"QueryRowContext": true,
		"Exec":            true,
		"ExecContext":     true,

		// Bob ORM
		"One":    true,
		"All":    true,
		"Cursor": true,
		"Count":  true,
		"Exists": true,

		// Squirrel (when used with RunWith)
		"RunWith": true,

		// pgx
		"Acquire": true,
		"Begin":   true,

		// Transactions
		"WithTx":  true,
		"ReadTx":  true,
		"BeginTx": true,

		// api-db package functions (gitlab.cept.gov.in/it-2.0-common/api-db)
		"SelectOne":     true,
		"SelectRows":    true,
		"QueueReturn":   true,
		"QueueExecRow":  true,
		"QueueReturnRow": true,
	}
}

// PackageLevelDBFunctions returns package-level functions that execute queries immediately
// These are called as dblib.FunctionName() rather than obj.Method()
func PackageLevelDBFunctions() map[string]bool {
	return map[string]bool{
		// Select functions (execute immediately)
		"SelectOne":     true,
		"SelectRows":    true,
		"SelectOneOK":   true,
		"SelectRowsTag": true,

		// Insert/Update functions (execute immediately)
		"Insert":              true,
		"Update":              true,
		"InsertReturning":     true,
		"UpdateReturning":     true,
		"InsertReturningrows": true,
	}
}

// BatchQueueFunctions returns functions that queue queries for batch execution
// These don't execute immediately, so multiple calls are OK
func BatchQueueFunctions() map[string]bool {
	return map[string]bool{
		"QueueReturn":    true,
		"QueueExecRow":   true,
		"QueueReturnRow": true,
	}
}

// BatchExecutionMethods returns methods that execute batched queries
// Multiple calls to SendBatch in a single function = multiple round trips
func BatchExecutionMethods() map[string]bool {
	return map[string]bool{
		"SendBatch": true,
	}
}

// RawDBMethodsToDblib maps raw database methods to their recommended dblib alternatives
// These are methods that should ideally use dblib functions instead
func RawDBMethodsToDblib() map[string]string {
	return map[string]string{
		// Raw query methods -> dblib alternatives
		"Query":           "dblib.SelectRows",
		"QueryRow":        "dblib.SelectOne",
		"QueryContext":    "dblib.SelectRows",
		"QueryRowContext": "dblib.SelectOne",
		"Exec":            "dblib.Insert/Update",
		"ExecContext":     "dblib.Insert/Update",
	}
}

// RawDBReceivers returns receiver patterns that indicate raw database usage
func RawDBReceivers() []string {
	return []string{
		"db", "DB", "tx", "Tx", "conn", "Conn", "pool", "Pool",
		".db", ".DB", ".tx", ".Tx", ".conn", ".Conn", ".pool", ".Pool",
	}
}

// LoopSensitiveMethods returns methods that are problematic when called in loops
func LoopSensitiveMethods() map[string]bool {
	return map[string]bool{
		"Query":           true,
		"QueryRow":        true,
		"QueryContext":    true,
		"QueryRowContext": true,
		"Exec":            true,
		"ExecContext":     true,
		"One":             true,
		"All":             true,
		"Count":           true,
		"Exists":          true,
		"WithTx":          true,
		"ReadTx":          true,

		// api-db package functions (immediate execution)
		"SelectOne":           true,
		"SelectRows":          true,
		"SelectOneOK":         true,
		"SelectRowsTag":       true,
		"Insert":              true,
		"Update":              true,
		"InsertReturning":     true,
		"UpdateReturning":     true,
		"InsertReturningrows": true,
	}
}

// SQLAntiPatterns defines SQL patterns to detect
type SQLAntiPattern struct {
	Name        string
	Pattern     string // Regex pattern
	Description string
	Severity    string // "error", "warning", "info"
}

// GetSQLAntiPatterns returns SQL anti-patterns to detect
func GetSQLAntiPatterns() []SQLAntiPattern {
	return []SQLAntiPattern{
		{
			Name:        "select_star",
			Pattern:     `(?i)SELECT\s+\*`,
			Description: "SELECT * fetches all columns, which may be inefficient",
			Severity:    "warning",
		},
		{
			Name:        "missing_limit",
			Pattern:     `(?i)SELECT\s+.+\s+FROM\s+\w+(?:\s+WHERE\s+.+)?(?:\s+ORDER\s+BY\s+.+)?$`,
			Description: "Query without LIMIT may return unbounded results",
			Severity:    "warning",
		},
		{
			Name:        "delete_without_where",
			Pattern:     `(?i)DELETE\s+FROM\s+\w+\s*$`,
			Description: "DELETE without WHERE clause will delete all rows",
			Severity:    "error",
		},
		{
			Name:        "update_without_where",
			Pattern:     `(?i)UPDATE\s+\w+\s+SET\s+.+(?:$|;)`,
			Description: "UPDATE without WHERE clause will update all rows",
			Severity:    "error",
		},
		{
			Name:        "like_leading_wildcard",
			Pattern:     `(?i)LIKE\s+['"]%`,
			Description: "LIKE with leading wildcard prevents index usage",
			Severity:    "info",
		},
		{
			Name:        "not_in_subquery",
			Pattern:     `(?i)NOT\s+IN\s*\(SELECT`,
			Description: "NOT IN with subquery can be slow, consider NOT EXISTS",
			Severity:    "info",
		},
		{
			Name:        "or_in_where",
			Pattern:     `(?i)WHERE\s+.+\s+OR\s+`,
			Description: "OR in WHERE clause may prevent index usage",
			Severity:    "info",
		},
	}
}
