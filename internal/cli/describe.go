package cli

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
	"github.com/spf13/cobra"
)

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func newDescribeCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "describe [database] <table>",
		Short: "Describe a table's columns, constraints, indexes, and foreign keys",
		Long:  "Show full detail for a table. Accepts 'schema.table' notation or use -s for schema.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var db, table string

			if len(args) == 2 {
				db = args[0]
				table = args[1]
			} else {
				var err error
				db, err = resolveDB(nil)
				if err != nil {
					return err
				}
				table = args[0]
			}

			// Support schema.table notation
			if parts := strings.SplitN(table, ".", 2); len(parts) == 2 {
				schema = parts[0]
				table = parts[1]
			}

			detail, err := eng.DescribeTable(db, schema, table)
			if err != nil {
				return err
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			if format == FormatJSON {
				return writeJSON(w, detail)
			}

			renderTableDetail(w, detail)
			return nil
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "schema name")
	return cmd
}

// renderTableDetail writes a human-readable description of a table to w.
func renderTableDetail(w io.Writer, detail *knowledgemap.TableDetail) {
	fmt.Fprintf(w, "Table: %s.%s\n", detail.Table.SchemaName, detail.Table.TableName)
	if detail.Table.TableType != "" {
		fmt.Fprintf(w, "Type: %s\n", detail.Table.TableType)
	}
	fmt.Fprintln(w)

	renderColumns(w, detail.Columns)
	renderConstraints(w, detail.Constraints)
	renderIndexes(w, detail.Indexes)
	renderForeignKeys(w, detail.ForeignKeys)
}

func renderColumns(w io.Writer, columns []knowledgemap.ColumnRow) {
	if len(columns) == 0 {
		return
	}
	fmt.Fprintln(w, "Columns:")
	headers := []string{"#", "column", "type", "nullable", "default"}
	rows := make([][]string, len(columns))
	for i, c := range columns {
		nullable := "NO"
		if c.IsNullable {
			nullable = "YES"
		}
		rows[i] = []string{
			fmt.Sprintf("%d", c.Ordinal),
			c.ColumnName, c.DataType, nullable,
			nullStr(c.ColumnDefault),
		}
	}
	writeTable(w, headers, rows)
	fmt.Fprintln(w)
}

func renderConstraints(w io.Writer, constraints []knowledgemap.ConstraintRow) {
	if len(constraints) == 0 {
		return
	}
	fmt.Fprintln(w, "Constraints:")
	headers := []string{"name", "type", "definition"}
	rows := make([][]string, len(constraints))
	for i, c := range constraints {
		rows[i] = []string{c.ConstraintName, c.ConstraintType, nullStr(c.Definition)}
	}
	writeTable(w, headers, rows)
	fmt.Fprintln(w)
}

func renderIndexes(w io.Writer, indexes []knowledgemap.IndexRow) {
	if len(indexes) == 0 {
		return
	}
	fmt.Fprintln(w, "Indexes:")
	headers := []string{"name", "unique", "primary", "definition"}
	rows := make([][]string, len(indexes))
	for i, idx := range indexes {
		rows[i] = []string{
			idx.IndexName,
			fmt.Sprintf("%v", idx.IsUnique),
			fmt.Sprintf("%v", idx.IsPrimary),
			nullStr(idx.Definition),
		}
	}
	writeTable(w, headers, rows)
	fmt.Fprintln(w)
}

func renderForeignKeys(w io.Writer, fks []knowledgemap.ForeignKeyRow) {
	if len(fks) == 0 {
		return
	}
	fmt.Fprintln(w, "Foreign Keys:")
	headers := []string{"name", "column", "references"}
	rows := make([][]string, len(fks))
	for i, fk := range fks {
		ref := fmt.Sprintf("%s.%s(%s)", fk.RefSchema, fk.RefTable, fk.RefColumn)
		rows[i] = []string{fk.ConstraintName, fk.ColumnName, ref}
	}
	writeTable(w, headers, rows)
}
