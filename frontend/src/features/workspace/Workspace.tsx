import { useState } from "react";
import { Database, Upload } from "lucide-react";

import { Button } from "../../components/ui/Button";
import { TabButton } from "../../components/ui/TabButton";
import { cn } from "../../lib/cn";
import { HealthPanel } from "../health/HealthPanel";
import { DatasetExplorer, type TableSelection } from "../datasets/DatasetExplorer";
import { ImportDialog } from "../datasets/ImportDialog";
import { TableDetailPanel } from "../datasets/TableDetailPanel";
import { useDatasets, useDeleteDataset } from "../datasets/useDatasets";
import { QueryWorkbench } from "../query/QueryWorkbench";
import type { DatasetDetail } from "../../types/dataset";

type DetailTab = "table" | "query";

function EmptyDetail({ onImport, hasDatasets }: { onImport: () => void; hasDatasets: boolean }) {
  return (
    <div className="mx-auto flex h-full max-w-md flex-col items-center justify-center px-6 py-12 text-center">
      <Database className="size-8 text-ink-faint" aria-hidden="true" />
      <h1 className="mt-3 text-base font-semibold tracking-tight">
        {hasDatasets ? "Select a table" : "No datasets yet"}
      </h1>
      <p className="mt-1 text-xs leading-relaxed text-ink-muted">
        {hasDatasets
          ? "Choose a table in the explorer to inspect its schema, preview its rows, and write SQL."
          : "Import a CSV file to create your first table, or explore the built-in sample datasets."}
      </p>
      {!hasDatasets ? (
        <Button className="mt-4" variant="primary" onClick={onImport}>
          <Upload className="size-3.5" aria-hidden="true" />
          Import CSV
        </Button>
      ) : null}
    </div>
  );
}

export function Workspace() {
  const datasets = useDatasets();
  const removeDataset = useDeleteDataset();

  const [selection, setSelection] = useState<TableSelection | null>(null);
  const [tab, setTab] = useState<DetailTab>("table");
  const [importOpen, setImportOpen] = useState(false);

  const handleImported = (datasetId: string, tableId: string) => {
    setImportOpen(false);
    if (datasetId && tableId) {
      setSelection({ datasetId, tableId });
    }
  };

  const handleDelete = (dataset: DatasetDetail) => {
    const confirmed = window.confirm(
      `Delete dataset "${dataset.name}"? Its tables are removed from this session.`,
    );
    if (!confirmed) {
      return;
    }

    removeDataset.mutate(dataset.id, {
      onSuccess: () => {
        if (selection?.datasetId === dataset.id) {
          setSelection(null);
        }
      },
      onError: (error) => {
        window.alert(error instanceof Error ? error.message : "Could not delete the dataset.");
      },
    });
  };

  if (datasets.isError && datasets.error instanceof TypeError) {
    return (
      <div className="mx-auto w-full max-w-3xl px-6 py-10">
        <HealthPanel />
      </div>
    );
  }

  const entries = datasets.data ?? [];
  const selectedDataset = selection
    ? entries.find((entry) => entry.id === selection.datasetId)
    : undefined;
  const selectedTable = selectedDataset?.tables.find((table) => table.id === selection?.tableId);

  return (
    <div className="flex h-full min-h-0 flex-col lg:grid lg:grid-cols-[260px_minmax(0,1fr)]">
      <DatasetExplorer
        datasets={entries}
        isLoading={datasets.isPending}
        error={datasets.isError ? datasets.error : null}
        selected={selection}
        onSelect={setSelection}
        onImport={() => setImportOpen(true)}
        onDelete={handleDelete}
        onRetry={() => void datasets.refetch()}
      />

      <section className="flex min-h-0 flex-col">
        {selection && selectedDataset ? (
          <>
            <div
              role="tablist"
              aria-label="Detail view"
              className="flex h-9 shrink-0 items-center gap-1 border-b border-line px-4"
            >
              <TabButton active={tab === "table"} onClick={() => setTab("table")}>
                Table
              </TabButton>
              <TabButton active={tab === "query"} onClick={() => setTab("query")}>
                Query
              </TabButton>
            </div>

            <div className="min-h-0 flex-1">
              <div className={cn("h-full overflow-y-auto", tab !== "table" && "hidden")}>
                <TableDetailPanel
                  key={`${selection.datasetId}/${selection.tableId}`}
                  datasetId={selection.datasetId}
                  datasetName={selectedDataset.name}
                  tableId={selection.tableId}
                />
              </div>

              <div className={cn("h-full", tab !== "query" && "hidden")}>
                <QueryWorkbench
                  key={`${selection.datasetId}/${selection.tableId}`}
                  datasetId={selection.datasetId}
                  datasetName={selectedDataset.name}
                  tableId={selection.tableId}
                  tableName={selectedTable?.name ?? selection.tableId}
                  visible={tab === "query"}
                />
              </div>
            </div>
          </>
        ) : (
          <EmptyDetail onImport={() => setImportOpen(true)} hasDatasets={entries.length > 0} />
        )}
      </section>

      <ImportDialog
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onImported={handleImported}
      />
    </div>
  );
}
