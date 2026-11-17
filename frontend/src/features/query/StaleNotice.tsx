import { AlertTriangle } from "lucide-react";

export function StaleNotice() {
  return (
    <div
      role="status"
      className="flex items-center gap-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-xs text-warning"
    >
      <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
      Showing the last successful result. The latest run did not complete.
    </div>
  );
}
