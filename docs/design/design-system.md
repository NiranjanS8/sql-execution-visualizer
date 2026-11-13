# Design system

## Direction: Instrumented Workbench

A technical workspace modelled on database consoles, code editors, and node-based debugging tools. The distinguishing element is a calm execution canvas holding compact operator cards. Everything around the graph stays quiet so the graph and the data remain the focus.

### Alternatives considered

| Direction | Description | Why it was rejected |
| --- | --- | --- |
| **Instrumented Workbench** (chosen) | Dark-first, dense, neutral surfaces, monospace technical metadata, restrained per-operator accents | — |
| Notebook Canvas | Light, document-first, cell-per-query, closer to a Jupyter notebook | Optimises for narrative and reproducibility, not for reading a plan. An operator graph inside a scrolling document has no stable home, and dense result grids fight the prose layout. |
| Dashboard Console | Card-based metrics above a chart area, light default | Encourages oversized metric cards and summary-first reading. QueryLens's value is the operator-level detail; a dashboard hierarchy buries it. |

### Rationale

The product's job is to make execution observable. That argues for a workbench: a persistent shell, resizable regions, and a large canvas that belongs to the graph. Dark-first is not decoration — it keeps a glowing operator selection and coloured operator families legible against low-luminance chrome, which is exactly the signal users scan for.

## Colour tokens

Implemented in `frontend/src/styles/tokens.css`. Components never use raw hex; they use the semantic names below.

| Token | Dark | Light | Use |
| --- | --- | --- | --- |
| `canvas` | `#0D1117` | `#F5F7FA` | Application background |
| `panel` | `#141A22` | `#FFFFFF` | Sidebars, bars, cards |
| `elevated` | `#1B2430` | `#FBFCFD` | Pills, inputs, raised rows |
| `hover` | `#222D3A` | `#EEF2F7` | Hover surface |
| `line` | `#2B3745` | `#DBE1EA` | Default border |
| `line-strong` | `#3A4859` | `#BCC6D4` | Separators, scrollbar thumb |
| `ink` | `#E6EDF3` | `#0F1620` | Primary text |
| `ink-muted` | `#9AA8B7` | `#4A5768` | Secondary text |
| `ink-faint` | `#718096` | `#6A7787` | Labels, metadata |
| `accent` | `#82A9FF` | `#2C5FD0` | Focus, primary action, selection |
| `success` | `#39C995` | `#0D7A56` | Completed, connected |
| `warning` | `#E8B45B` | `#8A5A05` | Truncation, caution |
| `danger` | `#F07878` | `#B02A22` | Failure, invalid input |

### Operator families

| Operator | Dark | Light |
| --- | --- | --- |
| Scan | `#6EA8FE` | `#1F5FBF` |
| Filter | `#E8B45B` | `#8A5A05` |
| Projection | `#A78BFA` | `#6B3FD4` |
| Join | `#4FD1C5` | `#0F6F68` |
| Aggregate | `#F472B6` | `#A8266B` |
| Sort / Limit | `#94A3B8` | `#4C5A6B` |
| Output | `#39C995` | `#0D7A56` |
| Failed | `#F07878` | `#B02A22` |

Colour is never the only signal. Every operator node carries its name and an icon; every status carries a text label.

## Typography

| Role | Stack | Size / weight |
| --- | --- | --- |
| UI text | Geist → Inter → system sans | 14px base, 1.5 line height |
| Section labels | same | 10px, 500, uppercase, `0.08em` tracking |
| SQL and identifiers | Geist Mono → IBM Plex Mono → Cascadia Code → Consolas | 13px |
| Metrics and counts | monospace | 13px |
| Page title | sans | 20px, 600, tight tracking |

Fonts resolve locally. No webfont is fetched, so the app works offline and cannot flash unstyled text.

## Spacing, radius, elevation

- Spacing scale: 4px increments (`1`, `2`, `3`, `4`, `6`, `10`).
- Radius: 4px (`xs`), 6px (`sm`), 8px (`md`), 12px (`lg`).
- Borders carry structure. Shadows appear only on floating surfaces (dialogs, menus, minimap).
- Hairline dividers are produced with `gap-px` over a `bg-line` container rather than borders, so adjacent cells never double up.

## Themes

`dark` (default), `light`, `system`. The preference persists in `localStorage` under `querylens.theme` and is applied as `data-theme` on `<html>`.

Light is designed, not inverted: paper-toned canvas, pure white panels, and accents darkened until they hold 4.5:1 against light surfaces.

## Component inventory

| Component | Layer | Status |
| --- | --- | --- |
| `Button` | ui | Built — Phase 0 |
| `Badge` | ui | Built — Phase 0 |
| `ThemeToggle` | ui | Built — Phase 0 |
| `AppShell` | layout | Built — Phase 0 (top bar, main, status strip) |
| `TopBar` | layout | Built — Phase 0 |
| `StatusStrip` | layout | Built — Phase 0 |
| `ConnectionStatus` | feedback | Built — Phase 0 |
| `HealthPanel` | feature | Built — Phase 0 |
| `ResizablePanelGroup` | layout | UI Phase B |
| `ExplorerSidebar` | layout | UI Phase B/C |
| `InspectorPanel` | layout | UI Phase B |
| `Tabs` | ui | UI Phase B |
| `Dialog` | ui | UI Phase C |
| `SqlEditor` | feature | UI Phase D |
| `ResultsGrid` | feature | UI Phase E |
| `ExecutionGraph` | feature | UI Phase F |
| `OperatorNode` | feature | UI Phase F |
| `OperatorInspector` | feature | UI Phase F |
| `OperatorList` | feature | UI Phase F (accessible alternative) |
| `QueryHistory` | feature | UI Phase G |
| `SettingsDialog` | feature | UI Phase G |

## Interaction states

Every interactive surface defines: default, hover, focus-visible, active, disabled, loading, empty, error.

| Surface | Loading | Empty | Error |
| --- | --- | --- | --- |
| Health check | `checking` label, muted dot | n/a | Red label, message, start command |
| Dataset list | Skeleton rows | "No datasets yet" + import action | Retry with message |
| Result grid | Row skeleton, header preserved | "Query returned no rows" | Last good result, marked stale |
| Execution graph | Node placeholders | "No plan — query did not execute" | Failed node in red, message in inspector |

## Motion

Transitions are 120–160ms and only communicate state: node selection, panel open/close, hover, spinner during fetch. Graph edges do not animate continuously. `prefers-reduced-motion` collapses all durations to near zero.

## Responsive behaviour

| Width | Layout |
| --- | --- |
| ≥1440px | Three regions, all resizable |
| 1024–1439px | Narrow explorer, collapsible inspector |
| 768–1023px | Explorer and inspector become drawers |
| <768px | Single work area; graph falls back to the operator list |

## Accessibility commitments

- Keyboard reachable controls with visible focus rings.
- Icon-only buttons carry `aria-label` and a `title`.
- Status changes announce through `aria-live="polite"`.
- Tables use real `<th>` elements with scope.
- The execution graph always has a list-view equivalent.
- Minimum contrast 4.5:1 for text, 3:1 for graphical objects.
- Layout remains usable at 200% zoom.
