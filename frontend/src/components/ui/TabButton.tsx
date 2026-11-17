import type { ReactNode } from "react";

import { cn } from "../../lib/cn";

type TabButtonProps = {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
};

export function TabButton({ active, onClick, children }: TabButtonProps) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "h-9 border-b-2 px-3 text-xs font-medium transition-colors",
        active ? "border-accent text-ink" : "border-transparent text-ink-muted hover:text-ink",
      )}
    >
      {children}
    </button>
  );
}
