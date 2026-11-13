# Wireframes

Low-fidelity layouts for the four primary surfaces. These fix structure, not pixels; the design system fixes the visual treatment.

## 1. Workspace

The default screen. Three regions around a persistent top bar and status strip.

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│ ◈ QueryLens   visual SQL execution            ● API connected   ☾ Dark      │ 48px
├───────────────┬──────────────────────────────────────────────┬───────────────┤
│ EXPLORER      │ query.sql                            ⌘↵ Run   │ INSPECTOR     │
│               ├──────────────────────────────────────────────┤               │
│ ▾ employees   │                                              │ Select an     │
│   employees   │  1  SELECT name, salary                      │ operator to   │
│   departments │  2  FROM employees                          │ inspect its   │
│               │  3  WHERE salary > 50000                     │ metrics.      │
│ ▸ orders      │  4  ORDER BY salary DESC                     │               │
│               │  5  LIMIT 100                                │               │
│ SAVED QUERIES │                                              │               │
│  top earners  ├──────────────────────────────────────────────┤               │
│  headcount    │ Results │ Execution plan │ Messages           │               │
│               ├──────────────────────────────────────────────┤               │
│ + Import CSV  │ ┌─────────┬──────────┬────────┐              │               │
│               │ │ name    │ salary   │ dept   │              │               │
│               │ │ text    │ integer  │ text   │              │               │
│               │ ├─────────┼──────────┼────────┤              │               │
│               │ │ Ada     │ 145000   │ Eng    │              │               │
│               │ │ Grace   │ 132000   │ Eng    │              │               │
│               │ └─────────┴──────────┴────────┘              │               │
│               │ 2 rows · 4.1 ms · not truncated              │               │
├───────────────┴──────────────────────────────────────────────┴───────────────┤
│ ready · 127.0.0.1:8787 · v0.1.0-dev                        phase 9 · graph  │ 32px
└──────────────────────────────────────────────────────────────────────────────┘
```

**Behaviour**

- Explorer and inspector are resizable and collapsible; widths persist for the session.
- Editor/results split is draggable. Run stays visible in the editor header regardless of split.
- Tabs never scroll away with the results: the tab strip is fixed above the panel body.
- Below 1024px the explorer and inspector become drawers triggered from the top bar.
- Status strip reports the real API state, never a hardcoded "ready".

## 2. Execution plan

The signature screen. Data flows top-down from sources to the result.

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│ Results │ Execution plan │ Messages          ⊞ graph  ☰ list   ⤢ fit  ⟲ reset│
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│                        ┌───────────────────────────┐                         │
│                        │ ▤ OUTPUT            3 rows│                         │
│                        │ result                    │                         │
│                        │ in 3 · out 3 · 0.1 ms     │                         │
│                        └─────────────┬─────────────┘                         │
│                                      │                                       │
│                        ┌─────────────┴─────────────┐                         │
│                        │ ↕ SORT              3 rows│                         │
│                        │ salary DESC               │                         │
│                        │ in 3 · out 3 · 0.3 ms     │                         │
│                        └─────────────┬─────────────┘                         │
│                                      │                                       │
│                        ┌─────────────┴─────────────┐                         │
│                        │ ▽ FILTER            3 rows│                         │
│                        │ salary > 50000            │                         │
│                        │ in 6 · out 3 · 1.2 ms     │                         │
│                        └─────────────┬─────────────┘                         │
│                                      │                                       │
│                        ┌─────────────┴─────────────┐                         │
│                        │ ▤ SCAN              6 rows│                         │
│                        │ employees                 │                         │
│                        │ in — · out 6 · 0.8 ms     │                         │
│                        └───────────────────────────┘                         │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Node anatomy**

```text
┌───────────────────────────────────────┐
│ ▽ FILTER                        3 rows│  ← icon + operator name + output count
│ salary > 50000                        │  ← configuration summary (monospace)
│ in 6 · out 3 · 1.2 ms                 │  ← metrics row
└───────────────────────────────────────┘
   ↑ 2px left border in the operator family colour
