import type { Cell, ColumnType } from "./dataset";

// A result column can be the NULL literal, which has no column type. The API
// reports it as "null" so the grid can render it without guessing.
export type ResultColumnType = ColumnType | "null";

export type QueryColumn = {
  id: string;
  name: string;
  type: ResultColumnType;
};

export type ExecutionSample = {
  columns: QueryColumn[];
  rows: Cell[][];
  truncated: boolean;
};

export type ExecutionNode = {
  id: string;
  kind: "scan" | "filter" | "sort" | "project" | "limit";
  label: string;
  parentId?: string;
  children: string[];
  columns: QueryColumn[];
  rowsIn: number;
  rowsOut: number;
  durationMs: number;
  status: string;
  error?: string;
  input: ExecutionSample;
  output: ExecutionSample;
};

export type ExecutionEdge = {
  from: string;
  to: string;
};

export type ExecutionTrace = {
  rootId: string;
  nodes: ExecutionNode[];
  edges: ExecutionEdge[];
  durationMs: number;
  status: string;
  error?: string;
};

export type QueryResult = {
  queryRunId: string;
  status: string;
  datasetId: string;
  columns: QueryColumn[];
  rows: Cell[][];
  rowCount: number;
  resultLimit: number;
  durationMs: number;
  execution: ExecutionTrace;
};
