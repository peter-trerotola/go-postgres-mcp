package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"name", "type"}
	rows := [][]string{
		{"id", "integer"},
		{"name", "text"},
	}
	writeTable(&buf, headers, rows)
	out := buf.String()

	if !strings.Contains(out, "name") || !strings.Contains(out, "type") {
		t.Error("expected headers in output")
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "integer") {
		t.Error("expected row data in output")
	}
	if !strings.Contains(out, "---") {
		t.Error("expected separator line")
	}
}

func TestWriteTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, []string{}, nil)
	if buf.Len() != 0 {
		t.Error("expected empty output for no headers")
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	if err := writeJSON(&buf, data); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"key"`) || !strings.Contains(out, `"value"`) {
		t.Error("expected JSON output")
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"col1", "col2"}
	rows := [][]string{{"a", "b"}, {"c", "d"}}
	if err := writeCSV(&buf, headers, rows); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d", len(lines))
	}
	if lines[0] != "col1,col2" {
		t.Errorf("expected header line, got %q", lines[0])
	}
}

func TestWritePlain(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]string{{"a", "b"}, {"c", "d"}}
	writePlain(&buf, rows)
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "a\tb") {
		t.Errorf("expected tab-separated values, got %q", lines[0])
	}
}

func TestFormatOutput_JSON(t *testing.T) {
	var buf bytes.Buffer
	data := []string{"hello"}
	if err := formatOutput(&buf, FormatJSON, nil, nil, data); err != nil {
		t.Fatalf("formatOutput JSON: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Error("expected JSON data")
	}
}

func TestFormatOutput_Table(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"x"}
	rows := [][]string{{"1"}}
	if err := formatOutput(&buf, FormatTable, headers, rows, nil); err != nil {
		t.Fatalf("formatOutput table: %v", err)
	}
	if !strings.Contains(buf.String(), "x") {
		t.Error("expected table header")
	}
}

func TestFormatOutput_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := formatOutput(&buf, "jsno", []string{"x"}, [][]string{{"1"}}, nil)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown output format") {
		t.Errorf("expected descriptive error, got: %v", err)
	}
}

func TestDefaultFormat_NonTTY(t *testing.T) {
	// In tests, stdout is typically not a TTY
	f := defaultFormat()
	if f != FormatPlain {
		t.Logf("defaultFormat=%s (expected 'plain' in non-TTY test context)", f)
	}
}
