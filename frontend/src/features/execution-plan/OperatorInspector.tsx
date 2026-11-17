import { AlertTriangle, CircleAlert } from "lucide-react";

import { DataGrid } from "../../components/data/DataGrid";
import { cn } from "../../lib/cn";
import { formatCount, formatDuration } from "../../lib/format";
import type { ExecutionNode, ExecutionSample, QueryColumn } from "../../types/query";
import { filteredOutPercent } from "./graph";
import { configurationLabel, operatorAccent, operatorMeta, statusMeta } from "./operatorMeta";

type OperatorInspectorProps = {
  node: ExecutionNode | null;
};

export function OperatorInspector({ node }: OperatorInspectorProps) {
  if (!node) {
    return (
      <aside aria-label="Operator inspector" className="flex h-full flex-col bg-panel">
        <div className="shrink-0 border-b border-line px-4 py-3">
          <h3 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
            Inspector
          </h3>
        </div>
        <div className="flex flex-1 items-center justify-center px-6 py-8 text-center">
          <p className="max-w-[15rem] text-xs leading-relaxed text-ink-faint">
            Select an operator to inspect its configuration, metrics, schema, and samples.
          </p>
        </div>
      </aside>
    );
  }

  const meta = operatorMeta(node.kind);
  const Icon = meta.icon;
  const status = statusMeta(node.status);
  const accent = operatorAccent(node);
  const filtered = filteredOutPercent(node);

  return (
    <aside aria-label="Operator inspector" className="flex h-full min-h-0 flex-col bg-panel">
      <header className="shrink-0 border-b border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <Icon className="size-4 shrink-0" style={{ color: accent }} aria-hidden="true" />
          <h3 className="text-sm font-semibold tracking-tight">{meta.name}</h3>
          <span className="ml-auto flex items-center gap-1.5">
            {node.status === "failed" ? (
              <CircleAlert className="size-3 text-danger" aria-hidden="true" />
            ) : (
              <span className={cn("size-1.5 rounded-full", status.dotClass)} aria-hidden="true" />
            )}
            <span className={cn("text-[11px]", status.textClass)}>{status.label}</span>
          </span>
        </div>
        <p className="mt-0.5 font-mono text-[10px] text-ink-faint">OPERATOR · {node.id}</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto">
        <section
          aria-label="Metrics"
          className={cn(
            "grid gap-px border-b border-line bg-line",
            filtered === null ? "grid-cols-3" : "grid-cols-4",
          )}
        >
          <Metric label="input" value={node.kind === "scan" ? "—" : formatCount(node.rowsIn)} />
          <Metric label="output" value={formatCount(node.rowsOut)} />
          <Metric label="duration" value={formatDuration(node.durationMs)} />
          {filtered === null ? null : <Metric label="filtered" value={`${filtered.toFixed(1)}%`} />}
        </section>

        {node.error ? (
          <section aria-label="Error" className="border-b border-line px-4 py-3">
            <div
              role="alert"
              className="flex gap-2 rounded-sm border border-danger/40 bg-danger/10 px-2.5 py-2"
            >
              <CircleAlert className="mt-0.5 size-3.5 shrink-0 text-danger" aria-hidden="true" />
              <p className="min-w-0 break-words font-mono text-[11px] text-danger">{node.error}</p>
            </div>
          </section>
        ) : null}

        <section aria-label="Configuration" className="border-b border-line px-4 py-3">
          <h4 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
            Configuration
          </h4>
          <dl className="mt-1.5 grid grid-cols-[5.5rem_minmax(0,1fr)] gap-x-2 gap-y-1 text-xs">
            <dt className="text-ink-faint">{configurationLabel(node.kind)}</dt>
            <dd className="min-w-0 break-words font-mono text-ink-muted">{node.label}</dd>
          </dl>
        </section>

        <section aria-label="Schema" className="border-b border-line px-4 py-3">
          <h4 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
            Schema
          </h4>
          <div className="mt-1.5 space-y-2">
            <SchemaRow label="in" columns={node.input.columns} empty="No input columns." />
            <SchemaRow label="out" columns={node.output.columns} empty="No output columns." />
          </div>
        </section>

        <SampleSection
          title="Input sample"
          sample={node.input}
          totalRows={node.kind === "scan" ? 0 : node.rowsIn}
          emptyMessage={
            node.kind === "scan"
              ? "No input — this operator reads the table directly."
              : "No rows entered this operator."
          }
        />
        <SampleSection
          title="Output sample"
          sample={node.output}
          totalRows={node.rowsOut}
          emptyMessage="No rows left this operator."
        />
      </div>
    </aside>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-panel px-3 py-2">
      <div className="truncate font-mono text-sm text-ink">{value}</div>
      <div className="mt-0.5 text-[9px] tracking-[0.08em] text-ink-faint uppercase">{label}</div>
    </div>
  );
}

function SchemaRow({
  label,
  columns,
  empty,
}: {
  label: string;
  columns: QueryColumn[];
  empty: string;
}) {
  return (
    <div className="grid grid-cols-[2rem_minmax(0,1fr)] gap-2 text-xs">
      <span className="pt-0.5 font-mono text-[10px] text-ink-faint">{label}</span>
      {columns.length === 0 ? (
        <span className="text-ink-faint">{empty}</span>
      ) : (
        <ul className="flex flex-wrap gap-1">
          {columns.map((column) => (
            <li
              key={column.id}
              className="rounded-xs border border-line bg-elevated px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
            >
              {column.name} <span className="text-ink-faint">{column.type}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function SampleSection({
  title,
  sample,
  totalRows,
  emptyMessage,
}: {
  title: string;
  sample: ExecutionSample;
  totalRows: number;
  emptyMessage: string;
}) {
  return (
    <section aria-label={title} className="border-b border-line px-4 py-3 last:border-b-0">
      <div className="flex items-baseline justify-between gap-2">
        <h4 className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
          {title}
        </h4>
        <span className="font-mono text-[10px] text-ink-faint">
          {sample.rows.length} of {formatCount(totalRows)} rows
        </span>
      </div>

      {sample.rows.length === 0 ? (
        <p className="mt-2 text-xs text-ink-faint">{emptyMessage}</p>
      ) : (
        <div className="mt-2 overflow-hidden rounded-sm border border-line">
          <DataGrid columns={sample.columns} rows={sample.rows} />
        </div>
      )}

      {sample.truncated ? (
        <p className="mt-1.5 flex items-start gap-1.5 text-[10px] text-warning">
          <AlertTriangle className="mt-px size-3 shrink-0" aria-hidden="true" />
          Sample truncated. The recorder retained {sample.rows.length} rows from{" "}
          {formatCount(totalRows)}.
        </p>
      ) : null}
    </section>
  );
}
