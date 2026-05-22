package importer

import (
	"strings"
	"sync"
	"testing"
)

// TestProcessedResultPoolReuse verifies that getProcessedResult returns a usable
// zeroed struct and that putProcessedResult properly resets fields so the next
// get does not observe stale data.
func TestProcessedResultPoolReuse(t *testing.T) {
	pr := getProcessedResult()
	pr.ParsedStruct = "stale-struct"
	pr.RawData = map[string]any{"k": "v"}

	putProcessedResult(pr)

	pr2 := getProcessedResult()
	if pr2.ParsedStruct != nil {
		t.Errorf("expected ParsedStruct to be nil after pool reuse, got %v", pr2.ParsedStruct)
	}
	if pr2.RawData != nil {
		t.Errorf("expected RawData to be nil after pool reuse, got %v", pr2.RawData)
	}
	if pr2.Error != nil {
		t.Errorf("expected Error to be nil after pool reuse, got %v", pr2.Error)
	}
	putProcessedResult(pr2)
}

// TestProcessedResultPoolConcurrency stresses the processedResult pool from
// multiple goroutines to ensure there are no data races.
func TestProcessedResultPoolConcurrency(t *testing.T) {
	const workers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pr := getProcessedResult()
				pr.ParsedStruct = id
				pr.RawData = map[string]any{"worker": id, "iter": j}
				putProcessedResult(pr)
			}
		}(i)
	}

	wg.Wait()
}

// TestBuilderPoolReuse verifies that pooled strings.Builder instances are
// properly reset between uses.
func TestBuilderPoolReuse(t *testing.T) {
	b := getBuilder()
	b.WriteString("previous-content")
	putBuilder(b)

	b2 := getBuilder()
	if b2.String() != "" {
		t.Errorf("expected empty builder after reset, got %q", b2.String())
	}
	b2.WriteString("new-content")
	if b2.String() != "new-content" {
		t.Errorf("expected 'new-content', got %q", b2.String())
	}
	putBuilder(b2)
}

// TestBuilderPoolConcurrency stresses the strings.Builder pool from multiple
// goroutines to verify there are no data races.
func TestBuilderPoolConcurrency(t *testing.T) {
	const workers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				b := getBuilder()
				b.WriteString("worker-")
				b.WriteString(string(rune('0' + id%10)))
				_ = b.String()
				putBuilder(b)
			}
		}(i)
	}

	wg.Wait()
}

// TestBuildRowMapCorrectness ensures buildRowMap behaves correctly with
// headers longer than, equal to, and shorter than the row slice.
func TestBuildRowMapCorrectness(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		row      []string
		expected map[string]any
	}{
		{
			name:    "normal row",
			headers: []string{"Name", "Email", "Role"},
			row:     []string{"Alice", "alice@example.com", "Admin"},
			expected: map[string]any{
				"Name":  "Alice",
				"Email": "alice@example.com",
				"Role":  "Admin",
			},
		},
		{
			name:    "row shorter than headers",
			headers: []string{"A", "B", "C"},
			row:     []string{"1", "2"},
			expected: map[string]any{
				"A": "1",
				"B": "2",
				"C": "",
			},
		},
		{
			name:    "row longer than headers",
			headers: []string{"A"},
			row:     []string{"1", "2", "3"},
			expected: map[string]any{
				"A": "1",
			},
		},
		{
			name:    "empty row",
			headers: []string{"A", "B"},
			row:     []string{},
			expected: map[string]any{
				"A": "",
				"B": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRowMap(tt.headers, tt.row)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %q=%q, got %q=%q", k, v, k, result[k])
				}
			}
		})
	}
}

const (
	testEmployeeID   = "444201123"
	testEmployeeName = "Fitri"
	testDate1        = "2024-03-26"
	testDate2        = "2024-03-27"
	testShiftValue   = "HONS"
)

// TestBuildPivotEntriesCorrectness verifies that buildPivotEntries produces
// the expected composite keys and values even when using pooled builders.
func TestBuildPivotEntriesCorrectness(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"Employee ID", "Employee Name", testDate1, testDate2}
	row := []string{testEmployeeID, testEmployeeName, testShiftValue, testShiftValue}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		ValueStartCol: 2,
		KeySeparator:  ";",
	}

	buildPivotEntries(data, headers, row, config)

	expected := map[string]string{
		testEmployeeID + ";" + testDate1: testShiftValue,
		testEmployeeID + ";" + testDate2: testShiftValue,
	}

	if len(data) != len(expected) {
		t.Errorf("expected %d entries, got %d", len(expected), len(data))
	}
	for k, v := range expected {
		if data[k] != v {
			t.Errorf("expected %q=%q, got %q=%q", k, v, k, data[k])
		}
	}
}

