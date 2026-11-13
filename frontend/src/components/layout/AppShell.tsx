import type { ReactNode } from "react";

import { StatusStrip } from "./StatusStrip";
import { TopBar } from "./TopBar";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="grid h-full grid-rows-[auto_minmax(0,1fr)_auto] bg-canvas text-ink">
      <TopBar />
      <main className="min-h-0 overflow-y-auto">{children}</main>
      <StatusStrip />
    </div>
  );
}
