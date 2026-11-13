import { ConnectionStatus } from "../feedback/ConnectionStatus";
import { ThemeToggle } from "../ui/ThemeToggle";

export function TopBar() {
  return (
    <header className="flex h-12 shrink-0 items-center gap-3 border-b border-line bg-panel px-4">
      <div className="flex items-center gap-2.5">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-elevated">
          <svg viewBox="0 0 16 16" className="size-3.5 text-accent" aria-hidden="true" fill="none">
            <path
              d="M3.5 3.5v9M3.5 8h5M8.5 4.5v7"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
        </span>
        <span className="text-[13px] font-semibold tracking-tight">QueryLens</span>
        <span className="hidden text-[11px] text-ink-faint md:inline">visual SQL execution</span>
      </div>

      <div className="ml-auto flex items-center gap-2">
        <ConnectionStatus />
        <ThemeToggle />
      </div>
    </header>
  );
}
