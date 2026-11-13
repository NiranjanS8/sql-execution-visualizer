import { useState } from "react";
import { ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";

import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import type { Column } from "../../types/dataset";
import { DataGrid } from "./DataGrid";
import { useRows, useTable } from "./useDatasets";

const PAGE_SIZE = 50;

type TableDetailPanelProps = {
  datasetId: string;
  datasetName: string;
  tableId: string;
};

const nullabilityLabels = {
  unknown: "unknown",
  nullable: "yes",
  not_null: "no",
} as const;

function SchemaTable({ columns }: { columns: Column[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-xs">
        <thead>
          <tr className="border-b border-line">
            {["Column", "Type", "Nullable", "ID"].map((heading) => (
              <th
                key={heading}
                scope="col"
                className="px-3 py-2 text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
              >
                {heading}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {columns.map((column) => (
            <tr key={column.id} className="border-b border-line/60 last:border-0">
              <td className="px-3 py-1.5 font-mono text-[11px] text-ink">{column.name}</td>
              <td className="px-3 py-1.5 font-mono text-[11px] text-ink-muted">{column.type}</td>
              <td className="px-3 py-1.5 text-ink-muted">
                {nullabilityLabels[column.nullability]}
              </td>
              <td className="px-3 py-1.5 font-mono text-[10px] text-ink-faint">{column.id}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function TableDetailPanel({ datasetId, datasetName, tableId }: TableDetailPanelProps) {
  const [offset, setOffset] = useState(0);
  const table = useTable(datasetId, tableId);
  const rows = useRows(datasetId, tableId, offset, PAGE_SIZE);

  if (table.isPending) {
    return (
      <div className="space-y-3 px-6 py-6" aria-label="Loading table">
        <div className="h-6 w-48 animate-pulse rounded-sm bg-elevated" />
        <div className="h-32 animate-pulse rounded-lg bg-elevated" />
        <div className="h-48 animate-pulse rounded-lg bg-elevated" />
      </div>
    );
  }

  if (table.isError) {
    return (
      <div className="px-6 py-6">
        <h1 className="text-lg font-semibold tracking-tight">Table unavailable</h1>
        <p className="mt-1 text-xs text-danger">{table.error.message}</p>
        <Button className="mt-3" onClick={() => void table.refetch()}>
          <RefreshCw className="size-3.5" aria-hidden="true" />
          Retry
        </Button>
      </div>
    );
  }

  const detail = table.data;
  const page = rows.data;
  const firstRow = page && page.rows.length > 0 ? page.offset + 1 : 0;
  const lastRow = page ? page.offset + page.rows.length : 0;

  return (
    <div className="mx-auto w-full max-w-5xl px-6 py-5">
      <p className="text-[11px] text-ink-faint">{datasetName}</p>
      <div className="mt-1 flex flex-wrap items-center gap-2">
        <h1 className="text-lg font-semibold tracking-tight">{detail.name}</h1>
        <Badge tone="neutral">{detail.rowCount} rows</Badge>
        <Badge tone="neutral">{detail.columnCount} columns</Badge>
      </div>

      <section aria-labelledby="schema-heading" className="mt-5">
        <h2
          id="schema-heading"
          className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
        >
          Schema
        </h2>
        <div className="mt-2 overflow-hidden rounded-lg border border-line bg-panel">
          <SchemaTable columns={detail.columns} />
        </div>
      </section>

      <section aria-labelledby="rows-heading" className="mt-6">
        <div className="flex flex-wrap items-baseline gap-2">
          <h2
            id="rows-heading"
            className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
          >
            Rows
          </h2>
          {rows.isFetching && !rows.isPending ? (
            <span className="text-[10px] text-ink-faint">loading…</span>
          ) : null}
          <span className="ml-auto font-mono text-[10px] text-ink-faint">
            {page ? `${firstRow}–${lastRow} of ${page.totalRows}` : "—"}
          </span>
        </div>

        <div className="mt-2 overflow-hidden rounded-lg border border-line bg-panel">
          {rows.isPending ? (
            <div className="h-40 animate-pulse bg-elevated" aria-label="Loading rows" />
          ) : null}

          {rows.isError ? (
            <div className="px-4 py-4">
              <p className="text-xs text-danger">{rows.error.message}</p>
              <Button className="mt-2" onClick={() => void rows.refetch()}>
                <RefreshCw className="size-3.5" aria-hidden="true" />
                Retry
              </Button>
            </div>
          ) : null}

          {page ? (
            <DataGrid
              columns={page.columns}
              rows={page.rows}
              startIndex={page.offset}
              emptyMessage="This table has no rows."
            />
          ) : null}
        </div>

        <div className="mt-2 flex items-center gap-2">
          <Button onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))} disabled={offset === 0}>
            <ChevronLeft className="size-3.5" aria-hidden="true" />
            Previous
          </Button>
          <Button
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={!page?.hasMore}
            className="ml-auto"
          >
            Next
            <ChevronRight className="size-3.5" aria-hidden="true" />
          </Button>
        </div>
      </section>
    </div>
  );
}