// TestBuildPivotEntriesWithMultipleKeyColumns verifies composite keys built
// from multiple key columns using the pooled builder.
func TestBuildPivotEntriesWithMultipleKeyColumns(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"Loc", "Emp", "2024-01-01"}
	row := []string{"JKT", "123", "OFF"}
	config := pivotReadConfig{
		KeyColumns:    []int{0, 1},
		ValueStartCol: 2,
		KeySeparator:  "|",
	}

	buildPivotEntries(data, headers, row, config)

	expectedKey := "JKT|123|2024-01-01"
	if data[expectedKey] != "OFF" {
		t.Errorf("expected %q=OFF, got %q=%q", expectedKey, expectedKey, data[expectedKey])
	}
}

// TestBuildPivotEntriesEmptyValueColumns ensures no panic or incorrect entries
// when value columns are empty.
func TestBuildPivotEntriesEmptyValueColumns(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"A", ""}
	row := []string{"1", "x"}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		ValueStartCol: 1,
		KeySeparator:  "-",
	}

	buildPivotEntries(data, headers, row, config)

	// Empty header should be skipped, so data should remain empty.
	if len(data) != 0 {
		t.Errorf("expected 0 entries when value header is empty, got %d", len(data))
	}
}

// TestBuildPivotEntriesWithOnEntry verifies that the OnEntry callback receives
// correct composite keys and values without populating the map.
func TestBuildPivotEntriesWithOnEntry(t *testing.T) {
	var collected []struct {
		key   string
		value string
	}

	data := make(map[string]string)
	headers := []string{"ID", testDate1}
	row := []string{"99", testShiftValue}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		ValueStartCol: 1,
		KeySeparator:  ";",
		OnEntry: func(key, value string) {
			collected = append(collected, struct {
				key   string
				value string
			}{key: key, value: value})
		},
	}

	buildPivotEntries(data, headers, row, config)

	if len(data) != 0 {
		t.Errorf("expected data map to be empty when OnEntry is set, got %d entries", len(data))
	}
	if len(collected) != 1 {
		t.Fatalf("expected 1 callback entry, got %d", len(collected))
	}
	if collected[0].key != "99;2024-03-26" {
		t.Errorf("expected key '99;2024-03-26', got %q", collected[0].key)
	}
	if collected[0].value != "HONS" {
		t.Errorf("expected value 'HONS', got %q", collected[0].value)
	}
}

// TestBuildPivotEntriesWhitespaceTrimming ensures spaces around cells and
// headers are trimmed when constructing keys and values.
func TestBuildPivotEntriesWhitespaceTrimming(t *testing.T) {
	data := make(map[string]string)
	headers := []string{" ID ", " 2024-03-26 "}
	row := []string{" 99 ", " HONS "}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		ValueStartCol: 1,
		KeySeparator:  ";",
	}

	buildPivotEntries(data, headers, row, config)

	expectedKey := "99;2024-03-26"
	if data[expectedKey] != "HONS" {
		t.Errorf("expected %q=HONS, got %q=%q", expectedKey, expectedKey, data[expectedKey])
	}
}

// BenchmarkProcessedResultPool compares pooled allocation against direct
// allocation to demonstrate the optimization benefit.
func BenchmarkProcessedResultPool(b *testing.B) {
	b.Run("pooled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pr := getProcessedResult()
			pr.ParsedStruct = i
			pr.RawData = map[string]any{"i": i}
			putProcessedResult(pr)
		}
	})

	b.Run("direct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pr := &processedResult{}
			pr.ParsedStruct = i
			pr.RawData = map[string]any{"i": i}
			_ = pr
		}
	})
}

// BenchmarkBuilderPool compares pooled strings.Builder against direct
// allocation when building pivot keys.
func BenchmarkBuilderPool(b *testing.B) {
	prefix := "444201123"
	sep := ";"
	header := "2024-03-26"

	b.Run("pooled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := getBuilder()
			sb.WriteString(prefix)
			sb.WriteString(sep)
			sb.WriteString(header)
			_ = sb.String()
			putBuilder(sb)
		}
	})

	b.Run("direct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var sb strings.Builder
			sb.WriteString(prefix)
			sb.WriteString(sep)
			sb.WriteString(header)
			_ = sb.String()
		}
	})
}
