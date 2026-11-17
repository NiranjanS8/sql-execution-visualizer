import { CircleAlert } from "lucide-react";

import { cn } from "../../lib/cn";
import { formatCount, formatDuration } from "../../lib/format";
import type { ExecutionTrace } from "../../types/query";
import { dataFlowOrder } from "./graph";
import { operatorAccent, operatorMeta, statusMeta } from "./operatorMeta";

type OperatorListProps = {
  trace: ExecutionTrace;
  selectedNodeId: string | null;
  onSelectNode: (id: string | null) => void;
};

export function OperatorList({ trace, selectedNodeId, onSelectNode }: OperatorListProps) {
  const entries = dataFlowOrder(trace);

  return (
    <div className="h-full overflow-y-auto px-4 py-3">
      <ol aria-label="Operators" className="space-y-1">
        {entries.map(({ node, depth }) => {
          const meta = operatorMeta(node.kind);
          const Icon = meta.icon;
          const status = statusMeta(node.status);
          const selected = node.id === selectedNodeId;

          return (
            <li key={node.id}>
              <button
                type="button"
                aria-pressed={selected}
                onClick={() => onSelectNode(node.id)}
                className={cn(
                  "flex w-full items-center gap-2 rounded-sm border py-1.5 pr-2 text-left transition-colors",
                  selected ? "border-accent/60 bg-accent/10" : "border-transparent hover:bg-hover",
                )}
                style={{ paddingLeft: 8 + depth * 18 }}
              >
                <Icon
                  className="size-3.5 shrink-0"
                  style={{ color: operatorAccent(node) }}
                  aria-hidden="true"
                />
                <span className="w-14 shrink-0 text-[10px] font-medium tracking-[0.06em] text-ink uppercase">
                  {meta.name}
                </span>
                <span
                  className="min-w-0 flex-1 truncate font-mono text-[11px] text-ink-muted"
                  title={node.label}
                >
                  {node.label}
                </span>
                <span className="shrink-0 font-mono text-[10px] text-ink-faint">
                  out {formatCount(node.rowsOut)}
                </span>
                <span className="shrink-0 font-mono text-[10px] text-ink-faint">
                  {formatDuration(node.durationMs)}
                </span>
                <span
                  className={cn("flex shrink-0 items-center gap-1 text-[10px]", status.textClass)}
                >
                  {node.status === "failed" ? (
                    <CircleAlert className="size-3" aria-hidden="true" />
                  ) : null}
                  {status.label}
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
