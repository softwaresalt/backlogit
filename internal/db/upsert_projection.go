package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/models"
)

type itemColumnQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type projectedCustomField struct {
	column string
	value  any
}

var baseItemColumnNames = map[string]struct{}{
	"id":             {},
	"title":          {},
	"status":         {},
	"artifact_type":  {},
	"parent_id":      {},
	"sprint":         {},
	"priority":       {},
	"description":    {},
	"custom_fields":  {},
	"created_at":     {},
	"updated_at":     {},
	"assigned_to":    {},
	"owner":          {},
	"labels":         {},
	"dependencies":   {},
	"references":     {},
	"commit":         {},
	"level":          {},
	"hierarchy_path": {},
}

func buildUpsertItemStatement(
	ctx context.Context,
	q itemColumnQuerier,
	artifact *models.Artifact,
	customFields string,
	labelsVal, depsVal, refsVal sql.NullString,
) (string, []any, error) {
	columns := []string{
		"id",
		"title",
		"status",
		"artifact_type",
		"parent_id",
		"sprint",
		"priority",
		"description",
		"custom_fields",
		"created_at",
		"updated_at",
		"assigned_to",
		"owner",
		"labels",
		"dependencies",
		`"references"`,
		`"commit"`,
		"level",
		"hierarchy_path",
	}
	args := []any{
		artifact.ID,
		artifact.Title,
		string(artifact.Status),
		artifact.ArtifactType,
		nullString(artifact.ParentID),
		nullString(artifact.Sprint),
		nullString(artifact.Priority),
		nullString(artifact.Description),
		customFields,
		artifact.CreatedAt.Format(time.RFC3339Nano),
		artifact.UpdatedAt.Format(time.RFC3339Nano),
		nullString(artifact.AssignedTo),
		nullString(artifact.Owner),
		labelsVal,
		depsVal,
		refsVal,
		nullString(artifact.Commit),
		nullInt64(artifact.Level),
		nullString(artifact.HierarchyPath),
	}

	projected, err := projectedCustomFields(ctx, q, artifact)
	if err != nil {
		return "", nil, err
	}
	for _, field := range projected {
		columns = append(columns, quoteIdentifier(field.column))
		args = append(args, field.value)
	}

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	stmt := `INSERT OR REPLACE INTO items (` + strings.Join(columns, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
	return stmt, args, nil
}

func projectedCustomFields(ctx context.Context, q itemColumnQuerier, artifact *models.Artifact) ([]projectedCustomField, error) {
	if len(artifact.CustomFields) == 0 {
		return nil, nil
	}
	columns, err := existingColumnsFromQuerier(ctx, q)
	if err != nil {
		return nil, err
	}

	projected := make([]projectedCustomField, 0, len(artifact.CustomFields))
	for name, value := range artifact.CustomFields {
		if name == "complexity" && artifact.ArtifactType != "task" {
			continue
		}
		if _, base := baseItemColumnNames[name]; base {
			continue
		}
		if !columns[name] {
			continue
		}
		if err := ValidateColumnName(name); err != nil {
			return nil, err
		}
		projectedValue, err := customFieldProjectionValue(value)
		if err != nil {
			return nil, fmt.Errorf("project custom field %q: %w", name, err)
		}
		projected = append(projected, projectedCustomField{column: name, value: projectedValue})
	}
	sort.Slice(projected, func(i, j int) bool {
		return projected[i].column < projected[j].column
	})
	return projected, nil
}

func existingColumnsFromQuerier(ctx context.Context, q itemColumnQuerier) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, "PRAGMA table_info(items)")
	if err != nil {
		return nil, fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info: %w", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table_info columns: %w", err)
	}
	return cols, nil
}

func customFieldProjectionValue(value any) (any, error) {
	switch v := value.(type) {
	case nil:
		return sql.NullString{}, nil
	case string:
		return nullString(v), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return v, nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal projection value: %w", err)
		}
		if string(encoded) == "null" {
			return sql.NullString{}, nil
		}
		return nullString(string(encoded)), nil
	}
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
