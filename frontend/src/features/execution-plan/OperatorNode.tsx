import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { CircleAlert } from "lucide-react";

import { cn } from "../../lib/cn";
import { formatCount, formatDuration } from "../../lib/format";
import type { ExecutionNode } from "../../types/query";
import { operatorAccent, operatorMeta, statusMeta } from "./operatorMeta";

export type OperatorFlowNode = Node<{ node: ExecutionNode }, "operator">;

export function OperatorNode({ data, selected }: NodeProps<OperatorFlowNode>) {
  const { node } = data;
  const meta = operatorMeta(node.kind);
  const Icon = meta.icon;
  const status = statusMeta(node.status);
  const accent = operatorAccent(node);

  return (
    <div
      className={cn(
        "w-[248px] rounded-md border border-line bg-panel text-left shadow-sm transition-shadow",
        selected && "ring-2 ring-accent ring-offset-2 ring-offset-canvas",
      )}
      style={{ borderLeft: `2px solid ${accent}` }}
    >
      <Handle
        type="target"
        position={Position.Top}
        isConnectable={false}
        style={{
          width: 6,
          height: 6,
          background: "var(--ql-line-strong)",
          borderColor: "var(--ql-panel)",
        }}
      />

      <div className="px-3 py-2">
        <div className="flex items-center gap-1.5">
          <Icon className="size-3.5 shrink-0" style={{ color: accent }} aria-hidden="true" />
          <span className="text-[10px] font-semibold tracking-[0.08em] text-ink uppercase">
            {meta.name}
          </span>
          <span className="ml-auto font-mono text-[10px] text-ink-muted">
            {formatCount(node.rowsOut)} rows
          </span>
        </div>

        <p className="mt-1 truncate font-mono text-[11px] text-ink-muted" title={node.label}>
          {node.label}
        </p>

        <div className="mt-1.5 flex items-center gap-1.5 font-mono text-[10px] text-ink-faint">
          <span>in {node.kind === "scan" ? "—" : formatCount(node.rowsIn)}</span>
          <span aria-hidden="true">·</span>
          <span>out {formatCount(node.rowsOut)}</span>
          <span aria-hidden="true">·</span>
          <span>{formatDuration(node.durationMs)}</span>
          <span className="ml-auto flex items-center gap-1">
            {node.status === "failed" ? (
              <CircleAlert className="size-3 text-danger" aria-hidden="true" />
            ) : (
              <span className={cn("size-1.5 rounded-full", status.dotClass)} aria-hidden="true" />
            )}
            <span className={cn("text-[10px]", status.textClass)}>{status.label}</span>
          </span>
        </div>
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        isConnectable={false}
        style={{
          width: 6,
          height: 6,
          background: "var(--ql-line-strong)",
          borderColor: "var(--ql-panel)",
        }}
      />
    </div>
  );
}
