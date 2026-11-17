# 0010 — Execution-plan visualization: React Flow, Dagre, and the trace as the only source

**Status:** Accepted
**Date:** 2026-11-17
**Phase:** 9

## Context

Phase 8 exposes the execution trace over `POST /api/v1/queries`. The trace already carries everything the graph needs: stable node IDs, parent/child links, operator kind, configuration label, output schema, rows in/out, exclusive duration, status, optional error, and bounded input/output samples. Phase 9 is a rendering problem, not an execution one.

The UI design plan names React Flow for the execution screen and requires zoom, pan, fit view, node selection, edge highlighting, an operator inspector, an accessible non-graph alternative, and large-plan handling. The product rule is stricter than the plan: the graph must show the backend plan, never a frontend approximation, and must remain readable without relying on colour.

The frontend also has to work at common viewport sizes, including a narrow window where a side inspector would squeeze the canvas to nothing.

## Decision

### React Flow 13 for the canvas, Dagre for layout

The graph uses `@xyflow/react` 13 for rendering, interaction, zoom/pan, and minimap. Layout is computed once per trace by `@dagrejs/dagre` 3 in `frontend/src/features/execution-plan/graph.ts`:

- `rankdir: "TB"` — data flows top-down, matching the wireframes.
- Fixed node size `248 × 92` (the operator card's real dimensions) so Dagre's rank spacing matches what React Flow renders.
- `nodesep: 28`, `ranksep: 64`; the result is centred by subtracting half the node size.

React Flow 13 is imported through its CSS layer. `globals.css` declares `@layer theme, base, xyflow, components, utilities;` before the Tailwind import, so React Flow's `xyflow` layer sits above Tailwind's base but below utilities. Component classes still win without `!important`.

### The trace is the only node and edge source

`layoutPlan(trace)` iterates `trace.nodes` in backend order. No node is synthesized, merged, or dropped. Node IDs are the backend IDs (`n1…nN`), so selection maps straight back to a trace node for the inspector.

Backend edges are consumer → producer (`from` is the parent that pulls, `to` is the child that feeds it). React Flow draws data from `source` to `target`, so `displayEdges` reverses each edge (`source = edge.to`, `target = edge.from`). Self-edges are dropped because a loop has no visual meaning here. Reversing in the view keeps the wire contract honest: the API keeps describing the pull direction, and only the presentation flips it.

### Node anatomy

`OperatorNode` is a custom React Flow node with `ariaRole: "button"` and no handles that can be dragged (`isConnectable={false}`). Each card shows:

- operator icon and name from `operatorMeta` (Scan, Filter, Project, Sort, Limit);
- the backend `label` (table, predicate, columns, order, or limit) as the configuration summary;
- `out N rows` in the header, then `in / out / duration` in a monospace metric row — a scan shows `in —`, never a fabricated zero;
- a status dot plus the status word (`completed`, `failed`, and so on), or a `CircleAlert` icon for a failed node.

Operator families use the existing `--ql-op-*` tokens. A failed node overrides the accent with the danger colour. Status is always written out, so the graph reads without colour.

### Selection, highlighting, and the inspector

Selection is owned by `QueryWorkbench` as `selectedNodeId` and passed down to both views. That keeps the graph, the list, and the inspector in sync and survives switching between graph and list. React Flow's own selection is mirrored into that state through `onSelectionChange`, `onNodeClick`, and `onPaneClick`; the component never treats its internal node state as the source of truth.

When a node is selected:

- its card gets a visible ring;
- edges touching it are redrawn with the accent colour, a thicker stroke, and an arrow marker, while unrelated edges dim to 35% opacity;
- `OperatorInspector` renders that trace node: status, metrics, configuration, input/output schemas, bounded samples, truncation notes, and any backend error.

The inspector's `filtered` percentage is the one derived number in the feature. It is computed only for filter nodes with `rowsIn > 0` as `(rowsIn − rowsOut) / rowsIn`. Every other number is read from the backend.

The selection is cleared when a run starts and the plan view is keyed by `queryRunId`, so a new run never inherits a stale selection or viewport.

### Graph and list are equal citizens

`ExecutionPlanView` offers a graph/list toggle. The list is the accessible alternative required by the design plan: backend pre-order, indentation derived from `parentId` depth, and real buttons with `aria-pressed`. It is not a fallback that appears only when something breaks; it is reachable at every size and is the default mental model for keyboard users.

### Plan states

- No result yet: "No plan yet" with an invitation to run, or a running indicator while the request is in flight.
- A result whose trace has no nodes: "No plan — the query did not execute."
- A retained plan after a failed run: the normal `StaleNotice`, because the graph still describes the last successful run.
- A trace-level error: the backend message in a danger block above the canvas.

### Large plans and controls

- The minimap appears only when the plan has more than 12 nodes, matching the wireframe threshold; below that it would add chrome without value.
- React Flow's own zoom controls sit bottom-left with interactive-lock hidden; fit view and reset layout are explicit panel buttons.
- `fitViewOptions.padding` is `"12%"`, not `12` — React Flow treats a bare number as pixels.
- Zoom is clamped to 0.25–1.75 so a wide plan cannot shrink into illegibility or grow into a maze.
- `deleteKeyCode={null}` and `nodesConnectable={false}` prevent the canvas from editing the backend plan.
- `proOptions={{ hideAttribution: true }}` removes the React Flow badge; the library is MIT-licensed and the app is local-only.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Hand-rolled SVG/DOM graph | Reimplements pan, zoom, hit-testing, keyboard focus, and minimap; React Flow is the named library and already solves these |
| ELK layout | Heavier and slower to integrate for a top-down tree; Dagre is sufficient for single-parent operator chains and is the smaller dependency |
| Lay out in the backend | Couples the wire contract to a renderer and makes the trace depend on presentation; the frontend owns pixels |
| Draw edges in backend direction | Data would appear to flow bottom-up, contradicting every database plan the product is teaching |
| Generate a plan in the frontend from the SQL | Forbidden: the graph must be the executed plan, not a re-parse |
| Keep selection inside `ExecutionGraph` | The list and inspector would disagree after a view switch; selection is workbench state |
| Always show the minimap | Visual noise on the common 3–5 node plans; the threshold keeps small plans calm |
| Put the inspector in a modal | Breaks the "select, then read" loop and hides the graph the user is reading |

## Consequences

- React Flow and Dagre are now production dependencies. The bundle grows to roughly 1 MB minified, dominated by CodeMirror and React Flow; Vite warns over 500 KB. Code-splitting is deferred.
- The frontend has one more place where a library's DOM assumptions matter. `frontend/src/test/setup.ts` shims `ResizeObserver`, `DOMMatrixReadOnly`, `getBBox`, and element offsets so React Flow renders under jsdom. These shims are test-only.
- The trace contract does not change. The UI reverses edges at render time and documents that it does so.
- `graph.ts` is pure and unit-tested independently of React: edge reversal, depth calculation, pre-order preservation, filter percentage, and layout ordering.
- The graph is not a live animation of a running query. It renders the completed trace; while a run is in flight the plan tab shows a running state. Streaming execution is out of scope.
- A very large plan remains a scroll/zoom exercise. The minimap and list view mitigate it, but there is no collapsing or clustering yet.
- Colour is never load-bearing: every node carries an icon and a name, and every status carries a word.
