"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { cashKeys } from "@/features/cash/api";
import { catalogKeys } from "@/features/catalog/api";
import { salesKeys } from "@/features/sales/api";
import { loadCatalog, refreshCatalog } from "./catalog-cache";
import { isNetworkError, probeServer, useConnectivity } from "./connectivity";
import { useOutbox } from "./outbox";

const PROBE_EVERY = 8_000;
const CATALOG_EVERY = 10 * 60_000;

/**
 * Keeps the POS ready to work without connection: loads the device's outbox and catalog,
 * notices when the server stops answering, and on reconnection sends the queued sales
 * and refreshes the catalog.
 */
export function useOfflineSync(userId: string | undefined) {
  const queryClient = useQueryClient();
  const online = useConnectivity((s) => s.online);
  const setOnline = useConnectivity((s) => s.setOnline);

  useEffect(() => {
    if (userId) void useOutbox.getState().load(userId);
    void loadCatalog();
  }, [userId]);

  useEffect(() => {
    const goOffline = () => setOnline(false);
    const check = async () => setOnline(await probeServer());
    window.addEventListener("offline", goOffline);
    window.addEventListener("online", check);
    return () => {
      window.removeEventListener("offline", goOffline);
      window.removeEventListener("online", check);
    };
  }, [setOnline]);

  useEffect(() => {
    if (online) return;
    const timer = setInterval(async () => {
      if (await probeServer()) setOnline(true);
    }, PROBE_EVERY);
    return () => clearInterval(timer);
  }, [online, setOnline]);

  useEffect(() => {
    if (!online || !userId) return;
    const refresh = () =>
      refreshCatalog().catch((error: unknown) => {
        if (isNetworkError(error)) setOnline(false);
      });
    void (async () => {
      const sent = await useOutbox.getState().sync(userId);
      if (sent > 0) {
        void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
        void queryClient.invalidateQueries({ queryKey: cashKeys.current });
        void queryClient.invalidateQueries({ queryKey: salesKeys.all });
      }
      await refresh();
    })();
    const timer = setInterval(refresh, CATALOG_EVERY);
    return () => clearInterval(timer);
  }, [online, userId, queryClient, setOnline]);
}
