package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/dataset"
)

const (
	defaultRowLimit   = 50
	maxRowLimit       = 200
	previewRowLimit   = 50
	multipartOverhead = 1 << 20
)

type upload struct {
	filename    string
	data        []byte
	tableName   string
	datasetName string
	hasHeader   bool
	nullValues  []string
}

func (s *Server) handleListDatasets(w http.ResponseWriter, _ *http.Request) {
	entries := s.datasets.List()

	details := make([]datasetDetailDTO, len(entries))
	for i, entry := range entries {
		details[i] = detailDataset(entry)
	}

	writeJSON(w, s.log, http.StatusOK, datasetListDTO{Datasets: details})
}

func (s *Server) handleCreateDataset(w http.ResponseWriter, r *http.Request) {
	incoming, ok := s.readUpload(w, r)
	if !ok {
		return
	}

	parsed, err := dataset.ParseCSV(incoming.data, incoming.importOptions())
	if err != nil {
		s.writeImportFailure(w, err)
		return
	}

	name := incoming.datasetName
	if name == "" {
		name = parsed.Table.Name
	}

	created, err := s.datasets.Create(name, dataset.Source{
		Filename: incoming.filename,
		Bytes:    int64(len(incoming.data)),
	}, parsed.Table)
	if err != nil {
		s.writeImportFailure(w, err)
		return
	}

	writeJSON(w, s.log, http.StatusCreated, detailDataset(created))
}

func (s *Server) handlePreviewDataset(w http.ResponseWriter, r *http.Request) {
	incoming, ok := s.readUpload(w, r)
	if !ok {
		return
	}

	parsed, err := dataset.ParseCSV(incoming.data, incoming.importOptions())
	if err != nil {
		s.writeImportFailure(w, err)
		return
	}

	writeJSON(w, s.log, http.StatusOK, previewOf(parsed))
}

func (s *Server) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.datasetFromPath(w, r)
	if !ok {
		return
	}

	writeJSON(w, s.log, http.StatusOK, detailDataset(entry))
}

func (s *Server) handleDeleteDataset(w http.ResponseWriter, r *http.Request) {
	id := dataset.ID(r.PathValue("datasetID"))
	if err := s.datasets.Delete(id); err != nil {
		s.writeDatasetNotFound(w, id)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTables(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.datasetFromPath(w, r)
	if !ok {
		return
	}

	writeJSON(w, s.log, http.StatusOK, tableListDTO{Tables: summarizeTables(entry)})
}

func (s *Server) handleGetTable(w http.ResponseWriter, r *http.Request) {
	table, ok := s.tableFromPath(w, r)
	if !ok {
		return
	}

	writeJSON(w, s.log, http.StatusOK, detailTable(table))
}

func (s *Server) handleTableRows(w http.ResponseWriter, r *http.Request) {
	table, ok := s.tableFromPath(w, r)
	if !ok {
		return
	}

	offset, limit, err := parsePagination(r)
	if err != nil {
		writeError(w, s.log, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	total := table.RowCount()
	start := min(offset, total)
	end := min(start+limit, total)

	writeJSON(w, s.log, http.StatusOK, rowsPageDTO{
		TableID:   string(table.ID),
		Columns:   columnsOf(table),
		Rows:      rowsOf(table.Rows[start:end]),
		Offset:    start,
		Limit:     limit,
		TotalRows: total,
		HasMore:   end < total,
	})
}

func (s *Server) datasetFromPath(w http.ResponseWriter, r *http.Request) (*dataset.Dataset, bool) {
	id := dataset.ID(r.PathValue("datasetID"))
	entry, ok := s.datasets.Get(id)
	if !ok {
		s.writeDatasetNotFound(w, id)
		return nil, false
	}
	return entry, true
}

func (s *Server) tableFromPath(w http.ResponseWriter, r *http.Request) (*catalog.Table, bool) {
	entry, ok := s.datasetFromPath(w, r)
	if !ok {
		return nil, false
	}

	id := catalog.TableID(r.PathValue("tableID"))
	table, ok := entry.Tables.Get(id)
	if !ok {
		writeError(w, s.log, http.StatusNotFound, "table_not_found",
			fmt.Sprintf("Table %q does not exist in dataset %q.", id, entry.ID))
		return nil, false
	}

	return table, true
}

func (s *Server) writeDatasetNotFound(w http.ResponseWriter, id dataset.ID) {
	writeError(w, s.log, http.StatusNotFound, "dataset_not_found",
		fmt.Sprintf("Dataset %q does not exist.", id))
}

func (s *Server) writeImportFailure(w http.ResponseWriter, err error) {
	var importErr *dataset.ImportError
	if errors.As(err, &importErr) {
		writeErrorWithLocation(w, s.log, http.StatusBadRequest, "invalid_csv",
			importErr.Message, importErr.Line, importErr.Column)
		return
	}
	writeError(w, s.log, http.StatusBadRequest, "invalid_csv", err.Error())
}

// readUpload reads the multipart import request. It writes the error response
// itself and returns false when the request is unusable.
func (s *Server) readUpload(w http.ResponseWriter, r *http.Request) (*upload, bool) {
	limits := dataset.DefaultLimits()
	r.Body = http.MaxBytesReader(w, r.Body, limits.MaxBytes+multipartOverhead)

	if err := r.ParseMultipartForm(4 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, s.log, http.StatusRequestEntityTooLarge, "file_too_large",
				fmt.Sprintf("The upload exceeds the %d MiB limit.", limits.MaxBytes>>20))
			return nil, false
		}
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			`Expected a multipart/form-data body with a "file" part.`)
		return nil, false
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			`The request is missing the "file" part.`)
		return nil, false
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, limits.MaxBytes+1))
	if err != nil {
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			"Could not read the uploaded file.")
		return nil, false
	}
	if int64(len(data)) > limits.MaxBytes {
		writeError(w, s.log, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("The file is larger than the %d MiB import limit.", limits.MaxBytes>>20))
		return nil, false
	}

	hasHeader, err := parseBoolField(r.FormValue("header"), true)
	if err != nil {
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			`"header" must be true or false.`)
		return nil, false
	}

	return &upload{
		filename:    header.Filename,
		data:        data,
		tableName:   r.FormValue("tableName"),
		datasetName: r.FormValue("datasetName"),
		hasHeader:   hasHeader,
		nullValues:  parseNullValues(r.MultipartForm.Value["nullValues"]),
	}, true
}

func (u *upload) importOptions() dataset.ImportOptions {
	return dataset.ImportOptions{
		Filename:   u.filename,
		TableName:  u.tableName,
		HasHeader:  u.hasHeader,
		NullValues: u.nullValues,
	}
}

func parsePagination(r *http.Request) (int, int, error) {
	offset := 0
	limit := defaultRowLimit

	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New(`"offset" must be a non-negative integer.`)
		}
		offset = parsed
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New(`"limit" must be a positive integer.`)
		}
		limit = min(parsed, maxRowLimit)
	}

	return offset, limit, nil
}

func parseBoolField(raw string, fallback bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return fallback, nil
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%q is not a boolean", raw)
	}
}

func parseNullValues(entries []string) []string {
	var values []string
	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				values = append(values, trimmed)
			}
		}
	}
	return values
}
