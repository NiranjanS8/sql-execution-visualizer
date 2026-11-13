import { useState } from "react";
import { AlertTriangle, FileSpreadsheet, RefreshCw } from "lucide-react";

import { Button } from "../../components/ui/Button";
import { Dialog } from "../../components/ui/Dialog";
import { DataGrid } from "./DataGrid";
import { useCreateDataset, usePreviewImport } from "./useDatasets";

type ImportDialogProps = {
  open: boolean;
  onClose: () => void;
  onImported: (datasetId: string, tableId: string) => void;
};

const PREVIEW_ROWS = 8;

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KiB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function ImportDialog({ open, onClose, onImported }: ImportDialogProps) {
  const [file, setFile] = useState<File | null>(null);
  const [tableName, setTableName] = useState("");
  const [datasetName, setDatasetName] = useState("");
  const [fileError, setFileError] = useState<string | null>(null);

  const preview = usePreviewImport();
  const create = useCreateDataset();

  const reset = () => {
    setFile(null);
    setTableName("");
    setDatasetName("");
    setFileError(null);
    preview.reset();
    create.reset();
  };

  const close = () => {
    reset();
    onClose();
  };

  const chooseFile = (selected: File | null) => {
    setFileError(null);
    create.reset();
    setFile(selected);

    if (!selected) {
      preview.reset();
      return;
    }

    preview.mutate(selected, {
      onSuccess: (result) => {
        setTableName(result.tableName);
        setDatasetName(result.tableName);
      },
      onError: (error) => {
        setFileError(messageOf(error, "The file could not be read."));
      },
    });
  };

  const commit = () => {
    if (!file) {
      return;
    }

    create.mutate(
      { file, options: { tableName, datasetName } },
      {
        onSuccess: (dataset) => {
          const table = dataset.tables[0];
          reset();
          onImported(dataset.id, table?.id ?? "");
        },
        onError: (error) => {
          setFileError(messageOf(error, "The import failed."));
        },
      },
    );
  };

  const result = preview.data;
  const busy = preview.isPending || create.isPending;
  const error = fileError ?? (create.error ? messageOf(create.error, "The import failed.") : null);

  return (
    <Dialog
      open={open}
      onClose={close}
      title="Import CSV"
      description="The preview is read-only. Nothing is stored until you import."
      size="lg"
      footer={
        <>
          <Button onClick={close} disabled={create.isPending}>
            Cancel
          </Button>
          <Button variant="primary" onClick={commit} disabled={!result || busy}>
            {create.isPending ? "Importing…" : "Import"}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <div>
          <label
            htmlFor="csv-file"
            className="flex cursor-pointer items-center gap-3 rounded-lg border border-dashed border-line-strong bg-elevated px-4 py-3 hover:border-accent/60 hover:bg-hover"
          >
            <FileSpreadsheet className="size-5 shrink-0 text-ink-muted" aria-hidden="true" />
            <span className="min-w-0">
              <span className="block truncate text-xs text-ink">
                {file ? file.name : "Choose a CSV file"}
              </span>
              <span className="mt-0.5 block text-[11px] text-ink-faint">
                UTF-8, up to 5 MiB, 100,000 rows, 100 columns.
              </span>
            </span>
            <input
              id="csv-file"
              type="file"
              accept=".csv,text/csv"
              className="sr-only"
              onChange={(event) => chooseFile(event.target.files?.[0] ?? null)}
            />
          </label>
        </div>

        {preview.isPending ? (
          <p className="flex items-center gap-2 text-xs text-ink-muted">
            <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
            Reading and inferring types…
          </p>
        ) : null}

        {error ? (
          <div className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2">
            <p className="text-xs text-danger">{error}</p>
          </div>
        ) : null}

        {result ? (
          <>
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] text-ink-faint">
              <span>{result.rowCount} rows</span>
              <span>{result.columns.length} columns</span>
              <span>{formatBytes(result.byteCount)}</span>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
                  Table name
                </span>
                <input
                  value={tableName}
                  onChange={(event) => {
                    const next = event.target.value;
                    if (datasetName === result.tableName) {
                      setDatasetName(next);
                    }
                    setTableName(next);
                  }}
                  className="mt-1 h-8 w-full rounded-sm border border-line bg-canvas px-2 font-mono text-xs text-ink"
                />
              </label>
              <label className="block">
                <span className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
                  Dataset name
                </span>
                <input
                  value={datasetName}
                  onChange={(event) => setDatasetName(event.target.value)}
                  className="mt-1 h-8 w-full rounded-sm border border-line bg-canvas px-2 font-mono text-xs text-ink"
                />
              </label>
            </div>

            {result.warnings.length > 0 ? (
              <div className="rounded-md border border-warning/40 bg-warning/10 px-3 py-2">
                <p className="flex items-center gap-1.5 text-[11px] font-medium text-warning">
                  <AlertTriangle className="size-3.5" aria-hidden="true" />
                  Headers adjusted
                </p>
                <ul className="mt-1 list-disc space-y-0.5 pl-5 text-[11px] text-ink-muted">
                  {result.warnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            <section aria-labelledby="inferred-types-heading">
              <h3
                id="inferred-types-heading"
                className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
              >
                Inferred types
              </h3>
              <ul className="mt-1.5 grid gap-1 sm:grid-cols-2">
                {result.columns.map((column) => (
                  <li
                    key={column.name}
                    className="flex items-baseline gap-2 rounded-sm border border-line bg-elevated px-2 py-1 font-mono text-[11px]"
                  >
                    <span className="truncate text-ink">{column.name}</span>
                    <span className="ml-auto shrink-0 text-ink-muted">{column.type}</span>
                    {column.nullCount > 0 ? (
                      <span className="shrink-0 text-[10px] text-ink-faint">
                        {column.nullCount} NULL
                      </span>
                    ) : null}
                  </li>
                ))}
              </ul>
            </section>

            <section aria-labelledby="preview-rows-heading">
              <div className="flex items-baseline gap-2">
                <h3
                  id="preview-rows-heading"
                  className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
                >
                  Preview
                </h3>
                <span className="ml-auto font-mono text-[10px] text-ink-faint">
                  first {Math.min(result.rows.length, PREVIEW_ROWS)} of {result.rowCount}
                </span>
              </div>
              <div className="mt-1.5 overflow-hidden rounded-md border border-line">
                <DataGrid
                  columns={result.columns}
                  rows={result.rows.slice(0, PREVIEW_ROWS)}
                  emptyMessage="The file has no data rows."
                />
              </div>
            </section>

            <p className="text-[11px] text-ink-faint">
              Empty fields and{" "}
              <span className="font-mono text-ink-muted">
                {result.nullValues.filter(Boolean).join(", ")}
              </span>{" "}
              are imported as NULL.
            </p>
          </>
        ) : null}
      </div>
    </Dialog>
  );
}
