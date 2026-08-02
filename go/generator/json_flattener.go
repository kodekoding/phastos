package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func FlattenJSONB(columns []string, rows [][]string) ([]string, [][]string) {
	jsonbCols := make(map[int]bool)
	for colIdx := range columns {
		for _, row := range rows {
			if colIdx < len(row) && isJSONObjectOrArray(row[colIdx]) {
				jsonbCols[colIdx] = true
				break
			}
		}
	}
	if len(jsonbCols) == 0 {
		return columns, rows
	}

	newHeaders, colExpansions := buildExpandedHeaders(columns, rows, jsonbCols)

	// A column detected as JSONB that expands to zero keys (e.g. every row is
	// `{}` or `[]`) is kept as a plain column with its raw value instead of
	// being silently dropped.
	plainFallback := make(map[int]bool)
	for colIdx := range jsonbCols {
		if len(colExpansions[colIdx]) == 0 {
			plainFallback[colIdx] = true
		}
	}

	expanded := make([][]string, len(rows))
	for rowIdx, row := range rows {
		expanded[rowIdx] = make([]string, len(newHeaders))
		headerToIdx := make(map[string]int)
		for hIdx, h := range newHeaders {
			headerToIdx[h] = hIdx
		}

		expandedVals := make(map[string]string)
		for colIdx := range columns {
			col := columns[colIdx]
			if colIdx >= len(row) {
				continue
			}
			if jsonbCols[colIdx] && !plainFallback[colIdx] && row[colIdx] != "" {
				for k, v := range expandJSONValue(col, row[colIdx]) {
					expandedVals[k] = v
				}
				continue
			}
			expandedVals[col] = row[colIdx]
		}

		for k, v := range expandedVals {
			if idx, ok := headerToIdx[k]; ok {
				expanded[rowIdx][idx] = v
			}
		}
	}

	return newHeaders, expanded
}

// isJSONObjectOrArray reports whether the value parses as a JSON object or
// array. Values that merely start with `{`/`[` but are not valid JSON (e.g. a
// Go struct dump like "{EMP001 true}") are treated as plain text.
func isJSONObjectOrArray(val string) bool {
	v := strings.TrimSpace(val)
	if len(v) == 0 {
		return false
	}
	if v[0] != '{' && v[0] != '[' {
		return false
	}
	return json.Valid([]byte(v))
}

func buildExpandedHeaders(columns []string, rows [][]string, jsonbCols map[int]bool) ([]string, map[int][]string) {
	colExpansions := make(map[int][]string)
	for colIdx := range columns {
		if !jsonbCols[colIdx] {
			continue
		}
		keySet := make(map[string]bool)
		for _, row := range rows {
			if colIdx >= len(row) || row[colIdx] == "" {
				continue
			}
			for k := range expandJSONValue(columns[colIdx], row[colIdx]) {
				keySet[k] = true
			}
		}
		colExpansions[colIdx] = sortedKeys(keySet)
	}

	var newHeaders []string
	for colIdx, col := range columns {
		if keys, ok := colExpansions[colIdx]; ok && len(keys) > 0 {
			newHeaders = append(newHeaders, keys...)
		} else {
			newHeaders = append(newHeaders, col)
		}
	}
	return newHeaders, colExpansions
}

func sortedKeys(s map[string]bool) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// expandJSONValue flattens a JSON object or array into `prefix_key` /
// `prefix_<index>` columns. Unparseable values yield no expansion.
func expandJSONValue(prefix, raw string) map[string]string {
	result := make(map[string]string)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		expandRecursive(prefix, obj, result)
		return result
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		expandArray(prefix, arr, result)
	}
	return result
}

func expandRecursive(prefix string, obj map[string]interface{}, result map[string]string) {
	for k, v := range obj {
		key := prefix + "_" + k
		switch val := v.(type) {
		case nil:
			result[key] = ""
		case string:
			result[key] = val
		case float64:
			result[key] = fmt.Sprintf("%v", val)
		case bool:
			result[key] = strconv.FormatBool(val)
		case map[string]interface{}:
			expandRecursive(key, val, result)
		case []interface{}:
			expandArray(key, val, result)
		}
	}
}

func expandArray(prefix string, arr []interface{}, result map[string]string) {
	for i, v := range arr {
		key := fmt.Sprintf("%s_%d", prefix, i)
		switch val := v.(type) {
		case nil:
			result[key] = ""
		case string:
			result[key] = val
		case float64:
			result[key] = fmt.Sprintf("%v", val)
		case bool:
			result[key] = strconv.FormatBool(val)
		case map[string]interface{}:
			expandRecursive(key, val, result)
		case []interface{}:
			expandArray(key, val, result)
		}
	}
}
