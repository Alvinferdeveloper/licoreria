"use client";

import { create } from "zustand";
import { ApiError } from "@/lib/api/client";
import { authApi, type SessionResponse, type SessionUser } from "./api";

type Status = "loading" | "authenticated" | "anonymous";

interface SessionState {
  status: Status;
  /** Kept in memory only; the refresh token lives in an HttpOnly cookie. */
  accessToken: string | null;
  user: SessionUser | null;
  setSession: (session: SessionResponse) => void;
  setUser: (user: SessionUser) => void;
  clear: () => void;
}

export const useSession = create<SessionState>()((set) => ({
  status: "loading",
  accessToken: null,
  user: null,
  setSession: ({ accessToken, user }) => set({ status: "authenticated", accessToken, user }),
  setUser: (user) => set({ user }),
  clear: () => set({ status: "anonymous", accessToken: null, user: null }),
}));

/** sessionStorage flag that makes the login screen explain the session closed for inactivity. */
export const IDLE_NOTICE_KEY = "anejo:idle-logout";

export function flagIdleLogout() {
  try {
    sessionStorage.setItem(IDLE_NOTICE_KEY, "1");
  } catch {}
}

let inFlight: Promise<string | null> | null = null;

/** Renews the session from the refresh cookie; concurrent callers share one request. */
export function refreshSession(): Promise<string | null> {
  inFlight ??= authApi
    .refresh()
    .then((session) => {
      useSession.getState().setSession(session);
      return session.accessToken;
    })
    .catch((error: unknown) => {
      if (error instanceof ApiError && error.code === "session_idle") flagIdleLogout();
      if (error instanceof ApiError && error.status === 401) useSession.getState().clear();
      else useSession.setState((state) => ({ status: state.user ? state.status : "anonymous" }));
      return null;
    })
    .finally(() => {
      inFlight = null;
    });
  return inFlight;
}

export async function logout() {
  try {
    await authApi.logout();
  } finally {
    useSession.getState().clear();
  }
}
