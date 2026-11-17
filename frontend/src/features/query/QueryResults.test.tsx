import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { QueryResults } from "./QueryResults";
import type { QueryRunState } from "./useQueryRun";
import type { QueryColumn, QueryResult } from "../../types/query";

const columns: QueryColumn[] = [
  { id: "c1", name: "name", type: "text" },
  { id: "c2", name: "salary", type: "integer" },
];

function queryResult(overrides: Partial<QueryResult> = {}): QueryResult {
  return {
    queryRunId: "run-1",
    status: "completed",
    datasetId: "samples",
    columns,
    rows: [["Ada Lovelace", "145000"]],
    rowCount: 1,
    resultLimit: 1000,
    durationMs: 1.5,
    execution: {
      rootId: "n1",
      nodes: [],
      edges: [],
      durationMs: 1,
      status: "completed",
    },
    ...overrides,
  };
}

function runState(overrides: Partial<QueryRunState> = {}): QueryRunState {
  return {
    result: null,
    error: null,
    status: "idle",
    stale: false,
    run: () => undefined,
    cancel: () => undefined,
    ...overrides,
  };
}

describe("QueryResults", () => {
  it("shows an idle empty state before a query runs", () => {
    render(<QueryResults state={runState()} />);

    expect(screen.getByText("No results yet")).toBeInTheDocument();
    expect(screen.getByText(/run the query to see its rows/i)).toBeInTheDocument();
  });

  it("shows an empty-result message without inventing rows", () => {
    render(
      <QueryResults
        state={runState({
          status: "completed",
          result: queryResult({ rows: [], rowCount: 0 }),
        })}
      />,
    );

    expect(screen.getByText("The query returned no rows.")).toBeInTheDocument();
    expect(screen.getByText("0 rows")).toBeInTheDocument();
  });

  it("keeps NULL and the empty string visually distinct", () => {
    render(
      <QueryResults
        state={runState({
          status: "completed",
          result: queryResult({
            columns: [
              { id: "c1", name: "missing", type: "text" },
              { id: "c2", name: "empty", type: "text" },
            ],
            rows: [[null, ""]],
            rowCount: 1,
          }),
        })}
      />,
    );

    expect(screen.getByText("NULL")).toBeInTheDocument();
    expect(screen.getByText('""')).toBeInTheDocument();
  });

  it("marks a preserved result as stale after a cancelled run", () => {
    render(
      <QueryResults
        state={runState({
          status: "cancelled",
          stale: true,
          result: queryResult(),
        })}
      />,
    );

    expect(screen.getByText("Query cancelled.")).toBeInTheDocument();
    expect(screen.getByText(/showing the last successful result/i)).toBeInTheDocument();
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
  });
});
