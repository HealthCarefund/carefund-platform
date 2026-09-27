"use client";

import { useSyncExternalStore } from "react";

type Theme = "light" | "dark";

// A theme "store" external to React: the source of truth is
// document.documentElement's data-theme attribute (set synchronously by
// ThemeScript before hydration, so there is nothing to reconcile on
// mount) plus localStorage for persistence. useSyncExternalStore reads it
// safely across server/client without ever calling setState from an
// effect body.
const listeners = new Set<() => void>();

function subscribe(onChange: () => void): () => void {
  listeners.add(onChange);
  return () => listeners.delete(onChange);
}

function getSnapshot(): Theme {
  const attr = document.documentElement.getAttribute("data-theme");
  if (attr === "light" || attr === "dark") return attr;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

// The server has no DOM/localStorage; render a value that never causes a
// hydration mismatch in the *inert placeholder* this component renders
// before the client snapshot is available (see the render below).
function getServerSnapshot(): Theme {
  return "light";
}

function setTheme(next: Theme): void {
  document.documentElement.setAttribute("data-theme", next);
  try {
    window.localStorage.setItem("carefund-theme", next);
  } catch {
    // Per-viewer convenience only; if storage is unavailable the toggle
    // still works for the current page view.
  }
  for (const listener of listeners) listener();
}

export function ThemeToggle() {
  const theme = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  const isHydrated = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  );

  if (!isHydrated) {
    return (
      <button
        type="button"
        aria-hidden="true"
        tabIndex={-1}
        className="h-9 w-9 rounded-full border border-[var(--border-subtle)]"
      />
    );
  }

  return (
    <button
      type="button"
      onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
      aria-pressed={theme === "dark"}
      className="focus-ring flex h-9 w-9 items-center justify-center rounded-full border border-[var(--border-subtle)] text-ink-600 transition-colors hover:bg-[var(--surface)] dark:text-ink-300"
    >
      <span aria-hidden="true">{theme === "dark" ? "☀" : "☽"}</span>
      <span className="sr-only">Switch to {theme === "dark" ? "light" : "dark"} theme</span>
    </button>
  );
}
