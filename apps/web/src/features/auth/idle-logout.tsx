"use client";

import { useEffect } from "react";
import { useConnectivity } from "@/features/pos/offline/connectivity";
import { flagIdleLogout, logout, useSession } from "./session-store";

const ACTIVITY_KEY = "anejo:last-activity";
const EVENTS = ["pointerdown", "keydown", "wheel", "touchstart", "mousemove"] as const;

function readActivity(): number {
  try {
    return Number(localStorage.getItem(ACTIVITY_KEY)) || Date.now();
  } catch {
    return Date.now();
  }
}

/** Closes the session after the configured minutes without mouse, keyboard or touch, in every tab. */
export function IdleLogout() {
  const minutes = useSession((s) => (s.status === "authenticated" ? (s.user?.idleTimeoutMinutes ?? 0) : 0));

  useEffect(() => {
    if (minutes <= 0) return;
    let last = 0;
    // Writes at most every 5 s; the shared key lets activity in one tab keep the others open.
    const mark = () => {
      const now = Date.now();
      if (now - last < 5_000) return;
      last = now;
      try {
        localStorage.setItem(ACTIVITY_KEY, String(now));
      } catch {}
    };
    mark();
    for (const e of EVENTS) window.addEventListener(e, mark, { passive: true });

    const timer = window.setInterval(() => {
      // Without connection nobody could sign back in, so an offline counter keeps its session.
      if (!useConnectivity.getState().online) return;
      if (Date.now() - Math.max(readActivity(), last) < minutes * 60_000) return;
      flagIdleLogout();
      void logout();
    }, 15_000);

    return () => {
      window.clearInterval(timer);
      for (const e of EVENTS) window.removeEventListener(e, mark);
    };
  }, [minutes]);

  return null;
}
