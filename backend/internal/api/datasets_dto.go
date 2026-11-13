package api

import (
	"math"
	"strconv"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/dataset"
)

type datasetSourceDTO struct {
	Filename string `json:"filename"`
	Bytes    int64  `json:"bytes"`
}

type datasetSummaryDTO struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	CreatedAt  time.Time        `json:"createdAt"`
	Source     datasetSourceDTO `json:"source"`
	TableCount int              `json:"tableCount"`
	RowCount   int              `json:"rowCount"`
}

type tableSummaryDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RowCount    int    `json:"rowCount"`
	ColumnCount int    `json:"columnCount"`
}

type columnDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullability string `json:"nullability"`
}

type datasetDetailDTO struct {
	datasetSummaryDTO
	Tables []tableSummaryDTO `json:"tables"`
}

type tableDetailDTO struct {
	tableSummaryDTO
	Columns []columnDTO `json:"columns"`
}

type datasetListDTO struct {
	Datasets []datasetDetailDTO `json:"datasets"`
}

type tableListDTO struct {
	Tables []tableSummaryDTO `json:"tables"`
}

// rowsPageDTO carries a bounded page of rows. Numeric cells are JSON strings so
// int64 values survive JavaScript without precision loss; the column type says
// how to interpret them. NULL is JSON null, never an empty string.
type rowsPageDTO struct {
	TableID   string      `json:"tableId"`
	Columns   []columnDTO `json:"columns"`
	Rows      [][]any     `json:"rows"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
	TotalRows int         `json:"totalRows"`
	HasMore   bool        `json:"hasMore"`
}

type previewColumnDTO struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullability string `json:"nullability"`
	NullCount   int    `json:"nullCount"`
}

type previewDTO struct {
	TableName  string             `json:"tableName"`
	Columns    []previewColumnDTO `json:"columns"`
	Rows       [][]any            `json:"rows"`
	RowCount   int                `json:"rowCount"`
	ByteCount  int64              `json:"byteCount"`
	RowLimit   int                `json:"rowLimit"`
	Warnings   []string           `json:"warnings"`
	NullValues []string           `json:"nullValues"`
}

func summarizeDataset(entry *dataset.Dataset) datasetSummaryDTO {
	return datasetSummaryDTO{
		ID:        string(entry.ID),
		Name:      entry.Name,
		CreatedAt: entry.CreatedAt,
		Source: datasetSourceDTO{
			Filename: entry.Source.Filename,
			Bytes:    entry.Source.Bytes,
		},
		TableCount: entry.Tables.Len(),
		RowCount:   entry.RowCount(),
	}
}

func summarizeTables(entry *dataset.Dataset) []tableSummaryDTO {
	tables := entry.Tables.List()
	summaries := make([]tableSummaryDTO, len(tables))
	for i, table := range tables {
		summaries[i] = summarizeTable(table)
	}
	return summaries
}

func summarizeTable(table *catalog.Table) tableSummaryDTO {
	return tableSummaryDTO{
		ID:          string(table.ID),
		Name:        table.Name,
		RowCount:    table.RowCount(),
		ColumnCount: table.ColumnCount(),
	}
}

func detailDataset(entry *dataset.Dataset) datasetDetailDTO {
	return datasetDetailDTO{
		datasetSummaryDTO: summarizeDataset(entry),
		Tables:            summarizeTables(entry),
	}
}

func detailTable(table *catalog.Table) tableDetailDTO {
	return tableDetailDTO{
		tableSummaryDTO: summarizeTable(table),
		Columns:         columnsOf(table),
	}
}

func columnsOf(table *catalog.Table) []columnDTO {
	columns := table.Schema.Columns()
	out := make([]columnDTO, len(columns))
	for i, column := range columns {
		out[i] = columnDTO{
			ID:          string(column.ID),
			Name:        column.Name,
			Type:        column.Type.String(),
			Nullability: string(column.Nullability),
		}
	}
	return out
}

func rowsOf(rows []catalog.Row) [][]any {
	out := make([][]any, len(rows))
	for i, row := range rows {
		cells := make([]any, len(row))
		for j, value := range row {
			cells[j] = cellOf(value)
		}
		out[i] = cells
	}
	return out
}

func cellOf(value catalog.Value) any {
	switch value.Type() {
	case catalog.TypeBoolean:
		boolean, _ := value.AsBool()
		return boolean
	case catalog.TypeInteger:
		integer, _ := value.AsInt()
		return strconv.FormatInt(integer, 10)
	case catalog.TypeDecimal:
		decimal, _ := value.AsDecimal()
		return formatDecimalCell(decimal)
	case catalog.TypeText:
		text, _ := value.AsText()
		return text
	default:
		return nil
	}
}

func formatDecimalCell(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "Infinity"
	case math.IsInf(value, -1):
		return "-Infinity"
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

func previewOf(parsed *dataset.Parsed) previewDTO {
	columns := make([]previewColumnDTO, len(parsed.Columns))
	for i, column := range parsed.Columns {
		columns[i] = previewColumnDTO{
			Name:        column.Name,
			Type:        column.Type.String(),
			Nullability: string(column.Nullability),
			NullCount:   column.NullCount,
		}
	}

	rowCount := parsed.Table.RowCount()
	limit := previewRowLimit
	if rowCount < limit {
		limit = rowCount
	}

	warnings := parsed.Warnings
	if warnings == nil {
		warnings = []string{}
	}

	return previewDTO{
		TableName:  parsed.Table.Name,
		Columns:    columns,
		Rows:       rowsOf(parsed.Table.Rows[:limit]),
		RowCount:   rowCount,
		ByteCount:  parsed.Bytes,
		RowLimit:   previewRowLimit,
		Warnings:   warnings,
		NullValues: parsed.NullValues,
	}
}
