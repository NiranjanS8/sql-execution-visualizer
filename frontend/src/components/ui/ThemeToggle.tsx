import { Monitor, Moon, Sun } from "lucide-react";

import { useTheme, type ThemePreference } from "../../app/providers/themeContext";
import { Button } from "./Button";

const order: ThemePreference[] = ["dark", "light", "system"];

const labels: Record<ThemePreference, string> = {
  dark: "Dark",
  light: "Light",
  system: "System",
};

const icons = {
  dark: Moon,
  light: Sun,
  system: Monitor,
};

export function ThemeToggle() {
  const { preference, setPreference } = useTheme();
  const next = order[(order.indexOf(preference) + 1) % order.length];
  const Icon = icons[preference];

  return (
    <Button
      onClick={() => setPreference(next)}
      aria-label={`Theme: ${labels[preference]}. Switch to ${labels[next].toLowerCase()}.`}
      title={`Theme: ${labels[preference]}`}
    >
      <Icon className="size-3.5" aria-hidden="true" />
      <span className="hidden sm:inline">{labels[preference]}</span>
    </Button>
  );
}
