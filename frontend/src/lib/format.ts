export function formatDuration(milliseconds: number): string {
  if (milliseconds < 1) {
    return "<1 ms";
  }
  if (milliseconds < 1000) {
    return `${milliseconds.toFixed(1)} ms`;
  }
  return `${(milliseconds / 1000).toFixed(2)} s`;
}

export function formatCount(value: number): string {
  return value.toLocaleString("en-US");
}
