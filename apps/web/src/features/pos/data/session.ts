"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSession } from "@/features/auth/session-store";
import { cashApi, cashKeys } from "@/features/cash/api";
import { salesApi, salesKeys } from "@/features/sales/api";
import { todayISO } from "@/lib/format/date";
import { idb } from "@/lib/offline/idb";
import { isNetworkError } from "../offline/connectivity";

export interface PosSession {
  storeName: string;
  registerName: string;
  cashierName: string;
  /** NIO per USD for today. */
  exchangeRate: number;
  discountLimitPercent: number;
}

export type PosBlocker = "loading" | "error" | "no-shift" | "no-rate";

interface Snapshot {
  userId: string;
  day: string;
  session: PosSession;
}

const SNAPSHOT_KEY = "pos-session";

/**
 * What the POS needs to sell: the user's open shift, today's rate and the store rules.
 * The last good answer is kept on the device, so a lost connection doesn't stop the POS.
 */
export function usePosSession(): { session: PosSession | null; blocker: PosBlocker | null } {
  const user = useSession((s) => s.user);
  const shift = useQuery({ queryKey: cashKeys.current, queryFn: cashApi.current });
  const rate = useQuery({ queryKey: cashKeys.rateToday, queryFn: cashApi.rateToday });
  const store = useQuery({ queryKey: salesKeys.store, queryFn: salesApi.store, staleTime: 5 * 60_000 });
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);

  useEffect(() => {
    idb.get<Snapshot>("kv", SNAPSHOT_KEY).then((s) => setSnapshot(s ?? null), () => undefined);
  }, []);

  // A failed background refetch keeps the data it had.
  const ready = shift.data !== undefined && rate.data !== undefined && store.data !== undefined;
  const session: PosSession | null =
    ready && shift.data && rate.data
      ? {
          storeName: store.data.business.name,
          registerName: shift.data.register.name,
          cashierName: user?.fullName ?? "",
          exchangeRate: Number(rate.data.rate),
          discountLimitPercent: store.data.pos.discountLimitPercent,
        }
      : null;

  useEffect(() => {
    if (!session || !user) return;
    void idb.put("kv", { userId: user.id, day: todayISO(), session } satisfies Snapshot, SNAPSHOT_KEY).catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- save only when the values change
  }, [user?.id, session?.registerName, session?.exchangeRate, session?.discountLimitPercent, session?.storeName]);

  if (session) return { session, blocker: null };
  if (ready) return { session: null, blocker: !shift.data ? "no-shift" : "no-rate" };

  const networkDown = [shift, rate, store].some((q) => q.isError && isNetworkError(q.error));
  if (networkDown && snapshot && snapshot.userId === user?.id && snapshot.day === todayISO()) {
    return { session: snapshot.session, blocker: null };
  }
  if (shift.isError || rate.isError || store.isError) return { session: null, blocker: "error" };
  return { session: null, blocker: "loading" };
}
