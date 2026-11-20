"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { salesApi, type Authorization, type SaleInput } from "@/features/sales/api";
import { ApiError } from "@/lib/api/client";
import { amountToCents, toAmountString, type Cents } from "@/lib/money";
import {
  addProduct,
  capNonCashPayment,
  computeTotals,
  fitsInStock,
  setQuantity,
  summarizeTender,
  type CartLine,
  type Customer,
  type Discount,
  type HeldSale,
  type Payment,
  type PosPresentation,
  type PosProduct,
} from "../domain";
import { posCopy } from "../copy";
import { discountCachedStock } from "../offline/catalog-cache";
import { isNetworkError, useConnectivity } from "../offline/connectivity";
import { buildOfflineSale, unitsByProduct, type OfflineContext } from "../offline/offline-sale";
import { useOutbox, type OfflineReceipt } from "../offline/outbox";

export type PosMode = "cart" | "tender" | "complete";

export interface CompletedSale {
  id: string;
  invoice: string;
  total: Cents;
  paid: Cents;
  change: Cents;
  itemCount: number;
  customerEmail: string | null;
  /** Set when the sale was saved without connection; it prints as a provisional receipt. */
  offline?: OfflineReceipt;
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
  discount: Discount | null;
  /** Null sells to the walk-in "Consumidor final". */
  customer: Customer | null;
  ageVerified: boolean;
  held: HeldSale[];
  mode: PosMode;
  payments: Payment[];
  /** NIO per USD, set from today's rate when the POS loads. */
  rate: number;
  /** Idempotency key of the sale being charged; kept across retries of the same sale. */
  saleKey: string | null;
  submitting: boolean;
  /** The API asked for an administrator PIN (discount over the role limit). */
  needsAuthorization: boolean;
  error: string | null;
  lastSale: CompletedSale | null;
  flash: AddedFlash | null;
  notice: string | null;
  /** Seller and rules to apply if a sale must be saved without connection. */
  offlineContext: OfflineContext | null;

  setRate: (rate: number) => void;
  setOfflineContext: (context: OfflineContext) => void;
  addToCart: (product: PosProduct, presentation?: PosPresentation | null) => boolean;
  changeQuantity: (key: string, quantity: number) => boolean;
  clearCart: () => void;
  setDiscount: (discount: Discount | null) => void;
  setCustomer: (customer: Customer | null) => void;
  setAgeVerified: (verified: boolean) => void;
  setNotice: (notice: string | null) => void;
  holdSale: () => boolean;
  resumeSale: (id: string) => void;
  discardHeld: (id: string) => void;
  startCheckout: () => void;
  backToCart: () => void;
  addPayment: (payment: Omit<Payment, "id">) => void;
  removePayment: (paymentId: string) => void;
  confirmSale: (authorization?: Authorization) => Promise<void>;
  startNewSale: () => void;
}

const emptySale = {
  lines: [],
  products: {},
  discount: null,
  customer: null,
  payments: [],
  ageVerified: false,
  saleKey: null,
  needsAuthorization: false,
  error: null,
  flash: null,
  notice: null,
};

/** The cart as the sales API expects it; prices are not sent, the API takes them from the catalog. */
export function toSaleInput(state: Pick<PosState, "lines" | "discount" | "customer" | "payments" | "ageVerified">, authorization: Authorization | null): SaleInput {
  return {
    customerId: state.customer?.id ?? null,
    ageVerified: state.ageVerified,
    notes: "",
    discount: state.discount
      ? { kind: state.discount.kind, value: state.discount.kind === "percent" ? state.discount.value.toFixed(2) : toAmountString(state.discount.value) }
      : null,
    authorization,
    lines: state.lines.map((l) => ({ productId: l.productId, presentationId: l.presentationId, quantity: l.quantity })),
    payments: state.payments.map((p) => ({ method: p.method, currency: p.currency, amount: toAmountString(p.amount), reference: null })),
  };
}

