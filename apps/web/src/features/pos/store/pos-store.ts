"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import type { Cents } from "@/lib/money";
import {
  addProduct,
  availableStock,
  capNonCashPayment,
  computeTotals,
  setQuantity,
  summarizeTender,
  type CartLine,
  type Payment,
  type Product,
} from "../domain";
import { PRODUCTS } from "../data/catalog";
import { DEMO_SESSION } from "../data/session";

export type PosMode = "cart" | "tender" | "complete";

export interface CompletedSale {
  invoice: string;
  total: Cents;
  paid: Cents;
  change: Cents;
  itemCount: number;
}

/** Last product added, with a nonce so repeated adds retrigger the highlight. */
export interface AddedFlash {
  productId: string;
  nonce: number;
}

interface PosState {
  products: Product[];
  lines: CartLine[];
  ageVerified: boolean;
  mode: PosMode;
  payments: Payment[];
  lastSale: CompletedSale | null;
  nextInvoiceNumber: number;
  flash: AddedFlash | null;
  notice: string | null;

  addToCart: (product: Product) => boolean;
  changeQuantity: (productId: string, quantity: number) => void;
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

const formatInvoice = (number: number) =>
  `${DEMO_SESSION.invoiceSeries}-${String(number).padStart(6, "0")}`;

const newId = () => crypto.randomUUID();

export const usePosStore = create<PosState>()(
  persist(
    (set, get) => ({
      products: [...PRODUCTS],
      lines: [],
      ageVerified: false,
      mode: "cart",
      payments: [],
      lastSale: null,
      nextInvoiceNumber: DEMO_SESSION.nextInvoiceNumber,
      flash: null,
      notice: null,

      addToCart: (product) => {
        const { lines, mode } = get();
        if (mode !== "cart" || availableStock(product, lines) <= 0) return false;
        set({
          lines: addProduct(lines, product),
          flash: { productId: product.id, nonce: Date.now() },
          notice: null,
        });
        return true;
      },

      changeQuantity: (productId, quantity) => {
        const product = get().products.find((item) => item.id === productId);
        const capped = product ? Math.min(quantity, product.stock) : quantity;
        set({ lines: setQuantity(get().lines, productId, capped) });
      },

      clearCart: () => set({ lines: [], ageVerified: false, payments: [], notice: null }),

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
        const { total } = computeTotals(lines);
        const { remaining } = summarizeTender(total, payments, rate);
        if (remaining <= 0 || payment.amount <= 0) return;
        const capped = capNonCashPayment({ ...payment, id: newId() }, remaining, rate);
        set({ payments: [...payments, capped] });
      },

      removePayment: (paymentId) =>
        set({ payments: get().payments.filter((payment) => payment.id !== paymentId) }),

      confirmSale: () => {
        const { lines, payments, products, nextInvoiceNumber } = get();
        const totals = computeTotals(lines);
        const tender = summarizeTender(totals.total, payments, rate);
        if (!tender.isCovered) return;

        const sold = new Map(lines.map((line) => [line.productId, line.quantity]));
        set({
          products: products.map((product) =>
            sold.has(product.id) ? { ...product, stock: product.stock - sold.get(product.id)! } : product,
          ),
          lastSale: {
            invoice: formatInvoice(nextInvoiceNumber),
            total: totals.total,
            paid: tender.paid,
            change: tender.change,
            itemCount: totals.itemCount,
          },
          nextInvoiceNumber: nextInvoiceNumber + 1,
          mode: "complete",
        });
      },

      startNewSale: () =>
        set({ lines: [], payments: [], ageVerified: false, mode: "cart", flash: null, notice: null }),
    }),
    {
      name: "anejo-pos-cart",
      storage: createJSONStorage(() => localStorage),
      partialize: ({ lines, ageVerified }) => ({ lines, ageVerified }),
      skipHydration: true,
    },
  ),
);

export const POS_EXCHANGE_RATE = rate;
