import type { SaleInput } from "@/features/sales/api";
import { computeTotals, requiresAuthorization, summarizeTender, type CartLine, type Customer, type Discount, type Payment } from "../domain";
import { receiptCode, type OutboxSale } from "./outbox";

/** Who is selling and the rules that apply, as last seen online. */
export interface OfflineContext {
  userId: string;
  cashierName: string;
  registerName: string;
  discountLimitPercent: number;
  canAuthorizeDiscounts: boolean;
}

export interface CartSnapshot {
  key: string;
  lines: CartLine[];
  discount: Discount | null;
  customer: Customer | null;
  payments: Payment[];
  rate: number;
  input: SaleInput;
}

export type OfflineSaleError = "cash-only" | "discount-needs-pin";

/**
 * Builds the queued sale for a cart charged without connection. Only cash can be taken
 * (cards need the network anyway) and no administrator PIN can be checked.
 */
export function buildOfflineSale(cart: CartSnapshot, ctx: OfflineContext, now: Date): { sale: OutboxSale } | { error: OfflineSaleError } {
  if (cart.payments.some((p) => p.method !== "cash")) return { error: "cash-only" };
  const totals = computeTotals(cart.lines, cart.discount);
  if (cart.discount && !ctx.canAuthorizeDiscounts && requiresAuthorization(cart.discount, totals.subtotal, ctx.discountLimitPercent)) {
    return { error: "discount-needs-pin" };
  }
  const tender = summarizeTender(totals.total, cart.payments, cart.rate);
  const createdAt = now.toISOString();
  return {
    sale: {
      key: cart.key,
      userId: ctx.userId,
      input: { ...cart.input, authorization: null, offlineAt: createdAt },
      status: "pending",
      error: null,
      receipt: {
        code: receiptCode(cart.key),
        createdAt,
        cashierName: ctx.cashierName,
        registerName: ctx.registerName,
        customer: cart.customer ? { name: cart.customer.name, taxId: cart.customer.taxId } : null,
        lines: cart.lines.map((l) => ({ name: l.name, quantity: l.quantity, unitsPerItem: l.unitsPerItem, unitPrice: l.unitPrice, subtotal: l.unitPrice * l.quantity })),
        subtotal: totals.subtotal,
        discount: totals.discount,
        tax: totals.tax,
        total: totals.total,
        payments: cart.payments.map((p) => ({ currency: p.currency, amount: p.amount })),
        change: tender.change,
        rate: cart.rate,
      },
    },
  };
}

/** Base units sold per product, to lower the cached stock. */
export function unitsByProduct(lines: readonly CartLine[]): Record<string, number> {
  const units: Record<string, number> = {};
  for (const l of lines) units[l.productId] = (units[l.productId] ?? 0) + l.quantity * l.unitsPerItem;
  return units;
}
