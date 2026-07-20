package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func FlattenJSONB(columns []string, rows [][]string) ([]string, [][]string) {
	jsonbCols := make(map[int]bool)
	for colIdx := range columns {
		for _, row := range rows {
			if colIdx < len(row) && isJSONValue(row[colIdx]) {
				jsonbCols[colIdx] = true
				break
			}
		}
	}
	if len(jsonbCols) == 0 {
		return columns, rows
	}

	newHeaders, _ := buildExpandedHeaders(columns, rows, jsonbCols)

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
			if colIdx < len(row) && jsonbCols[colIdx] && row[colIdx] != "" {
				for k, v := range expandJSONObject(col, row[colIdx]) {
					expandedVals[k] = v
				}
			} else if colIdx < len(row) {
				expandedVals[col] = row[colIdx]
			}
		}

		for k, v := range expandedVals {
			if idx, ok := headerToIdx[k]; ok {
				expanded[rowIdx][idx] = v
			}
		}
	}

	return newHeaders, expanded
}

func isJSONValue(val string) bool {
	v := strings.TrimSpace(val)
	if len(v) == 0 {
		return false
	}
	return v[0] == '{' || v[0] == '['
}

func buildExpandedHeaders(columns []string, rows [][]string, jsonbCols map[int]bool) ([]string, map[int][]string) {
	colExpansions := make(map[int][]string)
	for colIdx := range columns {
		if jsonbCols[colIdx] {
			keySet := make(map[string]bool)
			for _, row := range rows {
				if colIdx >= len(row) || row[colIdx] == "" {
					continue
				}
				expanded := expandJSONObject(columns[colIdx], row[colIdx])
				for k := range expanded {
					keySet[k] = true
				}
			}
			colExpansions[colIdx] = sortedKeys(keySet)
		}
	}

	var newHeaders []string
	for colIdx, col := range columns {
		if keys, ok := colExpansions[colIdx]; ok {
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

func expandJSONObject(prefix, raw string) map[string]string {
	result := make(map[string]string)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return result
	}
	expandRecursive(prefix, obj, result)
	return result
}

func expandRecursive(prefix string, obj map[string]interface{}, result map[string]string) {
	for k, v := range obj {
		key := prefix + "_" + k
		switch val := v.(type) {
		case string:
			result[key] = val
		case float64:
			result[key] = fmt.Sprintf("%v", val)
		case bool:
			if val {
				result[key] = "true"
			} else {
				result[key] = "false"
			}
		case map[string]interface{}:
			expandRecursive(key, val, result)
		default:
			if b, err := json.Marshal(val); err == nil {
				result[key] = string(b)
			}
		}
	}
}
