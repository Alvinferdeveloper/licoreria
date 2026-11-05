"use client";

import { useQuery } from "@tanstack/react-query";
import { useSession } from "@/features/auth/session-store";
import { cashApi, cashKeys } from "@/features/cash/api";
import { salesApi, salesKeys } from "@/features/sales/api";

export interface PosSession {
  storeName: string;
  registerName: string;
  cashierName: string;
  /** NIO per USD for today. */
  exchangeRate: number;
}

export type PosBlocker = "loading" | "error" | "no-shift" | "no-rate";

/** What the POS needs to sell: the user's open shift, today's rate and the store name. */
export function usePosSession(): { session: PosSession | null; blocker: PosBlocker | null } {
  const user = useSession((s) => s.user);
  const shift = useQuery({ queryKey: cashKeys.current, queryFn: cashApi.current });
  const rate = useQuery({ queryKey: cashKeys.rateToday, queryFn: cashApi.rateToday });
  const store = useQuery({ queryKey: salesKeys.store, queryFn: salesApi.store, staleTime: 5 * 60_000 });

  if (shift.isPending || rate.isPending || store.isPending) return { session: null, blocker: "loading" };
  if (shift.isError || rate.isError || store.isError) return { session: null, blocker: "error" };
  if (!shift.data) return { session: null, blocker: "no-shift" };
  if (!rate.data) return { session: null, blocker: "no-rate" };
  return {
    session: {
      storeName: store.data.business.name,
      registerName: shift.data.register.name,
      cashierName: user?.fullName ?? "",
      exchangeRate: Number(rate.data.rate),
    },
    blocker: null,
  };
}
