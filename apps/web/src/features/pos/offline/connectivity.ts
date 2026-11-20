"use client";

import { create } from "zustand";
import { ApiError, isUnreachable } from "@/lib/api/client";

interface ConnectivityState {
  /** False once a request to the server failed at the network level; true again after a probe answers. */
  online: boolean;
  setOnline: (online: boolean) => void;
}

export const useConnectivity = create<ConnectivityState>()((set) => ({
  online: true,
  setOnline: (online) => set({ online }),
}));

export const isNetworkError = (error: unknown) => error instanceof ApiError && error.status === 0;

/** Asks the API whether it is reachable; any answer from the API itself counts, even an error. */
export async function probeServer(): Promise<boolean> {
  try {
    const res = await fetch("/api/v1/health", { cache: "no-store", signal: AbortSignal.timeout(4000) });
    return !isUnreachable(res);
  } catch {
    return false;
  }
}
