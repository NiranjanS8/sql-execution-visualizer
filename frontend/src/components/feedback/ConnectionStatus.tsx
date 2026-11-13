import { useHealth } from "../../features/health/useHealth";
import { cn } from "../../lib/cn";

type ConnectionState = "checking" | "online" | "offline";

const presentation: Record<ConnectionState, { label: string; dot: string }> = {
  checking: { label: "Checking API", dot: "bg-ink-faint" },
  online: { label: "API connected", dot: "bg-success" },
  offline: { label: "API offline", dot: "bg-danger" },
};

export function ConnectionStatus() {
  const { status } = useHealth();

  const state: ConnectionState =
    status === "success" ? "online" : status === "error" ? "offline" : "checking";
  const { label, dot } = presentation[state];

  return (
    <span
      role="status"
      aria-live="polite"
      className="inline-flex items-center gap-1.5 rounded-full border border-line bg-elevated px-2 py-0.5 text-[11px] text-ink-muted"
    >
      <span className={cn("size-1.5 rounded-full", dot)} aria-hidden="true" />
      {label}
    </span>
  );
}
