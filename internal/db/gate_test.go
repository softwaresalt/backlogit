package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/backlogit/backlogit/internal/db"
)

func TestValidateQuery_AllowsSelect(t *testing.T) {
	tests := []string{
		"SELECT * FROM items",
		"SELECT id, title FROM items WHERE status = ?",
		"SELECT COUNT(*) FROM items",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			result := db.ValidateQuery(sql)
			assert.True(t, result.Allowed, "expected allowed for: %s", sql)
		})
	}
}

func TestValidateQuery_RejectsForbidden(t *testing.T) {
	tests := []struct {
		sql     string
		keyword string
	}{
		{"SELECT * FROM items; DROP TABLE items", "DROP"},
		{"SELECT * FROM items; DELETE FROM items", "DELETE"},
		{"SELECT * FROM items; INSERT INTO items VALUES ('X')", "INSERT"},
		{"SELECT * FROM items; UPDATE items SET status='done'", "UPDATE"},
		{"SELECT * FROM items; ATTACH DATABASE 'other.db' AS other", "ATTACH"},
	}
	for _, tc := range tests {
		t.Run(tc.keyword, func(t *testing.T) {
			result := db.ValidateQuery(tc.sql)
			assert.False(t, result.Allowed)
		})
	}
}

func TestValidateQuery_RejectsNonSelectStatements(t *testing.T) {
	tests := []string{
		"DROP TABLE items",
		"DELETE FROM items",
		"INSERT INTO items VALUES ('X')",
		"UPDATE items SET status='done'",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			result := db.ValidateQuery(sql)
			assert.False(t, result.Allowed)
			assert.Equal(t, "Only SELECT statements are permitted", result.Reason)
		})
	}
}

func TestValidateQuery_RejectsNonSelect(t *testing.T) {
	result := db.ValidateQuery("PRAGMA table_info(items)")
	assert.False(t, result.Allowed)
}