```

**Behaviour**

- Node positions come from an automatic layout of the backend's edges. The frontend never invents nodes or metrics.
- Selecting a node highlights it and its connected edges, and loads the inspector.
- Keyboard: arrow keys move between nodes, Enter opens the inspector, Escape clears selection.
- List view renders the same nodes as an ordered, indented list — the accessible equivalent of the graph.
- Failed operators use the danger colour, an error icon, and the word "failed".
- Large plans: minimap appears above ~12 nodes; fit-to-view remains available.

## 3. Dataset explorer

```text
┌──────────────────────────────────────────────────────────────┐
│ EXPLORER                                        + Import CSV │
├──────────────────────────────────────────────────────────────┤
│ ▾ employees                                    3 tables      │
│   ▸ employees                         6 rows · 4 columns     │
│     ┌──────────┬─────────┬──────────┬──────────┐             │
│     │ column   │ type    │ nullable │ sample   │             │
│     ├──────────┼─────────┼──────────┼──────────┤             │
│     │ id       │ integer │ no       │ 1        │             │
│     │ name     │ text    │ no       │ Ada      │             │
│     │ salary   │ integer │ yes      │ 145000   │             │
│     │ dept_id  │ integer │ yes      │ 10       │             │
│     └──────────┴─────────┴──────────┴──────────┘             │
│     Preview rows 1–50 of 6              ‹ 1 ›                 │
│   ▸ departments                       4 rows · 3 columns     │
│   ▸ titles                           12 rows · 5 columns     │
│ ▸ orders                                      1 table        │
└──────────────────────────────────────────────────────────────┘
```

**Import flow**

```text
┌─ Import CSV ────────────────────────────────────────────┐
│ 1  Choose file        [ employees.csv ]   ✔             │
│ 2  Preview                                              │
│    ┌────────┬────────┬────────┐                         │
│    │ id     │ name   │ salary │   ← headers from file   │
│    │ 1      │ Ada    │ 145000 │                         │
│    │ 2      │ Grace  │ 132000 │                         │
│    └────────┴────────┴────────┘                         │
│ 3  Table name         [ employees        ]              │
│ 4  Inferred types                                       │
│    id → integer · name → text · salary → integer        │
│    ⚠ salary has 2 empty values, imported as NULL        │
│                                                         │
│                              [ Cancel ]  [ Import ]     │
└─────────────────────────────────────────────────────────┘
```

**Behaviour**

- Type inference is shown before commit and is never silently applied.
- Ambiguous columns are called out with the reason, not just a warning icon.
- Import errors name the row and column: `Row 47: expected 4 fields, found 5.`
- The preview is paginated; it never loads an entire file into the DOM.

## 4. Operator inspector

```text
┌──────────────────────────────────────────────┐
│ ▽ Filter                        ● completed  │
│ OPERATOR · filter-002                        │
├──────────────────────────────────────────────┤
│   6        3         1.2 ms       50.0%      │
│   input    output    duration     filtered   │
├──────────────────────────────────────────────┤
│ CONFIGURATION                                │
│   predicate   salary > 50000                 │
├──────────────────────────────────────────────┤
│ SCHEMA                                       │
│   in    employees(id, name, salary, dept_id) │
│   out   same                                 │
├──────────────────────────────────────────────┤
│ OUTPUT SAMPLE                    3 of 3 rows │
│   ┌────────┬──────────┐                      │
│   │ name   │ salary   │                      │
│   │ Ada    │ 145000   │                      │
│   │ Grace  │ 132000   │                      │
│   └────────┴──────────┘                      │
│   sample complete                            │
├──────────────────────────────────────────────┤
│ ⚠ 1 warning                                  │
│   Sample limited to 50 rows per operator.    │
└──────────────────────────────────────────────┘
```

**Behaviour**

- Every value comes from the backend response for that operator. Missing metrics render as `—`, never as a plausible-looking zero.
- Truncated samples say so, with the configured limit.
- The panel is scrollable and keeps its header visible.
- When nothing is selected it shows a short instruction, not an empty box.

## State coverage

| Surface | Empty | Loading | Error |
| --- | --- | --- | --- |
| Workspace | No dataset selected → prompt to import or open a sample | Run button shows progress, cancel replaces it | Messages tab gains a badge; last result marked stale |
| Execution plan | "No plan yet" with the reason | Node skeletons in layout order | Failed node highlighted; inspector shows the error |
| Explorer | "No datasets" + import action | Skeleton rows | Inline message with retry |
| Inspector | Instruction text | Metric placeholders | Error block with code and message |
