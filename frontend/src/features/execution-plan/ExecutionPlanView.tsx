import { useState } from "react";
import { CircleAlert, Clock3, List, Waypoints } from "lucide-react";

import { Button } from "../../components/ui/Button";
import { formatDuration } from "../../lib/format";
import { StaleNotice } from "../query/StaleNotice";
import type { QueryRunState } from "../query/useQueryRun";
import { ExecutionGraph } from "./ExecutionGraph";
import { OperatorInspector } from "./OperatorInspector";
import { OperatorList } from "./OperatorList";

type PlanView = "graph" | "list";

type ExecutionPlanViewProps = {
  state: QueryRunState;
  selectedNodeId: string | null;
  onSelectNode: (id: string | null) => void;
  active: boolean;
};

export function ExecutionPlanView({
  state,
  selectedNodeId,
  onSelectNode,
  active,
}: ExecutionPlanViewProps) {
  const [view, setView] = useState<PlanView>("graph");
  const { result, status, stale } = state;

  if (!result) {
    return (
      <div role="region" aria-label="Execution plan" className="flex h-full flex-col">
        <div className="flex h-9 shrink-0 items-center border-b border-line px-4">
          <span className="text-[10px] tracking-[0.08em] text-ink-faint uppercase">No plan</span>
        </div>
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 py-10 text-center">
          {status === "running" ? (
            <>
              <Clock3 className="size-5 animate-pulse text-ink-faint" aria-hidden="true" />
              <p className="mt-2 text-xs text-ink-muted">Running query…</p>
            </>
          ) : (
            <>
              <Waypoints className="size-5 text-ink-faint" aria-hidden="true" />
              <p className="mt-2 text-sm font-medium">No plan yet</p>
              <p className="mt-1 max-w-xs text-xs leading-relaxed text-ink-faint">
                Run the query to see its operator graph, row counts, and intermediate samples.
              </p>
              {state.error ? (
                <p className="mt-2 max-w-xs font-mono text-[10px] text-danger">
                  {state.error.message}
                </p>
              ) : null}
            </>
          )}
        </div>
      </div>
    );
  }

  const trace = result.execution;

  if (trace.nodes.length === 0) {
    return (
      <div role="region" aria-label="Execution plan" className="flex h-full flex-col">
        <div className="flex h-9 shrink-0 items-center border-b border-line px-4">
          <span className="text-[10px] tracking-[0.08em] text-ink-faint uppercase">No plan</span>
        </div>
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 py-10 text-center">
          <Waypoints className="size-5 text-ink-faint" aria-hidden="true" />
          <p className="mt-2 text-sm font-medium">No plan — the query did not execute.</p>
        </div>
      </div>
    );
  }

  const selectedNode = trace.nodes.find((node) => node.id === selectedNodeId) ?? null;

  return (
    <div role="region" aria-label="Execution plan" className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-line px-4">
        <span className="font-mono text-[10px] text-ink-faint">{trace.nodes.length} operators</span>
        <span aria-hidden="true" className="text-line-strong">
          ·
        </span>
        <span className="font-mono text-[10px] text-ink-faint">
          execution {formatDuration(trace.durationMs)}
        </span>
        <span aria-live="polite" className="font-mono text-[10px] text-ink-faint">
          {status === "running" ? "running" : trace.status}
        </span>

        <div role="group" aria-label="Plan view" className="ml-auto flex items-center gap-1">
          <Button
            size="sm"
            variant={view === "graph" ? "primary" : "ghost"}
            aria-pressed={view === "graph"}
            onClick={() => setView("graph")}
          >
            <Waypoints className="size-3.5" aria-hidden="true" />
            Graph
          </Button>
          <Button
            size="sm"
            variant={view === "list" ? "primary" : "ghost"}
            aria-pressed={view === "list"}
            onClick={() => setView("list")}
          >
            <List className="size-3.5" aria-hidden="true" />
            List
          </Button>
        </div>
      </div>

      {stale ? (
        <div className="px-4 pt-2">
          <StaleNotice />
        </div>
      ) : null}

      {trace.error ? (
        <div className="px-4 pt-2">
          <div
            role="alert"
            className="flex gap-2 rounded-sm border border-danger/40 bg-danger/10 px-2.5 py-2"
          >
            <CircleAlert className="mt-0.5 size-3.5 shrink-0 text-danger" aria-hidden="true" />
            <p className="min-w-0 break-words font-mono text-[11px] text-danger">{trace.error}</p>
          </div>
        </div>
      ) : null}

      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <div className="min-h-0 flex-1">
          {view === "graph" ? (
            <ExecutionGraph
              trace={trace}
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
              active={active}
            />
          ) : (
            <OperatorList
              trace={trace}
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
            />
          )}
        </div>

        <div className="min-h-0 max-h-[45%] w-full shrink-0 overflow-hidden border-t border-line lg:max-h-none lg:w-80 lg:border-t-0 lg:border-l">
          <OperatorInspector node={selectedNode} />
        </div>
      </div>
    </div>
  );
}
