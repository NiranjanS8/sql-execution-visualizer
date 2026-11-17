import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ExecutionNode, QueryColumn } from "../../types/query";
import { OperatorInspector } from "./OperatorInspector";

const columns: QueryColumn[] = [
  { id: "c1", name: "name", type: "text" },
  { id: "c2", name: "salary", type: "integer" },
];

function filterNode(overrides: Partial<ExecutionNode> = {}): ExecutionNode {
  return {
    id: "n2",
    kind: "filter",
    label: '"salary" > 50000',
    children: ["n3"],
    columns,
    rowsIn: 8,
    rowsOut: 3,
    durationMs: 2.4,
    status: "completed",
    input: {
      columns,
      rows: [
        ["Ada Lovelace", "145000"],
        ["Grace Hopper", "132000"],
        ["Alan Turing", "151000"],
      ],
      truncated: true,
    },
    output: {
      columns,
      rows: [["Ada Lovelace", "145000"]],
      truncated: false,
    },
    ...overrides,
  };
}

describe("OperatorInspector", () => {
  it("asks for a selection when nothing is selected", () => {
    render(<OperatorInspector node={null} />);

    expect(screen.getByText(/select an operator/i)).toBeInTheDocument();
  });

  it("shows backend metrics, configuration, schema, and samples", () => {
    render(<OperatorInspector node={filterNode()} />);

    expect(screen.getByRole("heading", { name: "Filter" })).toBeInTheDocument();
    expect(screen.getByText("OPERATOR · n2")).toBeInTheDocument();

    const metrics = screen.getByRole("region", { name: "Metrics" });
    expect(within(metrics).getByText("8")).toBeInTheDocument();
    expect(within(metrics).getByText("3")).toBeInTheDocument();
    expect(within(metrics).getByText("62.5%")).toBeInTheDocument();
    expect(within(metrics).getByText("2.4 ms")).toBeInTheDocument();

    expect(screen.getByText('"salary" > 50000')).toBeInTheDocument();

    const schema = screen.getByRole("region", { name: "Schema" });
    expect(within(schema).getAllByText("salary").length).toBeGreaterThan(0);
    expect(within(schema).getAllByText("integer").length).toBeGreaterThan(0);

    const inputSample = screen.getByRole("region", { name: "Input sample" });
    expect(within(inputSample).getByText("3 of 8 rows")).toBeInTheDocument();
    expect(within(inputSample).getByText("Ada Lovelace")).toBeInTheDocument();
    expect(within(inputSample).getByText(/sample truncated/i)).toBeInTheDocument();

    const outputSample = screen.getByRole("region", { name: "Output sample" });
    expect(within(outputSample).getByText("1 of 3 rows")).toBeInTheDocument();
    expect(within(outputSample).getByText("Ada Lovelace")).toBeInTheDocument();
  });

  it("does not claim an input for a scan", () => {
    const scan = filterNode({
      id: "n3",
      kind: "scan",
      label: "employees",
      rowsIn: 0,
      rowsOut: 3,
      input: { columns: [], rows: [], truncated: false },
    });

    render(<OperatorInspector node={scan} />);

    const metrics = screen.getByRole("region", { name: "Metrics" });
    expect(within(metrics).getByText("—")).toBeInTheDocument();

    const inputSample = screen.getByRole("region", { name: "Input sample" });
    expect(within(inputSample).getByText(/reads the table directly/i)).toBeInTheDocument();
  });

  it("shows an operator error from the backend", () => {
    render(
      <OperatorInspector node={filterNode({ status: "failed", error: "division by zero" })} />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("division by zero");
    expect(screen.getByText("failed")).toBeInTheDocument();
  });
});
