import { fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { QueryRunState } from "../query/useQueryRun";
import type { ExecutionNode, ExecutionTrace, QueryColumn } from "../../types/query";
import { ExecutionPlanView } from "./ExecutionPlanView";

const columns: QueryColumn[] = [{ id: "c1", name: "name", type: "text" }];

function node(
  id: string,
  kind: ExecutionNode["kind"],
  overrides: Partial<ExecutionNode> = {},
): ExecutionNode {
  return {
    id,
    kind,
    label: kind,
    children: [],
    columns,
    rowsIn: 0,
    rowsOut: 0,
    durationMs: 0,
    status: "completed",
    input: { columns: [], rows: [], truncated: false },
    output: { columns, rows: [], truncated: false },
    ...overrides,
  };
}

const trace: ExecutionTrace = {
  rootId: "n1",
  nodes: [
    node("n1", "limit", { label: "10", children: ["n2"], rowsIn: 3, rowsOut: 3 }),
    node("n2", "project", {
      label: "name",
      parentId: "n1",
      children: ["n3"],
      rowsIn: 3,
      rowsOut: 3,
    }),
    node("n3", "scan", { label: "employees", parentId: "n2", rowsOut: 3 }),
  ],
  edges: [
    { from: "n1", to: "n2" },
    { from: "n2", to: "n3" },
  ],
  durationMs: 1,
  status: "completed",
};

function runState(overrides: Partial<QueryRunState> = {}): QueryRunState {
  return {
    result: {
      queryRunId: "run-1",
      status: "completed",
      datasetId: "samples",
      columns,
      rows: [["Ada Lovelace"]],
      rowCount: 1,
      resultLimit: 1000,
      durationMs: 1.2,
      execution: trace,
    },
    error: null,
    status: "completed",
    stale: false,
    run: vi.fn(),
    cancel: vi.fn(),
    ...overrides,
  };
}

function Harness() {
  const [selected, setSelected] = useState<string | null>(null);
  return (
    <ExecutionPlanView
      state={runState()}
      selectedNodeId={selected}
      onSelectNode={setSelected}
      active
    />
  );
}

describe("ExecutionPlanView", () => {
  it("renders the backend operators and execution summary", () => {
    render(
      <ExecutionPlanView
        state={runState()}
        selectedNodeId={null}
        onSelectNode={() => undefined}
        active
      />,
    );

    const plan = screen.getByRole("region", { name: "Execution plan" });
    expect(within(plan).getByText("3 operators")).toBeInTheDocument();
    expect(within(plan).getByText("Limit")).toBeInTheDocument();
    expect(within(plan).getByText("Project")).toBeInTheDocument();
    expect(within(plan).getByText("Scan")).toBeInTheDocument();
  });

  it("selects a graph node for the inspector", () => {
    const onSelectNode = vi.fn();
    render(
      <ExecutionPlanView
        state={runState()}
        selectedNodeId={null}
        onSelectNode={onSelectNode}
        active
      />,
    );

    fireEvent.click(screen.getByText("Project"));

    expect(onSelectNode).toHaveBeenCalledWith("n2");
  });

  it("offers an accessible list view and shows the selected operator", () => {
    const { rerender } = render(
      <ExecutionPlanView
        state={runState()}
        selectedNodeId={null}
        onSelectNode={() => undefined}
        active
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "List" }));

    const list = screen.getByRole("list", { name: "Operators" });
    expect(within(list).getAllByRole("button")).toHaveLength(3);

    rerender(
      <ExecutionPlanView
        state={runState()}
        selectedNodeId="n2"
        onSelectNode={() => undefined}
        active
      />,
    );

    expect(screen.getByText("OPERATOR · n2")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Project" })).toBeInTheDocument();
  });

  it("keeps the selection when switching from the list back to the graph", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: "List" }));
    const list = screen.getByRole("list", { name: "Operators" });
    fireEvent.click(within(list).getAllByRole("button")[1]);

    expect(screen.getByText("OPERATOR · n2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Graph" }));

    expect(screen.getByText("OPERATOR · n2")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Project" })).toBeInTheDocument();
  });

  it("explains when no plan exists yet", () => {
    render(
      <ExecutionPlanView
        state={runState({ result: null, status: "idle" })}
        selectedNodeId={null}
        onSelectNode={() => undefined}
        active
      />,
    );

    expect(screen.getByText("No plan yet")).toBeInTheDocument();
  });

  it("marks a retained plan as stale", () => {
    render(
      <ExecutionPlanView
        state={runState({ stale: true, status: "failed" })}
        selectedNodeId={null}
        onSelectNode={() => undefined}
        active
      />,
    );

    expect(screen.getByText(/showing the last successful result/i)).toBeInTheDocument();
  });
});
