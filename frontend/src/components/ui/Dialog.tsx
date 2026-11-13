import { useEffect, useRef, type ReactNode } from "react";
import { X } from "lucide-react";

import { cn } from "../../lib/cn";
import { Button } from "./Button";

type DialogProps = {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  size?: "md" | "lg";
};

export function Dialog({
  open,
  title,
  description,
  onClose,
  children,
  footer,
  size = "md",
}: DialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    panelRef.current?.focus();

    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/60" aria-hidden="true" onClick={onClose} />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        tabIndex={-1}
        className={cn(
          "relative flex max-h-[calc(100vh-2rem)] w-full flex-col overflow-hidden rounded-lg border border-line bg-panel shadow-xl",
          size === "lg" ? "max-w-3xl" : "max-w-xl",
        )}
      >
        <header className="flex items-start gap-3 border-b border-line px-4 py-3">
          <div className="min-w-0">
            <h2 id="dialog-title" className="text-[13px] font-semibold">
              {title}
            </h2>
            {description ? <p className="mt-0.5 text-xs text-ink-muted">{description}</p> : null}
          </div>
          <Button
            className="ml-auto shrink-0"
            onClick={onClose}
            aria-label="Close dialog"
            title="Close dialog"
          >
            <X className="size-3.5" aria-hidden="true" />
          </Button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">{children}</div>

        {footer ? (
          <footer className="flex justify-end gap-2 border-t border-line px-4 py-3">{footer}</footer>
        ) : null}
      </div>
    </div>
  );
}
