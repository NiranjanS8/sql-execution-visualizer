import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createDataset,
  deleteDataset,
  fetchDatasets,
  fetchRows,
  fetchTable,
  previewImport,
  type ImportOptions,
} from "../../lib/api/datasets";

const datasetsKey = ["datasets"] as const;

export function useDatasets() {
  return useQuery({
    queryKey: datasetsKey,
    queryFn: ({ signal }) => fetchDatasets(signal),
  });
}

export function useTable(datasetId: string, tableId: string) {
  return useQuery({
    queryKey: ["datasets", datasetId, "tables", tableId],
    queryFn: ({ signal }) => fetchTable(datasetId, tableId, signal),
    enabled: Boolean(datasetId && tableId),
  });
}

export function useRows(datasetId: string, tableId: string, offset: number, limit: number) {
  return useQuery({
    queryKey: ["datasets", datasetId, "tables", tableId, "rows", offset, limit],
    queryFn: ({ signal }) => fetchRows(datasetId, tableId, offset, limit, signal),
    enabled: Boolean(datasetId && tableId),
    placeholderData: keepPreviousData,
  });
}

export function useDeleteDataset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => deleteDataset(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: datasetsKey });
    },
  });
}

export function usePreviewImport() {
  return useMutation({
    mutationFn: (file: File) => previewImport(file),
  });
}

export function useCreateDataset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ file, options }: { file: File; options: ImportOptions }) =>
      createDataset(file, options),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: datasetsKey });
    },
  });
}
