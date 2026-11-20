"use client";

import { create } from "zustand";
import { salesApi, type SaleInput } from "@/features/sales/api";
import { ApiError } from "@/lib/api/client";
import type { Cents, CurrencyCode } from "@/lib/money";
import { idb } from "@/lib/offline/idb";
import { isNetworkError, useConnectivity } from "./connectivity";

/** What the provisional receipt prints; the real invoice comes from the server on sync. */
export interface OfflineReceipt {
  code: string;
  createdAt: string;
  cashierName: string;
  registerName: string;
  customer: { name: string; taxId: string | null } | null;
  lines: { name: string; quantity: number; unitsPerItem: number; unitPrice: Cents; subtotal: Cents }[];
  subtotal: Cents;
  discount: Cents;
  tax: Cents;
  total: Cents;
  payments: { currency: CurrencyCode; amount: Cents }[];
  change: Cents;
  rate: number;
}

export interface OutboxSale {
  /** The sale's Idempotency-Key: sending it twice never charges twice. */
  key: string;
  userId: string;
  input: SaleInput;
  receipt: OfflineReceipt;
  status: "pending" | "failed";
  error: string | null;
  /** Invoice the server assigned, once synced. */
  invoice?: string;
}

/** Short code printed on the provisional receipt, taken from the sale key. */
export const receiptCode = (key: string) => `P-${key.slice(0, 8).toUpperCase()}`;

interface OutboxState {
  items: OutboxSale[];
  syncing: boolean;
  /** Sales synced in this session, so the cashier sees which invoice each one got. */
  synced: { code: string; invoice: string }[];
  load: (userId: string) => Promise<void>;
  enqueue: (sale: OutboxSale) => Promise<void>;
  sync: (userId: string) => Promise<number>;
  retry: (key: string) => Promise<void>;
  discard: (key: string) => Promise<void>;
}

const byDate = (a: OutboxSale, b: OutboxSale) => a.receipt.createdAt.localeCompare(b.receipt.createdAt);

export const useOutbox = create<OutboxState>()((set, get) => ({
  items: [],
  syncing: false,
  synced: [],

  load: async (userId) => {
    const all = await idb.all<OutboxSale>("outbox").catch(() => []);
    set({ items: all.filter((s) => s.userId === userId).sort(byDate) });
  },

  enqueue: async (sale) => {
    await idb.put("outbox", sale);
    set({ items: [...get().items, sale].sort(byDate) });
  },

  // Sends pending sales in the order they happened; stops at the first network failure.
  sync: async (userId) => {
    if (get().syncing) return 0;
    set({ syncing: true });
    let sent = 0;
    try {
      for (const sale of get().items.filter((s) => s.userId === userId && s.status === "pending")) {
        try {
          const created = await salesApi.create(sale.key, sale.input);
          await idb.delete("outbox", sale.key);
          sent++;
          set((s) => ({
            items: s.items.filter((i) => i.key !== sale.key),
            synced: [...s.synced, { code: sale.receipt.code, invoice: created.invoice }],
          }));
        } catch (error) {
          if (isNetworkError(error)) {
            useConnectivity.getState().setOnline(false);
            break;
          }
          const failed = { ...sale, status: "failed" as const, error: error instanceof ApiError ? error.message : String(error) };
          await idb.put("outbox", failed);
          set((s) => ({ items: s.items.map((i) => (i.key === sale.key ? failed : i)) }));
        }
      }
    } finally {
      set({ syncing: false });
    }
    return sent;
  },

  retry: async (key) => {
    const sale = get().items.find((i) => i.key === key);
    if (!sale) return;
    const pending = { ...sale, status: "pending" as const, error: null };
    await idb.put("outbox", pending);
    set((s) => ({ items: s.items.map((i) => (i.key === key ? pending : i)) }));
  },

  discard: async (key) => {
    await idb.delete("outbox", key);
    set((s) => ({ items: s.items.filter((i) => i.key !== key) }));
  },
}));
