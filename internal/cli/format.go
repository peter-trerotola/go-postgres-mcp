package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Format constants for output formatting.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatCSV   = "csv"
	FormatPlain = "plain"
)

// defaultFormat returns the default output format based on whether stdout is a TTY.
func defaultFormat() string {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return FormatPlain
	}
	if fi.Mode()&os.ModeCharDevice != 0 {
		return FormatTable
	}
	return FormatPlain
}

// writeTable renders headers and rows as a psql-style aligned table.
func writeTable(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)

	// Header
	fmt.Fprintln(tw, " "+strings.Join(headers, "\t| "))

	// Separator
	seps := make([]string, len(headers))
	for i, h := range headers {
		seps[i] = strings.Repeat("-", len(h)+1)
	}
	fmt.Fprintln(tw, strings.Join(seps, "\t+-"))

	// Rows
	for _, row := range rows {
		fmt.Fprintln(tw, " "+strings.Join(row, "\t| "))
	}
	tw.Flush()
}

// writeJSON renders data as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// writeCSV renders headers and rows as CSV.
func writeCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writePlain renders rows as tab-separated values without headers.
func writePlain(w io.Writer, rows [][]string) {
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
}

// formatOutput is a convenience that dispatches to the right formatter.
func formatOutput(w io.Writer, format string, headers []string, rows [][]string, jsonData any) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, jsonData)
	case FormatCSV:
		return writeCSV(w, headers, rows)
	case FormatPlain:
		writePlain(w, rows)
		return nil
	case FormatTable:
		writeTable(w, headers, rows)
		return nil
	default:
		return fmt.Errorf("unknown output format %q (valid: table, json, csv, plain)", format)
	}
}
