import {
  ArrowDownWideNarrow,
  Columns3,
  Filter,
  Scissors,
  Table2,
  type LucideIcon,
} from "lucide-react";

import type { ExecutionNode } from "../../types/query";

export type OperatorKind = ExecutionNode["kind"];

type OperatorMeta = {
  name: string;
  icon: LucideIcon;
  colorVariable: string;
};

const operators: Record<OperatorKind, OperatorMeta> = {
  scan: { name: "Scan", icon: Table2, colorVariable: "--ql-op-scan" },
  filter: { name: "Filter", icon: Filter, colorVariable: "--ql-op-filter" },
  sort: { name: "Sort", icon: ArrowDownWideNarrow, colorVariable: "--ql-op-sort" },
  project: { name: "Project", icon: Columns3, colorVariable: "--ql-op-project" },
  limit: { name: "Limit", icon: Scissors, colorVariable: "--ql-op-sort" },
};

export function operatorMeta(kind: OperatorKind): OperatorMeta {
  return operators[kind];
}

export function operatorAccent(node: ExecutionNode): string {
  if (node.status === "failed") {
    return "var(--ql-op-failed)";
  }
  return `var(${operatorMeta(node.kind).colorVariable})`;
}

type StatusMeta = {
  label: string;
  textClass: string;
  dotClass: string;
};

const statuses: Record<string, StatusMeta> = {
  not_started: {
    label: "not started",
    textClass: "text-ink-faint",
    dotClass: "bg-ink-faint",
  },
  running: {
    label: "running",
    textClass: "text-accent",
    dotClass: "bg-accent",
  },
  completed: {
    label: "completed",
    textClass: "text-success",
    dotClass: "bg-success",
  },
  stopped: {
    label: "stopped",
    textClass: "text-warning",
    dotClass: "bg-warning",
  },
  failed: {
    label: "failed",
    textClass: "text-danger",
    dotClass: "bg-danger",
  },
  cancelled: {
    label: "cancelled",
    textClass: "text-warning",
    dotClass: "bg-warning",
  },
};

// Unknown statuses are shown verbatim rather than mapped onto a familiar one.
export function statusMeta(status: string): StatusMeta {
  return (
    statuses[status] ?? {
      label: status,
      textClass: "text-ink-muted",
      dotClass: "bg-ink-muted",
    }
  );
}

export function configurationLabel(kind: OperatorKind): string {
  switch (kind) {
    case "scan":
      return "table";
    case "filter":
      return "predicate";
    case "sort":
      return "order";
    case "project":
      return "columns";
    case "limit":
      return "row limit";
  }
}
