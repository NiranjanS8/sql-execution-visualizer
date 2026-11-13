import { Badge } from "../../components/ui/Badge";
import { HealthPanel } from "../health/HealthPanel";

const upcoming = [
  {
    phase: "01",
    title: "Typed values and in-memory catalog",
    detail: "Schemas, tables, stable identifiers, sample datasets",
  },
  {
    phase: "02",
    title: "CSV import and dataset browser",
    detail: "Conservative type inference, previews, pagination",
  },
  {
    phase: "03",
    title: "SQL parser and validation",
    detail: "Supported subset, name resolution, structured errors",
  },
];

export function FoundationOverview() {
  return (
    <div className="mx-auto w-full max-w-3xl px-6 py-10">
      <div className="flex items-center gap-2">
        <Badge>Phase 0</Badge>
        <span className="text-[11px] text-ink-faint">engineering foundation</span>
      </div>

      <h1 className="mt-3 text-xl font-semibold tracking-tight">Execution workbench</h1>
      <p className="mt-1.5 max-w-xl text-[13px] leading-relaxed text-ink-muted">
        The shell, the API, and the design tokens are in place. Datasets, SQL parsing, and the
        execution graph arrive in the phases below.
      </p>

      <div className="mt-6">
        <HealthPanel />
      </div>

      <section aria-labelledby="next-up-heading" className="mt-6">
        <h2
          id="next-up-heading"
          className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase"
        >
          Next up
        </h2>
        <ol className="mt-2 divide-y divide-line overflow-hidden rounded-lg border border-line bg-panel">
          {upcoming.map((item) => (
            <li key={item.phase} className="flex items-baseline gap-4 px-4 py-3">
              <span className="font-mono text-[11px] text-ink-faint">{item.phase}</span>
              <div className="min-w-0">
                <p className="text-[13px] text-ink">{item.title}</p>
                <p className="mt-0.5 text-xs text-ink-faint">{item.detail}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>
    </div>
  );
}
