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
	regexp.MustCompile(`(?m);.*\S+.*$`),
}

// allowedPragmas lists PRAGMA statements that pass the read-only gate.
var allowedPragmas = []string{"table_info", "table_list", "database_list"}

// MaxRows caps the number of rows returned by a gated query.
const MaxRows = 500

// GateResult reports whether a SQL statement passed the read-only gate.
type GateResult struct {
	Allowed bool
	Reason  string
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
	return GateResult{Allowed: true}
}

// ExecuteGatedQuery runs a validated read-only query capped at MaxRows.
//
// Worker: Implement gated query execution with row scanning.
func ExecuteGatedQuery(db *sql.DB, query string, params ...any) ([]map[string]interface{}, error) {
	panic("not implemented: Worker: Implement gated query execution")
}
