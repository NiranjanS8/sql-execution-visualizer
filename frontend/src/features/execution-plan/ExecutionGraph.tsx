import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  MiniMap,
  Panel,
  ReactFlow,
  useNodesState,
  type Edge,
  type OnSelectionChangeParams,
  type ReactFlowInstance,
} from "@xyflow/react";
import { Maximize2, RotateCcw } from "lucide-react";

import { Button } from "../../components/ui/Button";
import type { ExecutionTrace } from "../../types/query";
import { layoutPlan, type PlanLayout } from "./graph";
import { OperatorNode, type OperatorFlowNode } from "./OperatorNode";
import { operatorAccent } from "./operatorMeta";

type ExecutionGraphProps = {
  trace: ExecutionTrace;
  selectedNodeId: string | null;
  onSelectNode: (id: string | null) => void;
  active: boolean;
};

const nodeTypes = { operator: OperatorNode };
const fitViewOptions = { padding: "12%" as const };

function buildNodes(layout: PlanLayout, selectedNodeId: string | null): OperatorFlowNode[] {
  return layout.nodes.map(({ node, x, y }) => ({
    id: node.id,
    type: "operator",
    position: { x, y },
    data: { node },
    selected: node.id === selectedNodeId,
    ariaRole: "button",
  }));
}

export function ExecutionGraph({
  trace,
  selectedNodeId,
  onSelectNode,
  active,
}: ExecutionGraphProps) {
  const layout = useMemo(() => layoutPlan(trace), [trace]);
  const [nodes, setNodes, onNodesChange] = useNodesState<OperatorFlowNode>(
    buildNodes(layout, selectedNodeId),
  );
  const [flow, setFlow] = useState<ReactFlowInstance<OperatorFlowNode, Edge> | null>(null);

  const edges = useMemo<Edge[]>(
    () =>
      layout.edges.map((edge) => {
        const connected =
          selectedNodeId !== null &&
          (edge.source === selectedNodeId || edge.target === selectedNodeId);
        const dimmed = selectedNodeId !== null && !connected;
        return {
          ...edge,
          type: "smoothstep",
          markerEnd: {
            type: MarkerType.ArrowClosed,
            width: 14,
            height: 14,
            color: connected ? "var(--ql-accent)" : "var(--ql-line-strong)",
          },
          style: {
            stroke: connected ? "var(--ql-accent)" : "var(--ql-line-strong)",
            strokeWidth: connected ? 2 : 1.5,
            opacity: dimmed ? 0.35 : 1,
          },
        };
      }),
    [layout.edges, selectedNodeId],
  );

  // Selection is owned by the query workbench so the graph, list, and inspector
  // always agree. React Flow still handles keyboard selection internally.
  useEffect(() => {
    setNodes((current) => {
      let changed = false;
      const next = current.map((node) => {
        const selected = node.id === selectedNodeId;
        if (node.selected === selected) {
          return node;
        }
        changed = true;
        return { ...node, selected };
      });
      return changed ? next : current;
    });
  }, [selectedNodeId, setNodes]);

  useEffect(() => {
    if (!active || !flow) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      void flow.fitView({ ...fitViewOptions, duration: 0 });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [active, flow]);

  const handleSelectionChange = useCallback(
    ({ nodes: selected }: OnSelectionChangeParams<OperatorFlowNode>) => {
      onSelectNode(selected[0]?.id ?? null);
    },
    [onSelectNode],
  );

  const resetLayout = useCallback(() => {
    setNodes(buildNodes(layout, selectedNodeId));
    void flow?.fitView({ ...fitViewOptions, duration: 200 });
  }, [flow, layout, selectedNodeId, setNodes]);

  return (
    <div className="h-full w-full">
      <ReactFlow<OperatorFlowNode, Edge>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        onNodeClick={(_, node) => onSelectNode(node.id)}
        onPaneClick={() => onSelectNode(null)}
        onSelectionChange={handleSelectionChange}
        onInit={setFlow}
        fitView
        fitViewOptions={fitViewOptions}
        minZoom={0.25}
        maxZoom={1.75}
        nodesConnectable={false}
        edgesReconnectable={false}
        edgesFocusable={false}
        deleteKeyCode={null}
        proOptions={{ hideAttribution: true }}
        aria-label="Execution plan graph"
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--ql-line)" />
        <Controls showInteractive={false} showFitView={false} position="bottom-left" />

        {layout.nodes.length > 12 ? (
          <MiniMap
            pannable
            zoomable
            position="bottom-right"
            ariaLabel="Plan minimap"
            nodeColor={(node) => {
              const data = node.data as OperatorFlowNode["data"] | undefined;
              return data?.node ? operatorAccent(data.node) : "var(--ql-line-strong)";
            }}
          />
        ) : null}

        <Panel position="top-right" className="flex items-center gap-1">
          <Button
            size="sm"
            aria-label="Fit view"
            title="Fit view"
            className="bg-panel"
            onClick={() => void flow?.fitView({ ...fitViewOptions, duration: 200 })}
          >
            <Maximize2 className="size-3.5" aria-hidden="true" />
          </Button>
          <Button
            size="sm"
            aria-label="Reset layout"
            title="Reset layout"
            className="bg-panel"
            onClick={resetLayout}
          >
            <RotateCcw className="size-3.5" aria-hidden="true" />
          </Button>
        </Panel>
      </ReactFlow>
    </div>
  );
}
