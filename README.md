# Go Query Analyzer

A CLI tool to analyze Go code for database query issues, helping identify performance problems and enforce best practices.

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/lallihdp-web/codereview.git
cd codereview

# Build the binary
go build -o go-query-analyzer ./main.go

# (Optional) Install to your PATH
go install
```

### Quick Install

```bash
go install github.com/lallihdp-web/codereview@latest

# Or build from source
git clone https://github.com/lallihdp-web/codereview.git
cd codereview
go build -o go-query-analyzer .
```

## Usage

### Basic Analysis

```bash
# Analyze a directory
./go-query-analyzer analyze ./path/to/code

# Analyze with multi-repository detection (for services/handlers)
./go-query-analyzer analyze ./path/to/code --repos

# Analyze specific service/handler directories
./go-query-analyzer analyze-service ./core/service
```

### Output Formats

```bash
# Text output (default) - human readable
./go-query-analyzer analyze ./path

# JSON output - for CI/CD pipelines
./go-query-analyzer analyze ./path -f json

# Reviewdog format - for PR comments
./go-query-analyzer analyze ./path -f reviewdog
```

### CI/CD Integration

```bash
# Use with reviewdog
./go-query-analyzer analyze ./src -f reviewdog | reviewdog -f=rdjsonl -reporter=github-pr-review

# JSON for custom processing
./go-query-analyzer analyze ./src -f json > report.json
```

## What It Detects

### 1. N+1 Query Detection (`n+1`)

Detects database calls inside loops that cause N+1 query problems.

```go
// BAD: N+1 query - executes query for each user
for _, user := range users {
    orders, _ := db.Query("SELECT * FROM orders WHERE user_id = ?", user.ID)
}

// GOOD: Batch query
userIDs := extractIDs(users)
orders, _ := db.Query("SELECT * FROM orders WHERE user_id IN (?)", userIDs)
```

| Severity | Condition |
|----------|-----------|
| `error` | Nested loops (depth > 1) |
| `warning` | Single loop |

### 2. Multi-Trip Detection (`multi-trip`)

Detects sequential database queries that could be combined.

```go
// BAD: Multiple round trips
user, _ := dblib.SelectOne(ctx, db, userQuery)
orders, _ := dblib.SelectRows(ctx, db, ordersQuery)
payments, _ := dblib.SelectRows(ctx, db, paymentsQuery)

// GOOD: Use JOIN or batch
result, _ := dblib.SelectOne(ctx, db, joinedQuery)
```

| Severity | Condition |
|----------|-----------|
| `error` | 5+ sequential queries |
| `warning` | 3-4 sequential queries |
| `info` | 2 sequential queries |

### 3. Multiple SendBatch Detection (`multiple-batch`)

Detects functions with more than one `SendBatch` call.

```go
// BAD: Multiple SendBatch = multiple round trips
batch1.Queue(query1)
db.SendBatch(ctx, batch1)  // Round trip 1
batch2.Queue(query2)
db.SendBatch(ctx, batch2)  // Round trip 2

// GOOD: Single batch
batch.Queue(query1)
batch.Queue(query2)
db.SendBatch(ctx, batch)   // Single round trip
```

| Severity | Condition |
|----------|-----------|
| `warning` | 2+ SendBatch calls in one function |

### 4. SQL Anti-Pattern Detection (`sql-pattern`)

| Pattern | Severity | Description |
|---------|----------|-------------|
| `SELECT *` | `warning` | Fetches all columns, may be inefficient |
| Missing `LIMIT` | `warning` | Query may return unbounded results |
| `DELETE` without `WHERE` | `error` | Will delete all rows |
| `UPDATE` without `WHERE` | `error` | Will update all rows |
| `LIKE '%...'` | `info` | Leading wildcard prevents index usage |
| `NOT IN (SELECT...)` | `info` | Can be slow, consider NOT EXISTS |
| `OR` in `WHERE` | `info` | May prevent index usage |

### 5. Raw DB Usage Detection (`raw-db`, `multiple-raw-db`)

Detects when raw database methods are used instead of dblib functions.

```go
// BAD: Raw database methods
rows, _ := db.Query("SELECT * FROM users")
row := db.QueryRow("SELECT * FROM users WHERE id = ?", id)

