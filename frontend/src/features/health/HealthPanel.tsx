import { Activity, RefreshCw } from "lucide-react";

import { Button } from "../../components/ui/Button";
import { apiBaseUrl } from "../../lib/api/client";
import { cn } from "../../lib/cn";
import { formatUptime } from "./formatUptime";
import { useHealth } from "./useHealth";

type StatTone = "default" | "success" | "danger" | "muted";

const statTones: Record<StatTone, string> = {
  default: "text-ink",
  success: "text-success",
  danger: "text-danger",
  muted: "text-ink-muted",
};

function Stat({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: string;
  tone?: StatTone;
}) {
  return (
    <div className="bg-panel px-4 py-3">
      <dt className="text-[10px] font-medium tracking-[0.08em] text-ink-faint uppercase">
        {label}
      </dt>
      <dd className={cn("mt-1 font-mono text-[13px]", statTones[tone])}>{value}</dd>
    </div>
  );
}

export function HealthPanel() {
  const { data, error, isFetching, refetch, status } = useHealth();

  const failed = status === "error";
  const connected = status === "success";

  return (
    <section
      aria-labelledby="health-heading"
      className="overflow-hidden rounded-lg border border-line bg-panel"
    >
      <header className="flex items-center gap-2 border-b border-line px-4 py-2.5">
        <Activity className="size-4 text-ink-muted" aria-hidden="true" />
        <h2 id="health-heading" className="text-[13px] font-medium">
          API connection
        </h2>
        <Button className="ml-auto" onClick={() => void refetch()} disabled={isFetching}>
          <RefreshCw className={cn("size-3.5", isFetching && "animate-spin")} aria-hidden="true" />
          Recheck
        </Button>
      </header>

      <dl className="grid grid-cols-2 gap-px bg-line sm:grid-cols-4">
        <Stat
          label="Status"
          value={failed ? "unreachable" : connected ? "connected" : "checking"}
          tone={failed ? "danger" : connected ? "success" : "muted"}
        />
        <Stat label="Service" value={data?.service ?? "—"} />
        <Stat label="Version" value={data ? `v${data.version}` : "—"} />
        <Stat label="Uptime" value={data ? formatUptime(data.uptimeMs) : "—"} />
      </dl>

      {failed ? (
        <div className="border-t border-line px-4 py-3">
          <p className="text-xs text-danger">
            {error instanceof Error ? error.message : "The API did not respond."}
          </p>
          <p className="mt-1 text-xs text-ink-faint">
            Start it with <code className="font-mono text-ink-muted">npm run dev:api</code>, then
            recheck. Expected at <span className="font-mono text-ink-muted">{apiBaseUrl}</span>.
          </p>
        </div>
      ) : null}
    </section>
  );
}
