# Design references

What each reference was studied for, and what was actually taken from it. Nothing is copied wholesale; no proprietary assets are used.

| Reference | Studied for | Taken |
| --- | --- | --- |
| [Supabase Studio](https://supabase.com/dashboard) | Database navigation, table browsing, schema presentation | Explorer hierarchy: dataset → table → column, with types visible without opening a detail pane |
| [Neon Console](https://console.neon.tech/) | Developer console layout, information hierarchy | Restrained chrome; one accent colour; technical metadata in monospace |
| [Linear](https://linear.app/) | Spacing, typography, keyboard navigation | Compact 32–48px bars, tight type scale, focus states that are visible without being loud |
| [Retool](https://retool.com/) | Dense workbench composition, inspectors | Three-region layout with a persistent contextual inspector |
| [React Flow](https://reactflow.dev/examples) | Custom nodes, edge selection, zoom, layout | Node anatomy (icon, name, metric row) and the selection-driven inspector pattern |
| [shadcn/ui](https://ui.shadcn.com/blocks) | Component composition | Composition approach for dialogs, tabs, and resizable panels — adapted to QueryLens tokens, not imported as a theme |
| [Beekeeper Studio](https://www.beekeeperstudio.io/) | SQL editor layout | Editor above results, run action adjacent to the editor header |
| [Jam SQL Studio](https://jamsql.com/quick-tour/) | Workspace organisation | Sidebar for saved queries and datasets without leaving the workspace |
| [CH-UI](https://ch-ui.com/) | Dataset exploration | Column type badges and row-count affordances |
| [DBWillow](https://dbwillow.com/) | Query history | History as a compact list with status and duration, not a separate page |
| [Chat2DB](https://github.com/OtterMind/Chat2DB) | Multi-panel database client | Panel resizing behaviour and collapse affordances |
| [Greybeam: Snowflake query optimization](https://www.greybeam.ai/blog/snowflake-query-optimization) | Query plan presentation | Reading a plan top-down as data flow, with cost visible per node |
| [Uyava: Where are the maps for code?](https://uyava.io/blog/where-are-the-maps-for-code/) | Graph plus inspector | Keeping the inspector tied to selection rather than navigation |

## Skills and guidance consulted

| Resource | Used for |
| --- | --- |
| [Anthropic Frontend Design](https://github.com/anthropics/skills/tree/main/skills/frontend-design) | Avoiding templated dashboard aesthetics; committing to one visual direction |
| [Vercel Agent Skills](https://github.com/vercel-labs/agent-skills) | React structure and rendering performance |
| [SvaraNova Skills](https://github.com/SvaraNova/skills) | Motion and visualisation workflow |
| [TanStack Table](https://tanstack.com/table/latest/docs/framework/react/quick-start) | Result grid headless model |
| [Monaco Editor](https://microsoft.github.io/monaco-editor/) | SQL editor evaluation (Phase D) |

## Deliberately not borrowed

- Gradient hero panels, glassmorphism, and glow effects.
- Oversized KPI cards.
- Purple-on-white SaaS palettes.
- Decorative continuous animation on graph edges.
- Any layout that implies a hosted account, team workspace, or billing surface — QueryLens is local and single-user.

## Library decisions

| Need | Choice | Note |
| --- | --- | --- |
| Styling | Tailwind CSS v4 | CSS-first tokens; no config file to drift from the design system |
| Icons | Lucide | One library, consistent stroke weight |
| Server state | TanStack Query | Added in Phase 0 with the first real request |
| Graph | React Flow 13 | Phase 9; custom operator nodes, selection-driven inspector, built-in pan/zoom/minimap |
| Graph layout | Dagre 3 | Phase 9; top-down single-parent operator chains; ELK judged heavier than needed |
| Editor | CodeMirror 6 | Chosen in Phase 8; SQL highlighting and `Mod-Enter`, lighter than Monaco for this scope |
| Tables | Hand-rolled `DataGrid` | Phase 8; the grid is fixed-column and small enough that TanStack Table would add weight without benefit. Revisit if sorting or column resizing arrives |
