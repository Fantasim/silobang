package queries

import (
	"database/sql"
	"fmt"
	"regexp"
)

// QueryResult contains the result of a query execution
type QueryResult struct {
	Preset   string          `json:"preset"`
	RowCount int             `json:"row_count"`
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
}

// QueryRequest contains parameters for executing a query
type QueryRequest struct {
	Params map[string]interface{} `json:"params"`
	Topics []string               `json:"topics"` // If empty, query all healthy topics
}

// ParamsToStrings converts interface{} param values to strings
func ParamsToStrings(params map[string]interface{}) map[string]string {
	if params == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range params {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// paramRegex matches :paramName patterns in SQL
var paramRegex = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

// ValidateParams validates and fills in default values for query parameters
// Returns the final params map with defaults applied
func ValidateParams(preset *Preset, params map[string]string) (map[string]string, error) {
	if params == nil {
		params = make(map[string]string)
	}

	result := make(map[string]string)

	for _, p := range preset.Params {
		value, provided := params[p.Name]

		if !provided || value == "" {
			if p.Required {
				return nil, fmt.Errorf("required parameter missing: %s", p.Name)
			}
			if p.Default != "" {
				result[p.Name] = p.Default
			}
		} else {
			result[p.Name] = value
		}
	}

	return result, nil
}

// BuildQuery converts named parameters to positional parameters for SQLite
// Returns the query with ? placeholders and the ordered argument slice
func BuildQuery(sqlTemplate string, params map[string]string) (string, []interface{}) {
	var args []interface{}
	paramIndex := make(map[string]int) // Track which params we've seen
	argCounter := 0

	result := paramRegex.ReplaceAllStringFunc(sqlTemplate, func(match string) string {
		paramName := match[1:] // Remove leading :

		if _, seen := paramIndex[paramName]; !seen {
			paramIndex[paramName] = argCounter
			if value, exists := params[paramName]; exists {
				args = append(args, value)
			} else {
				args = append(args, nil)
			}
			argCounter++
		}

		return "?"
	})

	return result, args
}

// ExecuteQuery runs a query and returns columns and rows
func ExecuteQuery(db *sql.DB, query string, args []interface{}) ([]string, [][]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Prepare result slice
	var result [][]interface{}

	// Scan rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert []byte to string for JSON serialization
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}

		result = append(result, values)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("row iteration error: %w", err)
	}

	return columns, result, nil
}

// ExecutePresetQuery executes a preset query against a single topic database
// Adds _topic column to results
func ExecutePresetQuery(preset *Preset, params map[string]string, db *sql.DB, topicName string) ([]string, [][]interface{}, error) {
	// Build query with parameters
	query, args := BuildQuery(preset.SQL, params)

	// Execute query
	columns, rows, err := ExecuteQuery(db, query, args)
	if err != nil {
		return nil, nil, err
	}

	// Add _topic column
	columns = append(columns, "_topic")
	for i := range rows {
		rows[i] = append(rows[i], topicName)
	}

	return columns, rows, nil
}

// ExecuteCrossTopicQuery executes a preset query across multiple topics
// Results are interleaved (not grouped by topic)
func ExecuteCrossTopicQuery(preset *Preset, params map[string]string, topicDBs map[string]*sql.DB, topicNames []string) (*QueryResult, error) {
	var allColumns []string
	var allRows [][]interface{}

	for _, topicName := range topicNames {
		db, exists := topicDBs[topicName]
		if !exists {
			continue
		}

		columns, rows, err := ExecutePresetQuery(preset, params, db, topicName)
		if err != nil {
			// Log error but continue with other topics
			continue
		}

		// Set columns from first successful query
		if allColumns == nil {
			allColumns = columns
		}

		// Append rows (interleaved)
		allRows = append(allRows, rows...)
	}

	if allColumns == nil {
		allColumns = []string{}
	}
	if allRows == nil {
		allRows = [][]interface{}{}
	}

	return &QueryResult{
		RowCount: len(allRows),
		Columns:  allColumns,
		Rows:     allRows,
	}, nil
}
