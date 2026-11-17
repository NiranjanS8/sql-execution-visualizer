import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ExecutionNode, ExecutionTrace, QueryColumn } from "../../types/query";
import { OperatorList } from "./OperatorList";

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
      status: "failed",
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

describe("OperatorList", () => {
  it("renders every operator in execution order with its status", () => {
    render(<OperatorList trace={trace} selectedNodeId={null} onSelectNode={() => undefined} />);

    const list = screen.getByRole("list", { name: "Operators" });
    const items = within(list).getAllByRole("button");

    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("Limit");
    expect(items[1]).toHaveTextContent("Project");
    expect(items[2]).toHaveTextContent("Scan");
    expect(items[1]).toHaveTextContent("failed");
  });

  it("reports the operator the user selects", () => {
    const onSelectNode = vi.fn();
    render(<OperatorList trace={trace} selectedNodeId={null} onSelectNode={onSelectNode} />);

    const list = screen.getByRole("list", { name: "Operators" });
    fireEvent.click(within(list).getAllByRole("button")[1]);

    expect(onSelectNode).toHaveBeenCalledWith("n2");
  });

  it("marks the selected operator", () => {
    render(<OperatorList trace={trace} selectedNodeId="n3" onSelectNode={() => undefined} />);

    const list = screen.getByRole("list", { name: "Operators" });
    const items = within(list).getAllByRole("button");

    expect(items[0]).toHaveAttribute("aria-pressed", "false");
    expect(items[2]).toHaveAttribute("aria-pressed", "true");
  });
});
