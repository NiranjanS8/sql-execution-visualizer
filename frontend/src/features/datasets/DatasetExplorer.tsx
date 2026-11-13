import { Database, RefreshCw, Table2, Trash2, Upload } from "lucide-react";

import { Button } from "../../components/ui/Button";
import { cn } from "../../lib/cn";
import type { DatasetDetail } from "../../types/dataset";

export type TableSelection = {
  datasetId: string;
  tableId: string;
};

type DatasetExplorerProps = {
  datasets: DatasetDetail[];
  isLoading: boolean;
  error: Error | null;
  selected: TableSelection | null;
  onSelect: (selection: TableSelection) => void;
  onImport: () => void;
  onDelete: (dataset: DatasetDetail) => void;
  onRetry: () => void;
};

function SkeletonRow() {
  return <div className="mx-2 my-1 h-7 animate-pulse rounded-sm bg-elevated" />;
}

export function DatasetExplorer({
  datasets,
  isLoading,
  error,
  selected,
  onSelect,
  onImport,
  onDelete,
  onRetry,
}: DatasetExplorerProps) {
  return (
    <aside className="flex min-h-0 flex-col border-line bg-panel lg:border-r">
      <header className="flex h-11 shrink-0 items-center gap-2 border-b border-line px-3">
        <h2 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
          Explorer
        </h2>
        <Button className="ml-auto" onClick={onImport}>
          <Upload className="size-3.5" aria-hidden="true" />
          Import CSV
        </Button>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto py-2">
        {isLoading ? (
          <div aria-label="Loading datasets">
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
          </div>
        ) : null}

        {error ? (
          <div className="px-3 py-4">
            <p className="text-xs text-danger">{error.message}</p>
            <Button className="mt-2" onClick={onRetry}>
              <RefreshCw className="size-3.5" aria-hidden="true" />
              Retry
            </Button>
          </div>
        ) : null}

        {!isLoading && !error && datasets.length === 0 ? (
          <div className="px-3 py-6">
            <p className="text-xs text-ink-muted">No datasets yet.</p>
            <p className="mt-1 text-xs text-ink-faint">
              Import a CSV file to create your first table.
            </p>
            <Button className="mt-3" variant="primary" onClick={onImport}>
              <Upload className="size-3.5" aria-hidden="true" />
              Import CSV
            </Button>
          </div>
        ) : null}

        {datasets.map((dataset) => (
          <section key={dataset.id} className="mb-3">
            <div className="flex items-center gap-2 px-3 py-1">
              <Database className="size-3.5 shrink-0 text-ink-faint" aria-hidden="true" />
              <h3 className="min-w-0 truncate text-[11px] font-medium text-ink-muted">
                {dataset.name}
              </h3>
              <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-faint">
                {dataset.tableCount} {dataset.tableCount === 1 ? "table" : "tables"}
              </span>
              <button
                type="button"
                onClick={() => onDelete(dataset)}
                aria-label={`Delete dataset ${dataset.name}`}
                title={`Delete dataset ${dataset.name}`}
                className="shrink-0 rounded-xs p-1 text-ink-faint hover:bg-hover hover:text-danger"
              >
                <Trash2 className="size-3.5" aria-hidden="true" />
              </button>
            </div>

            <ul className="mt-0.5">
              {dataset.tables.map((table) => {
                const active =
                  selected?.datasetId === dataset.id && selected.tableId === table.id;

                return (
                  <li key={table.id}>
                    <button
                      type="button"
                      onClick={() => onSelect({ datasetId: dataset.id, tableId: table.id })}
                      aria-current={active ? "true" : undefined}
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-1.5 pl-8 text-left text-xs",
                        active
                          ? "bg-hover text-ink"
                          : "text-ink-muted hover:bg-hover hover:text-ink",
                      )}
                    >
                      <Table2 className="size-3.5 shrink-0" aria-hidden="true" />
                      <span className="min-w-0 truncate">{table.name}</span>
                      <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-faint">
                        {table.rowCount} rows
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>
    </aside>
  );
}
