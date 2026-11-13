import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import { ThemeContext, type ResolvedTheme, type ThemePreference } from "./themeContext";

const STORAGE_KEY = "querylens.theme";

function readStoredPreference(): ThemePreference {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return stored === "dark" || stored === "light" || stored === "system" ? stored : "dark";
}

function prefersLight(): boolean {
  return window.matchMedia("(prefers-color-scheme: light)").matches;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setStoredPreference] = useState<ThemePreference>(readStoredPreference);
  const [systemPrefersLight, setSystemPrefersLight] = useState(prefersLight);

  useEffect(() => {
    const query = window.matchMedia("(prefers-color-scheme: light)");
    const listener = (event: MediaQueryListEvent) => setSystemPrefersLight(event.matches);
    query.addEventListener("change", listener);
    return () => query.removeEventListener("change", listener);
  }, []);

  const resolved: ResolvedTheme =
    preference === "system" ? (systemPrefersLight ? "light" : "dark") : preference;

  useEffect(() => {
    document.documentElement.dataset.theme = resolved;
  }, [resolved]);

  const setPreference = useCallback((next: ThemePreference) => {
    setStoredPreference(next);
    window.localStorage.setItem(STORAGE_KEY, next);
  }, []);

  const value = useMemo(
    () => ({ preference, resolved, setPreference }),
    [preference, resolved, setPreference],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
