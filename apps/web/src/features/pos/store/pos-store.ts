"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import type { Cents } from "@/lib/money";
import {
  addProduct,
  capNonCashPayment,
  computeTotals,
  fitsInStock,
  setQuantity,
  summarizeTender,
  type CartLine,
  type Payment,
  type PosPresentation,
  type PosProduct,
} from "../domain";
import { DEMO_SESSION } from "../data/session";

export type PosMode = "cart" | "tender" | "complete";

export interface CompletedSale {
  invoice: string;
  total: Cents;
  paid: Cents;
  change: Cents;
  itemCount: number;
}

/** Last line added, with a nonce so repeated adds retrigger the highlight. */
export interface AddedFlash {
  key: string;
  nonce: number;
}

interface PosState {
  lines: CartLine[];
  /** Products in the cart as last seen, used to check stock when quantities change. */
  products: Record<string, PosProduct>;
  ageVerified: boolean;
  mode: PosMode;
  payments: Payment[];
  lastSale: CompletedSale | null;
  nextInvoiceNumber: number;
  flash: AddedFlash | null;
  notice: string | null;

  addToCart: (product: PosProduct, presentation?: PosPresentation | null) => boolean;
  changeQuantity: (key: string, quantity: number) => boolean;
  clearCart: () => void;
  setAgeVerified: (verified: boolean) => void;
  setNotice: (notice: string | null) => void;
  startCheckout: () => void;
  backToCart: () => void;
  addPayment: (payment: Omit<Payment, "id">) => void;
  removePayment: (paymentId: string) => void;
  confirmSale: () => void;
  startNewSale: () => void;
}

const rate = DEMO_SESSION.exchangeRate;

const formatInvoice = (number: number) => `${DEMO_SESSION.invoiceSeries}-${String(number).padStart(6, "0")}`;

const emptySale = { lines: [], products: {}, payments: [], ageVerified: false, flash: null, notice: null };

export const usePosStore = create<PosState>()(
  persist(
    (set, get) => ({
      ...emptySale,
      mode: "cart",
      lastSale: null,
      nextInvoiceNumber: DEMO_SESSION.nextInvoiceNumber,

      addToCart: (product, presentation = null) => {
        const { lines, mode, products } = get();
        if (mode !== "cart") return false;
        const next = addProduct(lines, product, presentation);
        if (!next) return false;
        const key = next.find((l) => l.productId === product.id && l.presentationId === (presentation?.id ?? null))!.key;
        set({ lines: next, products: { ...products, [product.id]: product }, flash: { key, nonce: Date.now() }, notice: null });
        return true;
      },

      changeQuantity: (key, quantity) => {
        const { lines, products } = get();
        const line = lines.find((l) => l.key === key);
        if (!line) return false;
        const product = products[line.productId];
        const extraUnits = (quantity - line.quantity) * line.unitsPerItem;
        if (product && extraUnits > 0 && !fitsInStock(product, lines, extraUnits)) return false;
        set({ lines: setQuantity(lines, key, quantity) });
        return true;
      },

      clearCart: () => set(emptySale),

      setAgeVerified: (ageVerified) => set({ ageVerified }),

      setNotice: (notice) => set({ notice }),

      startCheckout: () => {
        const { lines, ageVerified } = get();
        if (lines.length === 0 || !ageVerified) return;
        set({ mode: "tender", payments: [] });
      },

      backToCart: () => set({ mode: "cart", payments: [] }),

      addPayment: (payment) => {
        const { lines, payments } = get();
        const { remaining } = summarizeTender(computeTotals(lines).total, payments, rate);
        if (remaining <= 0 || payment.amount <= 0) return;
        set({ payments: [...payments, capNonCashPayment({ ...payment, id: crypto.randomUUID() }, remaining, rate)] });
      },

      removePayment: (paymentId) => set({ payments: get().payments.filter((payment) => payment.id !== paymentId) }),

      // Local only until sales are persisted by the API (phase 5).
      confirmSale: () => {
        const { lines, payments, nextInvoiceNumber } = get();
        const totals = computeTotals(lines);
        const tender = summarizeTender(totals.total, payments, rate);
        if (!tender.isCovered) return;
        set({
          lastSale: { invoice: formatInvoice(nextInvoiceNumber), total: totals.total, paid: tender.paid, change: tender.change, itemCount: totals.itemCount },
          nextInvoiceNumber: nextInvoiceNumber + 1,
          mode: "complete",
        });
      },

      startNewSale: () => set({ ...emptySale, mode: "cart" }),
    }),
    {
      name: "anejo-pos-cart-v2",
      storage: createJSONStorage(() => localStorage),
      partialize: ({ lines, products, ageVerified }) => ({ lines, products, ageVerified }),
      skipHydration: true,
    },
  ),
);

export const POS_EXCHANGE_RATE = rate;
