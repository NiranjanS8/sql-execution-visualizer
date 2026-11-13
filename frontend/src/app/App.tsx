import { AppShell } from "../components/layout/AppShell";
import { Workspace } from "../features/workspace/Workspace";

export function App() {
  return (
    <AppShell>
      <Workspace />
    </AppShell>
  );
}
