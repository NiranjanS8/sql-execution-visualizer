import { AlertTriangle, Ban, CircleAlert, Clock3 } from "lucide-react";

import { DataGrid } from "../../components/data/DataGrid";
import { cn } from "../../lib/cn";
import type { QueryRunState } from "./useQueryRun";

function formatDuration(milliseconds: number): string {
  if (milliseconds < 1) {
    return "<1 ms";
  }
  if (milliseconds < 1000) {
    return `${milliseconds.toFixed(1)} ms`;
  }
  return `${(milliseconds / 1000).toFixed(2)} s`;
}

function ErrorNotice({ state }: { state: QueryRunState }) {
  const { error } = state;
  if (!error) {
    return null;
  }

  return (
    <div
      role="alert"
      className="flex gap-2.5 rounded-md border border-danger/40 bg-danger/10 px-3 py-2.5"
    >
      <CircleAlert className="mt-0.5 size-4 shrink-0 text-danger" aria-hidden="true" />
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-2">
          <span className="text-xs font-medium text-danger">Query failed</span>
          <span className="font-mono text-[10px] text-danger/80">{error.code}</span>
          {error.line ? (
            <span className="font-mono text-[10px] text-ink-faint">
              line {error.line}
              {error.column ? `, column ${error.column}` : ""}
            </span>
          ) : null}
        </div>
        <p className="mt-0.5 text-xs leading-relaxed text-ink-muted">{error.message}</p>
      </div>
    </div>
  );
}

function StaleNotice() {
  return (
    <div
      role="status"
      className="flex items-center gap-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-xs text-warning"
    >
      <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
      Showing the last successful result. The latest run did not complete.
    </div>
  );
}

export function QueryResults({ state }: { state: QueryRunState }) {
  const { result, status, stale } = state;

  if (!result) {
    return (
      <div role="region" aria-label="Query results" className="flex h-full flex-col">
        <ResultsHeader state={state} />
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 py-10 text-center">
          {status === "running" ? (
            <>
              <Clock3 className="size-5 animate-pulse text-ink-faint" aria-hidden="true" />
              <p className="mt-2 text-xs text-ink-muted">Running query…</p>
            </>
          ) : (
            <>
              <p className="text-sm font-medium">No results yet</p>
              <p className="mt-1 max-w-xs text-xs leading-relaxed text-ink-faint">
                Run the query to see its rows, column types, and execution summary.
              </p>
            </>
          )}
          {status !== "running" ? <div className="mt-4 w-full max-w-md"><ErrorNotice state={state} /></div> : null}
        </div>
      </div>
    );
  }

  return (
    <div role="region" aria-label="Query results" className="flex h-full min-h-0 flex-col">
      <ResultsHeader state={state} />

      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
        {stale ? <div className="mb-3"><StaleNotice /></div> : null}
        {state.error ? <div className="mb-3"><ErrorNotice state={state} /></div> : null}
        {status === "cancelled" ? (
          <div className="mb-3 flex items-center gap-2 rounded-md border border-line bg-elevated px-3 py-2 text-xs text-ink-muted">
            <Ban className="size-3.5" aria-hidden="true" />
            Query cancelled.
          </div>
        ) : null}

        <div className="overflow-hidden rounded-lg border border-line bg-panel">
          <DataGrid
            columns={result.columns}
            rows={result.rows}
            emptyMessage="The query returned no rows."
            stickyHeader
          />
        </div>
      </div>
    </div>
  );
}

function ResultsHeader({ state }: { state: QueryRunState }) {
  const { result, status } = state;

  return (
    <div className="flex h-9 shrink-0 items-center gap-2 border-b border-line px-4">
      <h3 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
        Results
      </h3>

      {status === "running" ? (
        <span className="flex items-center gap-1.5 text-[11px] text-ink-muted">
          <span className="size-1.5 animate-pulse rounded-full bg-accent" aria-hidden="true" />
          running
        </span>
      ) : null}

      {result ? (
        <div className="ml-auto flex flex-wrap items-center justify-end gap-x-3 gap-y-1 font-mono text-[10px] text-ink-faint">
          <span className={cn(status === "failed" && "text-warning")}>
            {result.rowCount} {result.rowCount === 1 ? "row" : "rows"}
          </span>
          <span>total {formatDuration(result.durationMs)}</span>
          <span>execution {formatDuration(result.execution.durationMs)}</span>
          <span>{result.execution.nodes.length} operators</span>
          <span>limit {result.resultLimit}</span>
          <span title={result.queryRunId}>{result.queryRunId}</span>
        </div>
      ) : null}
    </div>
  );
}
