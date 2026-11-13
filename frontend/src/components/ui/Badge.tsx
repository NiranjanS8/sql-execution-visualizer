import type { ReactNode } from "react";

import { cn } from "../../lib/cn";

type BadgeProps = {
  children: ReactNode;
  tone?: "neutral" | "accent" | "success" | "danger";
};

const tones = {
  neutral: "border-line bg-elevated text-ink-muted",
  accent: "border-accent/40 bg-accent/10 text-accent",
  success: "border-success/40 bg-success/10 text-success",
  danger: "border-danger/40 bg-danger/10 text-danger",
};

export function Badge({ children, tone = "accent" }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.08em]",
        tones[tone],
      )}
    >
      {children}
    </span>
  );
}
