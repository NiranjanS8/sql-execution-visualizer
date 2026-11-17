import { useMemo, useState } from "react";
import { Play, Square } from "lucide-react";

import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { TabButton } from "../../components/ui/TabButton";
import { cn } from "../../lib/cn";
import type { Column } from "../../types/dataset";
import { ExecutionPlanView } from "../execution-plan/ExecutionPlanView";
import { useTable } from "../datasets/useDatasets";
import { QueryResults } from "./QueryResults";
import { SqlEditor } from "./SqlEditor";
import { useQueryRun } from "./useQueryRun";

type QueryWorkbenchProps = {
  datasetId: string;
  datasetName: string;
  tableId: string;
  tableName: string;
  visible?: boolean;
};

type Example = {
  label: string;
  sql: string;
};

// Generated examples must be valid for every imported table, including names
// that collide with reserved words, so identifiers are always quoted.
function quoteIdentifier(name: string): string {
  return `"${name.replaceAll('"', '""')}"`;
}

function defaultSQL(tableName: string): string {
  return `SELECT * FROM ${quoteIdentifier(tableName)} LIMIT 100;`;
}

function examplesFor(tableName: string, columns: Column[]): Example[] {
  const table = quoteIdentifier(tableName);
  const examples: Example[] = [{ label: "Preview rows", sql: defaultSQL(tableName) }];

  const sortable = columns.find((column) => column.type !== "boolean");
  if (sortable) {
    const name = quoteIdentifier(sortable.name);
    examples.push({
      label: `Order by ${sortable.name}`,
      sql: `SELECT * FROM ${table} ORDER BY ${name} LIMIT 100;`,
    });
  }

  const numeric = columns.find((column) => column.type === "integer" || column.type === "decimal");
  if (numeric) {
    const name = quoteIdentifier(numeric.name);
    examples.push({
      label: `Filter on ${numeric.name}`,
      sql: `SELECT * FROM ${table} WHERE ${name} > 0 LIMIT 100;`,
    });
  }

  const text = columns.find((column) => column.type === "text");
  if (text) {
    const name = quoteIdentifier(text.name);
    examples.push({
      label: `Select ${text.name}`,
      sql: `SELECT ${name} FROM ${table} LIMIT 100;`,
    });
  }

  return examples;
}

export function QueryWorkbench({
  datasetId,
  datasetName,
  tableId,
  tableName,
  visible = true,
}: QueryWorkbenchProps) {
  const table = useTable(datasetId, tableId);
  const [sql, setSql] = useState(() => defaultSQL(tableName));
  const [example, setExample] = useState("");
  const [outputTab, setOutputTab] = useState<"results" | "plan">("results");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const query = useQueryRun(datasetId);

  const runId = query.result?.queryRunId ?? null;

  const examples = useMemo(
    () => examplesFor(tableName, table.data?.columns ?? []),
    [tableName, table.data?.columns],
  );

  const canRun = query.status !== "running" && sql.trim() !== "";

  const handleRun = () => {
    if (!canRun) {
      return;
    }
    setSelectedNodeId(null);
    query.run(sql);
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex min-h-11 shrink-0 flex-wrap items-center gap-2 border-b border-line px-4 py-1.5">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-semibold tracking-tight">Query</h2>
            <Badge tone="neutral">{datasetName}</Badge>
            <Badge tone="neutral">{tableName}</Badge>
          </div>
          <p className="text-[10px] text-ink-faint">Ctrl/Cmd + Enter to run</p>
        </div>

        <div className="ml-auto flex flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="query-example">
            Example queries
          </label>
          <select
            id="query-example"
            value={example}
            onChange={(event) => {
              const next = event.target.value;
              setExample(next);
              if (next) {
                setSql(next);
              }
            }}
            className="h-7 max-w-44 rounded-sm border border-line bg-panel px-2 text-xs text-ink-muted"
          >
            <option value="">Examples…</option>
            {examples.map((item) => (
              <option key={item.label} value={item.sql}>
                {item.label}
              </option>
            ))}
          </select>

          <Button variant="primary" onClick={handleRun} disabled={!canRun}>
            <Play className="size-3.5" aria-hidden="true" />
            Run
          </Button>

          {query.status === "running" ? (
            <Button onClick={query.cancel}>
              <Square className="size-3.5" aria-hidden="true" />
              Cancel
            </Button>
          ) : null}
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-rows-[minmax(120px,220px)_minmax(0,1fr)]">
        <section aria-label="SQL editor" className="min-h-0 border-b border-line bg-panel">
          <SqlEditor value={sql} onChange={setSql} onRun={handleRun} visible={visible} />
        </section>

        <div className="flex min-h-0 flex-col">
          <div
            role="tablist"
            aria-label="Query output"
            className="flex h-9 shrink-0 items-center gap-1 border-b border-line px-4"
          >
            <TabButton active={outputTab === "results"} onClick={() => setOutputTab("results")}>
              Results
            </TabButton>
            <TabButton active={outputTab === "plan"} onClick={() => setOutputTab("plan")}>
              Execution plan
            </TabButton>
            {query.result ? (
              <span className="ml-auto font-mono text-[10px] text-ink-faint">
                {query.result.execution.nodes.length} operators
              </span>
            ) : null}
          </div>

          <div className="min-h-0 flex-1">
            <div className={cn("h-full", outputTab !== "results" && "hidden")}>
              <QueryResults state={query} />
            </div>
            <div className={cn("h-full", outputTab !== "plan" && "hidden")}>
              <ExecutionPlanView
                key={runId ?? "no-run"}
                state={query}
                selectedNodeId={selectedNodeId}
                onSelectNode={setSelectedNodeId}
                active={visible && outputTab === "plan"}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
