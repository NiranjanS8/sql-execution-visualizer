import dagre from "@dagrejs/dagre";

import type { ExecutionNode, ExecutionTrace } from "../../types/query";

export const OPERATOR_NODE_WIDTH = 248;
export const OPERATOR_NODE_HEIGHT = 92;

export type DisplayEdge = {
  id: string;
  source: string;
  target: string;
};

export type PositionedOperator = {
  node: ExecutionNode;
  x: number;
  y: number;
  depth: number;
};

export type PlanLayout = {
  nodes: PositionedOperator[];
  edges: DisplayEdge[];
};

// Backend edges point from a consumer to its input. React Flow draws data
// flowing from source to target, so display edges are reversed: producer first.
export function displayEdges(trace: ExecutionTrace): DisplayEdge[] {
  return trace.edges
    .filter((edge) => edge.from !== edge.to)
    .map((edge) => ({
      id: `${edge.to}->${edge.from}`,
      source: edge.to,
      target: edge.from,
    }));
}

export function operatorDepths(trace: ExecutionTrace): Map<string, number> {
  const byId = new Map(trace.nodes.map((node) => [node.id, node]));
  const depths = new Map<string, number>();

  for (const node of trace.nodes) {
    let depth = 0;
    const visited = new Set<string>([node.id]);
    let parent = node.parentId ? byId.get(node.parentId) : undefined;

    while (parent && !visited.has(parent.id)) {
      visited.add(parent.id);
      depth += 1;
      parent = parent.parentId ? byId.get(parent.parentId) : undefined;
    }

    depths.set(node.id, depth);
  }

  return depths;
}

// The recorder emits nodes in pre-order, so the backend order is already the
// order an operator list should show: consumer first, its inputs below it.
export function dataFlowOrder(
  trace: ExecutionTrace,
): Array<{ node: ExecutionNode; depth: number }> {
  const depths = operatorDepths(trace);
  return trace.nodes.map((node) => ({
    node,
    depth: depths.get(node.id) ?? 0,
  }));
}

export function filteredOutPercent(node: ExecutionNode): number | null {
  if (node.kind !== "filter" || node.rowsIn <= 0) {
    return null;
  }
  return ((node.rowsIn - node.rowsOut) / node.rowsIn) * 100;
}

export function layoutPlan(trace: ExecutionTrace): PlanLayout {
  const edges = displayEdges(trace);
  if (trace.nodes.length === 0) {
    return { nodes: [], edges: [] };
  }

  const graph = new dagre.graphlib.Graph();
  graph.setDefaultEdgeLabel(() => ({}));
  graph.setGraph({
    rankdir: "TB",
    nodesep: 28,
    ranksep: 64,
    marginx: 16,
    marginy: 16,
  });

  for (const node of trace.nodes) {
    graph.setNode(node.id, {
      width: OPERATOR_NODE_WIDTH,
      height: OPERATOR_NODE_HEIGHT,
    });
  }

  for (const edge of edges) {
    graph.setEdge(edge.source, edge.target);
  }

  dagre.layout(graph);

  const depths = operatorDepths(trace);
  const nodes = trace.nodes.map((node) => {
    const position = graph.node(node.id);
    return {
      node,
      x: position.x - OPERATOR_NODE_WIDTH / 2,
      y: position.y - OPERATOR_NODE_HEIGHT / 2,
      depth: depths.get(node.id) ?? 0,
    };
  });

  return { nodes, edges };
}
