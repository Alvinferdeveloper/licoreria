"use client";

import { useEffect, useRef } from "react";

export type HotkeyMap = Partial<Record<string, (event: KeyboardEvent) => void>>;

/** Binds global keys (e.g. "F2", "Escape") for as long as the component is mounted. */
export function useHotkeys(bindings: HotkeyMap) {
  const bindingsRef = useRef(bindings);

  useEffect(() => {
    bindingsRef.current = bindings;
  });

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const handler = bindingsRef.current[event.key];
      if (!handler || event.repeat) return;
      // Focused fields and buttons already handle Enter (submit, click).
      if (event.key === "Enter" && event.target instanceof Element && event.target.closest("input, textarea, select, button, a")) return;
      event.preventDefault();
      handler(event);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
}
