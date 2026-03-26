package cli

import (
	"database/sql"
	"fmt"
	"strings"

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

			// Render table info
			fmt.Fprintf(w, "Table: %s.%s\n", detail.Table.SchemaName, detail.Table.TableName)
			if detail.Table.TableType != "" {
				fmt.Fprintf(w, "Type: %s\n", detail.Table.TableType)
			}
			fmt.Fprintln(w)

			// Columns
			if len(detail.Columns) > 0 {
				fmt.Fprintln(w, "Columns:")
				headers := []string{"#", "column", "type", "nullable", "default"}
				rows := make([][]string, len(detail.Columns))
				for i, c := range detail.Columns {
					nullable := "NO"
					if c.IsNullable {
						nullable = "YES"
					}
					rows[i] = []string{
						fmt.Sprintf("%d", c.Ordinal),
						c.ColumnName,
						c.DataType,
						nullable,
						nullStr(c.ColumnDefault),
					}
				}
				writeTable(w, headers, rows)
				fmt.Fprintln(w)
			}

			// Constraints
			if len(detail.Constraints) > 0 {
				fmt.Fprintln(w, "Constraints:")
				headers := []string{"name", "type", "definition"}
				rows := make([][]string, len(detail.Constraints))
				for i, c := range detail.Constraints {
					rows[i] = []string{c.ConstraintName, c.ConstraintType, nullStr(c.Definition)}
				}
				writeTable(w, headers, rows)
				fmt.Fprintln(w)
			}

			// Indexes
			if len(detail.Indexes) > 0 {
				fmt.Fprintln(w, "Indexes:")
				headers := []string{"name", "unique", "primary", "definition"}
				rows := make([][]string, len(detail.Indexes))
				for i, idx := range detail.Indexes {
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

			// Foreign Keys
			if len(detail.ForeignKeys) > 0 {
				fmt.Fprintln(w, "Foreign Keys:")
				headers := []string{"name", "column", "references"}
				rows := make([][]string, len(detail.ForeignKeys))
				for i, fk := range detail.ForeignKeys {
					ref := fmt.Sprintf("%s.%s(%s)", fk.RefSchema, fk.RefTable, fk.RefColumn)
					rows[i] = []string{fk.ConstraintName, fk.ColumnName, ref}
				}
				writeTable(w, headers, rows)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "schema name")
	return cmd
}
