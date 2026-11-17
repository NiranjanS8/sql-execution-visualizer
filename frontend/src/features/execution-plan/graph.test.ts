import { describe, expect, it } from "vitest";

import type { ExecutionNode, ExecutionTrace, QueryColumn } from "../../types/query";
import {
  dataFlowOrder,
  displayEdges,
  filteredOutPercent,
  layoutPlan,
  operatorDepths,
} from "./graph";

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

function trace(): ExecutionTrace {
  return {
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
}

describe("displayEdges", () => {
  it("reverses backend edges so data flows from producer to consumer", () => {
    expect(displayEdges(trace())).toEqual([
      { id: "n2->n1", source: "n2", target: "n1" },
      { id: "n3->n2", source: "n3", target: "n2" },
    ]);
  });

  it("drops self edges rather than drawing a loop", () => {
    const value = trace();
    value.edges = [{ from: "n1", to: "n1" }];
    expect(displayEdges(value)).toEqual([]);
  });
});

describe("operatorDepths", () => {
  it("counts parents from the root", () => {
    expect([...operatorDepths(trace())]).toEqual([
      ["n1", 0],
      ["n2", 1],
      ["n3", 2],
    ]);
  });

  it("keeps the backend pre-order for the operator list", () => {
    expect(dataFlowOrder(trace()).map(({ node: entry, depth }) => [entry.id, depth])).toEqual([
      ["n1", 0],
      ["n2", 1],
      ["n3", 2],
    ]);
  });
});

describe("filteredOutPercent", () => {
  it("derives the share of rows a filter removed", () => {
    const filter = node("n1", "filter", { rowsIn: 8, rowsOut: 3 });
    expect(filteredOutPercent(filter)).toBe(62.5);
  });

  it("returns null when no rows entered the filter", () => {
    const filter = node("n1", "filter", { rowsIn: 0, rowsOut: 0 });
    expect(filteredOutPercent(filter)).toBeNull();
  });

  it("returns null for operators that are not filters", () => {
    expect(filteredOutPercent(node("n1", "project", { rowsIn: 8, rowsOut: 3 }))).toBeNull();
  });
});

describe("layoutPlan", () => {
  it("places producers above consumers", () => {
    const layout = layoutPlan(trace());
    const byId = new Map(layout.nodes.map((entry) => [entry.node.id, entry]));

    const scan = byId.get("n3");
    const project = byId.get("n2");
    const limit = byId.get("n1");

    expect(scan).toBeDefined();
    expect(project).toBeDefined();
    expect(limit).toBeDefined();
    expect(scan!.y).toBeLessThan(project!.y);
    expect(project!.y).toBeLessThan(limit!.y);
    expect(scan!.x).toBe(project!.x);
    expect(project!.x).toBe(limit!.x);
  });

  it("returns an empty layout for an empty trace", () => {
    expect(
      layoutPlan({
        rootId: "",
        nodes: [],
        edges: [],
        durationMs: 0,
        status: "completed",
      }),
    ).toEqual({ nodes: [], edges: [] });
  });
});
