const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

/** Formats an uptime in milliseconds as a compact human duration. */
export function formatUptime(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return "—";
  }
  if (ms < MINUTE) {
    return `${Math.floor(ms / 1000)}s`;
  }
  if (ms < HOUR) {
    return `${Math.floor(ms / MINUTE)}m ${Math.floor((ms % MINUTE) / 1000)}s`;
  }
  return `${Math.floor(ms / HOUR)}h ${Math.floor((ms % HOUR) / MINUTE)}m`;
}
