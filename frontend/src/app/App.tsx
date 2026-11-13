import { AppShell } from "../components/layout/AppShell";
import { FoundationOverview } from "../features/workspace/FoundationOverview";

export function App() {
  return (
    <AppShell>
      <FoundationOverview />
    </AppShell>
  );
}
