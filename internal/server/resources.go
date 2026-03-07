package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (a *App) registerResources() {
	a.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"schema:///{database}/tables",
			"Database Tables",
			mcp.WithTemplateDescription("List all tables in a database with column counts"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		a.handleResourceTables,
	)

	a.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"schema:///{database}/{schema}/{table}",
			"Table Detail",
			mcp.WithTemplateDescription("Full table detail including columns, constraints, indexes, and foreign keys"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		a.handleResourceTableDetail,
	)
}

func (a *App) handleResourceTables(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	dbName, ok := request.Params.Arguments["database"].(string)
	if !ok || dbName == "" {
		return nil, fmt.Errorf("missing required parameter: database")
	}

	schemas, err := a.store.ListSchemas(dbName)
	if err != nil {
		return nil, fmt.Errorf("listing schemas: %w", err)
	}

	type tableEntry struct {
		Schema      string `json:"schema"`
		Table       string `json:"table"`
		Type        string `json:"type"`
		RowEstimate int64  `json:"row_estimate"`
		ColumnCount int    `json:"column_count"`
	}

	entries := make([]tableEntry, 0)
	for _, s := range schemas {
		tables, err := a.store.ListTables(dbName, s.SchemaName)
		if err != nil {
			return nil, fmt.Errorf("listing tables for schema %s: %w", s.SchemaName, err)
		}
		for _, t := range tables {
			cols, _ := a.store.ListColumnsCompact(dbName, s.SchemaName, t.TableName)
			entries = append(entries, tableEntry{
				Schema:      s.SchemaName,
				Table:       t.TableName,
				Type:        t.TableType,
				RowEstimate: t.RowEstimate,
				ColumnCount: len(cols),
			})
		}
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling tables: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (a *App) handleResourceTableDetail(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	dbName, _ := request.Params.Arguments["database"].(string)
	schema, _ := request.Params.Arguments["schema"].(string)
	table, _ := request.Params.Arguments["table"].(string)

	if dbName == "" || schema == "" || table == "" {
		return nil, fmt.Errorf("missing required parameters: database, schema, table")
	}

	detail, err := a.store.DescribeTable(dbName, schema, table)
	if err != nil {
		return nil, fmt.Errorf("describing table: %w", err)
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling table detail: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
