# 0008 — Execution recorder: per-run IDs, metrics, and bounded samples

**Status:** Accepted
**Date:** 2026-11-15
**Phase:** 7

## Context

Phase 6 produces an inspectable logical plan and maps it to a pull-based operator chain, but nothing records what actually happened while that chain ran. Phase 7 must assign stable IDs to operators within a run, record durations and input/output row counts, capture bounded row samples, track errors and cancellation, record plan edges and the root identity, stay safe under concurrent query requests, avoid retaining full intermediate results, and define the units and sampling semantics.

The visualization needs real data, not estimates: the UI must never invent row counts, durations, or samples.

## Decision

Instrumentation lives in `internal/metrics`. `metrics.NewRecorder(plan, opts)` walks the logical plan, builds an instrumented operator chain, and returns a recorder whose `Operator()` can be drained. `metrics.Execute(ctx, plan, opts)` is a convenience wrapper that drains the operator, returns the result rows, and finishes the trace.

The recorder calls the newly exported `planner.BuildOperator(node, input)` for each node. `planner.BuildOperators(plan)` remains the uninstrumented path and builds the same chain recursively. This keeps one logical-to-physical mapping instead of duplicating the operator switch in `metrics`.

### IDs and graph shape

IDs are assigned in pre-order from the root: `n1` is the root, then each child subtree in data-flow order. The same plan shape always produces the same IDs, regardless of the table contents or the run. Nodes are reported in that order.

A `Trace` contains:

- `RootID` — the ID of the node whose rows are the result;
- `Nodes` — every operator, each with its ID, kind, label, parent ID, child IDs, output schema, metrics, status, error, and samples;
- `Edges` — parent-to-child plan edges;
- `Duration`, `Status`, and `Error` — the run as a whole.

All nodes are reachable from the root, and every non-root node has exactly one parent.

### Units and semantics

- **Duration** is wall-clock `time.Duration`. A node's duration is *exclusive*: the time spent inside its own `Next` calls minus the time spent waiting on its child. An attribution stack records nested calls, so the root does not absorb the whole pipeline and the per-node durations sum to roughly the run duration. The trace duration runs from the first operator call to `Finish`, excluding planning and recorder construction.
- **RowsIn** counts rows the node successfully pulled from its child. **RowsOut** counts rows the node successfully returned to its parent. Scan has no child, so its `RowsIn` is zero. A `LIMIT` that stops early therefore shows `RowsIn == RowsOut == limit` while its input shows more `RowsOut` than was consumed.
- **Statuses** describe what each operator observed:
  - `not_started` — `Next` was never called;
  - `running` — `Next` was called and has not reached a terminal state;
  - `completed` — `Next` returned `io.EOF`;
  - `stopped` — the operator started but an ancestor stopped pulling it;
  - `failed` — `Next` returned a non-EOF, non-cancellation error;
  - `cancelled` — `Next` observed context cancellation.

  `Finish` resolves any remaining `running` nodes to `stopped`, or to `cancelled` when the run ended in cancellation. A node that never started stays `not_started` even if a later node failed.

- **Samples** are copies of at most `Options.SampleLimit` rows at each enabled point. `CaptureInput` records rows entering an operator (the child's output); `CaptureOutput` records rows leaving it. The zero `Options` value records metrics without retaining rows. `DefaultOptions()` samples up to five rows at both points. `Truncated` is true when at least one row at that point was not retained, including when sampling is disabled but rows flowed.

Rows are copied when captured and copied again when the trace is snapshotted, so a trace cannot be changed by later table or operator state. Only bounded samples are retained; full intermediate results are never kept for visualization.

### Errors and cancellation

Operators return errors unchanged. The node that observed an error records it and is marked `failed` or `cancelled`; ancestors record the propagated error in the same way. Descendants that were never pulled stay `not_started`. This is honest about where the error surfaced without inventing a failure for operators that never ran.

`Finish(err)` accepts the error that ended the drain. If execution never started and the caller reports an error, the root is marked `failed` or `cancelled` and the trace still carries the error. `Finish` is idempotent and returns the same snapshot on later calls.

### Concurrency

A `Recorder` is owned by the goroutine that drains its operator and is not safe for concurrent use by multiple goroutines. Separate runs share no mutable state: each `NewRecorder` allocates its own node states, stack, samples, and trace. Concurrent query requests are therefore isolated by construction; the recorder does not introduce a global registry, lock, or shared counter.

### Result neutrality

Instrumentation wraps the same operators an uninstrumented run would use. Wrappers delegate `Schema` and pass rows through unchanged; they only count, time, and copy bounded samples. Tests execute the same plan with and without the recorder and compare every result row.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Put metrics inside each execution operator | Pollutes the operator contract and makes every operator know about traces |
| A global recorder or metrics registry | Shared mutable state, races, and cross-run contamination |
| Inclusive per-node durations | The root absorbs the whole pipeline; per-node numbers become misleading |
| Retain every intermediate row | Memory grows with the query, defeating the purpose of bounded samples |
| IDs derived from pointers or memory addresses | Not stable across runs or comparable between traces |
| A JSON/graph DTO in `metrics` | Transport and React Flow shapes belong to Phases 8 and 9 |
| Duplicate the logical-to-physical switch in `metrics` | Two mappings drift; `planner.BuildOperator` is the single source |

## Consequences

- Phase 8 can return the trace from the query endpoint after mapping it to JSON; Phase 9 can map `Nodes` and `Edges` to graph nodes.
- `planner.BuildOperator` is exported, and `planner.BuildOperators` is implemented in terms of the tree walk plus that function.
- Instrumentation adds a small per-row cost for timing, counters, and bounded copies. Sort still dominates memory because it materializes its input; the recorder does not add to that.
- A recorder must not be shared across goroutines. Concurrent requests are safe because each request owns one.
- `Trace` is an internal Go structure, not a stable wire format. Its JSON shape is decided in Phase 8.

## Tests

`backend/internal/metrics/recorder_test.go` covers:

- stable pre-order IDs, labels, parent/child links, edges, and graph completeness;
- row counts and statuses for an all-clause query, `LIMIT 0`, runtime failure, and cancellation before and after the first row;
- bounded samples, truncation, sample copies, one-sided sampling, and disabled sampling;
- result neutrality against uninstrumented execution for six queries;
- concurrent recorders staying isolated;
- recorder construction failures, schema preservation, `Finish` idempotence, and errors reported before execution.
