import { apiRequest } from "./client";
import type { DatasetDetail, ImportPreview, RowsPage, TableDetail } from "../../types/dataset";

export type ImportOptions = {
  tableName?: string;
  datasetName?: string;
  hasHeader?: boolean;
};

function buildUpload(file: File, options: ImportOptions): FormData {
  const form = new FormData();
  form.append("file", file);
  form.append("header", String(options.hasHeader ?? true));

  if (options.tableName) {
    form.append("tableName", options.tableName);
  }
  if (options.datasetName) {
    form.append("datasetName", options.datasetName);
  }

  return form;
}

export function fetchDatasets(signal?: AbortSignal): Promise<DatasetDetail[]> {
  return apiRequest<{ datasets: DatasetDetail[] }>("/api/v1/datasets", { signal }).then(
    (body) => body.datasets,
  );
}

export function fetchDataset(id: string, signal?: AbortSignal): Promise<DatasetDetail> {
  return apiRequest<DatasetDetail>(`/api/v1/datasets/${encodeURIComponent(id)}`, { signal });
}

export function fetchTable(
  datasetId: string,
  tableId: string,
  signal?: AbortSignal,
): Promise<TableDetail> {
  return apiRequest<TableDetail>(
    `/api/v1/datasets/${encodeURIComponent(datasetId)}/tables/${encodeURIComponent(tableId)}`,
    { signal },
  );
}

export function fetchRows(
  datasetId: string,
  tableId: string,
  offset: number,
  limit: number,
  signal?: AbortSignal,
): Promise<RowsPage> {
  const query = new URLSearchParams({ offset: String(offset), limit: String(limit) });
  return apiRequest<RowsPage>(
    `/api/v1/datasets/${encodeURIComponent(datasetId)}/tables/${encodeURIComponent(tableId)}/rows?${query}`,
    { signal },
  );
}

export function previewImport(file: File): Promise<ImportPreview> {
  return apiRequest<ImportPreview>("/api/v1/datasets/preview", {
    method: "POST",
    body: buildUpload(file, {}),
  });
}

export function createDataset(file: File, options: ImportOptions): Promise<DatasetDetail> {
  return apiRequest<DatasetDetail>("/api/v1/datasets", {
    method: "POST",
    body: buildUpload(file, options),
  });
}

export function deleteDataset(id: string): Promise<void> {
  return apiRequest<void>(`/api/v1/datasets/${encodeURIComponent(id)}`, { method: "DELETE" });
}
