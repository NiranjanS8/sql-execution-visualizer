import { useHealth } from "../../features/health/useHealth";
import { apiBaseUrl } from "../../lib/api/client";

function Separator() {
  return (
    <span aria-hidden="true" className="text-line-strong">
      ·
    </span>
  );
}

export function StatusStrip() {
  const { data, status } = useHealth();

  const label =
    status === "success" ? "ready" : status === "error" ? "API unreachable" : "connecting";

  return (
    <footer className="flex h-8 shrink-0 items-center gap-2 overflow-hidden border-t border-line bg-panel px-4 font-mono text-[11px] whitespace-nowrap text-ink-faint">
      <span className="text-ink-muted">{label}</span>
      <Separator />
      <span className="truncate">{apiBaseUrl}</span>
      {data ? (
        <>
          <Separator />
          <span>v{data.version}</span>
        </>
      ) : null}
      <span className="ml-auto hidden sm:inline">phase 6 · planner</span>
    </footer>
  );
}