// GOOD: Use dblib functions
users, _ := dblib.SelectRows(ctx, db, query)
user, _ := dblib.SelectOne(ctx, db, query)
```

| Issue Type | Severity | Description |
|------------|----------|-------------|
| `raw-db` | `info` | Individual raw DB call |
| `multiple-raw-db` | `warning` | 2+ raw DB calls in function |

**Raw Method to dblib Mapping:**

| Raw Method | Suggested dblib Alternative |
|------------|----------------------------|
| `Query` | `dblib.SelectRows` |
| `QueryRow` | `dblib.SelectOne` |
| `QueryContext` | `dblib.SelectRows` |
| `QueryRowContext` | `dblib.SelectOne` |
| `Exec` | `dblib.Insert/Update` |
| `ExecContext` | `dblib.Insert/Update` |

### 6. Multi-Repository Detection (`multi-repo`)

*Requires `--repos` flag*

Detects service/handler functions calling multiple repositories.

```go
// WARNING: Multiple repository calls
func (s *Service) ProcessOrder(ctx context.Context, orderID int) error {
    user, _ := s.userRepo.GetByID(ctx, userID)      // Repo 1
    order, _ := s.orderRepo.GetByID(ctx, orderID)   // Repo 2
    payment, _ := s.paymentRepo.Get(ctx, orderID)   // Repo 3
    return nil
}
```

| Severity | Condition |
|----------|-----------|
| `error` | Repository calls inside loops |
| `warning` | 2+ different repositories called |

## Supported Libraries

### ORM/Query Builders
- **Bob ORM** (`github.com/stephenafamo/bob`)
- **Squirrel** (`github.com/Masterminds/squirrel`)

### Database Drivers
- **database/sql** (standard library)
- **pgx** (`github.com/jackc/pgx/v5`)

### Internal Libraries
- **api-db/dblib** (`github.com/lallihdp-web/api-db`)

## dblib Functions Recognized

**Immediate Execution (flagged if sequential/in loops):**
- `SelectOne`, `SelectRows`, `SelectOneOK`, `SelectRowsTag`
- `Insert`, `Update`, `InsertReturning`, `UpdateReturning`, `InsertReturningrows`

**Batch Queue (OK to call multiple times):**
- `QueueReturn`, `QueueExecRow`, `QueueReturnRow`

**Batch Execution (should only have ONE per function):**
- `SendBatch`

## Example Output

### Text Format (Default)

```
=== Analysis Results ===
Found 29 issue(s): 2 error(s), 25 warning(s), 2 info(s)

ERRORS:
--------------------------------------------------
❌ [n+1] /path/to/file.go:797:21
   Function: SaveBulk
   Message: N+2 (nested loop depth: 2) query detected: 'psql.Insert()' called inside range loop.
   Suggestion: Consider:
     1. Use batch queries with IN clause
     2. Use JOINs to fetch related data
     3. Pre-fetch data before the loop

WARNINGS:
--------------------------------------------------
⚠️ [multiple-batch] /path/to/file.go:618:8
   Function: GetCollectionByBkgref
   Message: Multiple SendBatch calls detected: 2 batch executions in function.
   Suggestion: Combine all batch operations into a single pgx.Batch.

INFO:
--------------------------------------------------
ℹ️ [raw-db] /path/to/file.go:330:11
   Function: FetchBarcodeSeries
   Message: Raw database method 'Exec' used instead of dblib.
   Suggestion: Use dblib functions for consistent error handling and logging.
```

### JSON Format

```json
{
  "total_count": 29,
  "error_count": 2,
  "warning_count": 25,
  "info_count": 2,
  "issues": [
    {
      "type": "n+1",
      "severity": "warning",
      "file": "/path/to/file.go",
      "line": 387,
      "column": 16,
      "function": "SaveBulk",
      "message": "N+1 query detected...",
      "suggestion": "Consider using batch queries...",
      "details": {
        "method": "Insert",
        "receiver": "psql",
        "loop_type": "range",
        "loop_depth": 1
      }
    }
  ]
}
```

## Exit Codes

| Code | Description |
|------|-------------|
| `0` | No errors found |
| `1` | Errors found (severity: error) |
| `2` | Analysis failed |

## Project Structure

```
codereview/
├── cmd/
│   ├── root.go              # Root command with flags
│   ├── analyze.go           # Main analyze command
│   └── analyze_service.go   # Service-specific analysis
├── internal/
│   ├── analyzer/
│   │   └── analyzer.go      # Main analyzer coordinating detectors
│   ├── detector/
│   │   ├── n_plus_one.go    # N+1 query detection
│   │   ├── multi_trip.go    # Multi-trip & SendBatch detection
│   │   ├── multi_repo.go    # Multi-repository detection
│   │   ├── sql_pattern.go   # SQL anti-pattern detection
│   │   └── raw_db.go        # Raw DB usage detection
│   ├── output/
│   │   └── formatter.go     # Output formatters (text, JSON, reviewdog)
│   ├── parser/
│   │   └── parser.go        # Go AST parser
│   └── patterns/
│       └── patterns.go      # Database pattern definitions
├── main.go                  # Entry point
├── go.mod
└── README.md
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

MIT License