export const usePosStore = create<PosState>()(
  persist(
    (set, get) => ({
      ...emptySale,
      held: [],
      mode: "cart",
      rate: 0,
      submitting: false,
      lastSale: null,
      offlineContext: null,

      setRate: (rate) => set({ rate }),

      setOfflineContext: (offlineContext) => set({ offlineContext }),

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

      setDiscount: (discount) => set({ discount }),

      setCustomer: (customer) => set({ customer }),

      setAgeVerified: (ageVerified) => set({ ageVerified }),

      setNotice: (notice) => set({ notice }),

      holdSale: () => {
        const { lines, products, discount, customer, held, mode } = get();
        if (mode !== "cart" || lines.length === 0) return false;
        set({ ...emptySale, held: [...held, { id: crypto.randomUUID(), lines, products, discount, customer, heldAt: new Date().toISOString() }] });
        return true;
      },

      // Swaps with the current cart, which goes on hold if it has items.
      resumeSale: (id) => {
        const { held, lines, products, discount, customer, mode } = get();
        const sale = held.find((h) => h.id === id);
        if (!sale || mode !== "cart") return;
        const rest = held.filter((h) => h.id !== id);
        const current = lines.length ? [{ id: crypto.randomUUID(), lines, products, discount, customer, heldAt: new Date().toISOString() }] : [];
        set({ ...emptySale, lines: sale.lines, products: sale.products, discount: sale.discount, customer: sale.customer ?? null, held: [...rest, ...current] });
      },

      discardHeld: (id) => set({ held: get().held.filter((h) => h.id !== id) }),

      startCheckout: () => {
        const { lines, ageVerified } = get();
        if (lines.length === 0 || !ageVerified) return;
        set({ mode: "tender", payments: [], saleKey: crypto.randomUUID(), needsAuthorization: false, error: null });
      },

      backToCart: () => set({ mode: "cart", payments: [], saleKey: null, needsAuthorization: false, error: null }),

      addPayment: (payment) => {
        const { lines, discount, payments, rate } = get();
        const { remaining } = summarizeTender(computeTotals(lines, discount).total, payments, rate);
        if (remaining <= 0 || payment.amount <= 0) return;
        set({ payments: [...payments, capNonCashPayment({ ...payment, id: crypto.randomUUID() }, remaining, rate)], error: null });
      },

      removePayment: (paymentId) => set({ payments: get().payments.filter((payment) => payment.id !== paymentId), error: null }),

      confirmSale: async (authorization) => {
        const state = get();
        const totals = computeTotals(state.lines, state.discount);
        const tender = summarizeTender(totals.total, state.payments, state.rate);
        if (!tender.isCovered || state.submitting || !state.saleKey) return;
        set({ submitting: true, error: null });

        // Without connection the sale goes to the outbox and syncs later with the same key.
        const saveOffline = async (): Promise<void> => {
          const ctx = get().offlineContext;
          if (!ctx || !state.saleKey) {
            set({ submitting: false, error: posCopy.offline.unavailable });
            return;
          }
          const result = buildOfflineSale(
            { key: state.saleKey, lines: state.lines, discount: state.discount, customer: state.customer, payments: state.payments, rate: state.rate, input: toSaleInput(state, null) },
            ctx,
            new Date(),
          );
          if ("error" in result) {
            set({ submitting: false, error: posCopy.offline.errors[result.error] });
            return;
          }
          await useOutbox.getState().enqueue(result.sale);
          await discountCachedStock(unitsByProduct(state.lines)).catch(() => undefined);
          const { receipt } = result.sale;
          set({
            ...emptySale,
            mode: "complete",
            submitting: false,
            lastSale: {
              id: result.sale.key,
              invoice: receipt.code,
              total: receipt.total,
              paid: tender.paid,
              change: receipt.change,
              itemCount: totals.itemCount,
              customerEmail: null,
              offline: receipt,
            },
          });
        };

        if (!useConnectivity.getState().online) return saveOffline();
        try {
          const sale = await salesApi.create(state.saleKey, toSaleInput(state, authorization ?? null));
          const paid = (sale.payments ?? []).reduce((sum, p) => sum + amountToCents(p.amountNio), 0);
          set({
            ...emptySale,
            mode: "complete",
            submitting: false,
            lastSale: {
              id: sale.id,
              invoice: sale.invoice,
              total: amountToCents(sale.total),
              paid,
              change: amountToCents(sale.changeNio),
              itemCount: sale.itemCount,
              customerEmail: sale.customer.email,
            },
          });
        } catch (error) {
          if (isNetworkError(error)) {
            useConnectivity.getState().setOnline(false);
            return saveOffline();
          }
          const needsAuthorization = error instanceof ApiError && (error.code === "authorization_required" || error.code === "authorization_failed");
          set({
            submitting: false,
            needsAuthorization: needsAuthorization || state.needsAuthorization,
            error: error instanceof ApiError ? error.message : "No se pudo registrar la venta. Intenta de nuevo.",
          });
        }
      },

      startNewSale: () => set({ ...emptySale, mode: "cart" }),
    }),
    {
      name: "anejo-pos-cart-v3",
      storage: createJSONStorage(() => localStorage),
      partialize: ({ lines, products, discount, customer, ageVerified, held }) => ({ lines, products, discount, customer, ageVerified, held }),
      skipHydration: true,
    },
  ),
);
