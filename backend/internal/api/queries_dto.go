package api

import (
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/metrics"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/query"
)

type queryRequestDTO struct {
	DatasetID string `json:"datasetId"`
	SQL       string `json:"sql"`
}

// queryColumnDTO describes one result or operator column. Result columns are not
// table columns, so they carry no nullability or table identity.
type queryColumnDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// executionSampleDTO is a bounded copy of the rows entering or leaving one
// operator. Columns describe the rows at that point, which can differ from the
// operator's output schema for a projection.
type executionSampleDTO struct {
	Columns   []queryColumnDTO `json:"columns"`
	Rows      [][]any          `json:"rows"`
	Truncated bool             `json:"truncated"`
}

type executionNodeDTO struct {
	ID         string             `json:"id"`
	Kind       string             `json:"kind"`
	Label      string             `json:"label"`
	ParentID   string             `json:"parentId,omitempty"`
	Children   []string           `json:"children"`
	Columns    []queryColumnDTO   `json:"columns"`
	RowsIn     int64              `json:"rowsIn"`
	RowsOut    int64              `json:"rowsOut"`
	DurationMS float64            `json:"durationMs"`
	Status     string             `json:"status"`
	Error      string             `json:"error,omitempty"`
	Input      executionSampleDTO `json:"input"`
	Output     executionSampleDTO `json:"output"`
}

type executionEdgeDTO struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type executionTraceDTO struct {
	RootID     string             `json:"rootId"`
	Nodes      []executionNodeDTO `json:"nodes"`
	Edges      []executionEdgeDTO `json:"edges"`
	DurationMS float64            `json:"durationMs"`
	Status     string             `json:"status"`
	Error      string             `json:"error,omitempty"`
}

type queryResponseDTO struct {
	QueryRunID  string            `json:"queryRunId"`
	Status      string            `json:"status"`
	DatasetID   string            `json:"datasetId"`
	Columns     []queryColumnDTO  `json:"columns"`
	Rows        [][]any           `json:"rows"`
	RowCount    int               `json:"rowCount"`
	ResultLimit int               `json:"resultLimit"`
	DurationMS  float64           `json:"durationMs"`
	Execution   executionTraceDTO `json:"execution"`
}

func queryResponse(result *query.Result, datasetID string, limits query.Limits) queryResponseDTO {
	return queryResponseDTO{
		QueryRunID:  result.RunID,
		Status:      string(result.Status),
		DatasetID:   datasetID,
		Columns:     queryColumns(result.Columns),
		Rows:        rowsOf(result.Rows),
		RowCount:    len(result.Rows),
		ResultLimit: limits.MaxRows,
		DurationMS:  durationMillis(result.Duration),
		Execution:   executionTrace(result.Trace),
	}
}

func queryColumns(columns []catalog.Column) []queryColumnDTO {
	out := make([]queryColumnDTO, len(columns))
	for i, column := range columns {
		out[i] = queryColumnDTO{
			ID:   string(column.ID),
			Name: column.Name,
			Type: column.Type.String(),
		}
	}
	return out
}

func executionTrace(trace metrics.Trace) executionTraceDTO {
	schemas := make(map[string][]catalog.Column, len(trace.Nodes))
	for _, node := range trace.Nodes {
		schemas[node.ID] = node.Schema
	}

	nodes := make([]executionNodeDTO, len(trace.Nodes))
	for i, node := range trace.Nodes {
		children := make([]string, len(node.Children))
		copy(children, node.Children)

		inputColumns := []catalog.Column(nil)
		if len(node.Children) > 0 {
			inputColumns = schemas[node.Children[0]]
		}

		nodes[i] = executionNodeDTO{
			ID:         node.ID,
			Kind:       string(node.Kind),
			Label:      node.Label,
			ParentID:   node.ParentID,
			Children:   children,
			Columns:    queryColumns(node.Schema),
			RowsIn:     node.RowsIn,
			RowsOut:    node.RowsOut,
			DurationMS: durationMillis(node.Duration),
			Status:     string(node.Status),
			Error:      node.Error,
			Input:      executionSample(node.Input, inputColumns),
			Output:     executionSample(node.Output, node.Schema),
		}
	}

	edges := make([]executionEdgeDTO, len(trace.Edges))
	for i, edge := range trace.Edges {
		edges[i] = executionEdgeDTO{From: edge.From, To: edge.To}
	}

	return executionTraceDTO{
		RootID:     trace.RootID,
		Nodes:      nodes,
		Edges:      edges,
		DurationMS: durationMillis(trace.Duration),
		Status:     string(trace.Status),
		Error:      trace.Error,
	}
}

func executionSample(sample metrics.Sample, columns []catalog.Column) executionSampleDTO {
	return executionSampleDTO{
		Columns:   queryColumns(columns),
		Rows:      rowsOf(sample.Rows),
		Truncated: sample.Truncated,
	}
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
