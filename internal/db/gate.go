package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var forbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|REPLACE)\b`),
	regexp.MustCompile(`(?i)\bATTACH\b`),
	regexp.MustCompile(`(?i)\bPRAGMA\b`),
	regexp.MustCompile(`--`),
}

// semicolonGuard is checked separately from forbiddenPatterns because it
// requires string-literal stripping before evaluation. Keeping it as a named
// variable avoids relying on a brittle slice index.
var semicolonGuard = regexp.MustCompile(`(?m);.*\S+.*$`)

// allowedPragmas lists PRAGMA statements that pass the read-only gate.
var allowedPragmas = []string{"table_info", "table_list", "database_list"}

// MaxRows caps the number of rows returned by a gated query.
const MaxRows = 500

// GateResult reports whether a SQL statement passed the read-only gate.
type GateResult struct {
	Allowed bool
	Reason  string
}

// stripStringLiterals removes the content of single-quoted SQL string literals
// so that structural pattern checks (like the semicolon guard) do not trigger
// on values embedded inside strings. The function handles SQL escaped quotes
// (”) and conservatively rejects unterminated string literals.
func stripStringLiterals(sql string) (string, error) {
	// Step 1: Replace escaped quotes ('') with a placeholder that cannot
	// appear in valid SQL identifiers or string content.
	const placeholder = "\x00\x00"
	normalized := strings.ReplaceAll(sql, "''", placeholder)

	// Step 2: Strip content between remaining single-quote pairs.
	var result strings.Builder
	result.Grow(len(normalized))
	inString := false
	for i := 0; i < len(normalized); i++ {
		if normalized[i] == '\'' {
			inString = !inString
			// Write the quote itself — we only strip the content between them.
			result.WriteByte('\'')
			continue
		}
		if !inString {
			result.WriteByte(normalized[i])
		}
	}

	// Step 3: If we ended inside a string, reject conservatively.
	if inString {
		return "", fmt.Errorf("unterminated string literal")
	}

	return result.String(), nil
}

// ValidateQuery checks whether a SQL statement is safe for read-only execution.
func ValidateQuery(sqlStr string) GateResult {
	stripped := strings.TrimSpace(sqlStr)
	if !strings.HasPrefix(strings.ToUpper(stripped), "SELECT") {
		return GateResult{Allowed: false, Reason: "Only SELECT statements are permitted"}
	}

	for _, pattern := range forbiddenPatterns {
		if loc := pattern.FindString(stripped); loc != "" {
			// Allow whitelisted PRAGMA statements
			if strings.EqualFold(loc, "PRAGMA") {
				lowerSQL := strings.ToLower(stripped)
				whitelisted := false
				for _, allowed := range allowedPragmas {
					if strings.Contains(lowerSQL, allowed) {
						whitelisted = true
						break
					}
				}
				if whitelisted {
					continue
				}
			}
			return GateResult{
				Allowed: false,
				Reason:  fmt.Sprintf("Forbidden pattern: %s", loc),
			}
		}
	}

	// Semicolon guard: strip string literals first so that semicolons inside
	// SQL values like '%key;value%' pass, then check for multi-statement injection.
	literalStripped, err := stripStringLiterals(stripped)
	if err != nil {
		return GateResult{Allowed: false, Reason: fmt.Sprintf("Invalid query: %s", err)}
	}
	if loc := semicolonGuard.FindString(literalStripped); loc != "" {
		return GateResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Forbidden pattern: %s", loc),
		}
	}

	return GateResult{Allowed: true}
}

// ExecuteGatedQuery runs a validated read-only query capped at MaxRows.
func ExecuteGatedQuery(db *sql.DB, query string, params ...any) ([]map[string]interface{}, error) {
	gate := ValidateQuery(query)
	if !gate.Allowed {
		return nil, fmt.Errorf("query rejected: %s", gate.Reason)
	}

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() && len(results) < MaxRows {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
