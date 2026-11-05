import { nioToUsd, sumCents, toBaseCents, type Cents, type CurrencyCode } from "@/lib/money";
import type { Payment } from "./types";

export interface TenderSummary {
  /** Total paid, converted to NIO cents. */
  paid: Cents;
  /** NIO cents still owed; zero once the sale is covered. */
  remaining: Cents;
  /** Change owed to the customer in NIO cents. */
  change: Cents;
  isCovered: boolean;
}

export const paymentInBase = (payment: Payment, rate: number): Cents =>
  toBaseCents(payment.amount, payment.currency, rate);

export function summarizeTender(
  total: Cents,
  payments: readonly Payment[],
  rate: number,
): TenderSummary {
  const paid = sumCents(payments.map((payment) => paymentInBase(payment, rate)));
  return {
    paid,
    remaining: Math.max(total - paid, 0),
    change: Math.max(paid - total, 0),
    isCovered: payments.length > 0 && paid >= total,
  };
}

/** Exact amount owed in the given currency; USD rounds up so the sale is always covered. */
export function amountDue(remaining: Cents, currency: CurrencyCode, rate: number): Cents {
  if (currency === "NIO") return remaining;
  const usd = nioToUsd(remaining, rate);
  return toBaseCents(usd, "USD", rate) < remaining ? usd + 1 : usd;
}

/** Card and transfer can't overpay: they are capped at the remaining balance. */
export function capNonCashPayment(payment: Payment, remaining: Cents, rate: number): Payment {
  if (payment.method === "cash") return payment;
  return { ...payment, amount: Math.min(payment.amount, amountDue(remaining, payment.currency, rate)) };
}

export const BILL_SHORTCUTS: Record<CurrencyCode, readonly number[]> = {
  NIO: [100, 200, 500, 1000],
  USD: [5, 10, 20, 50, 100],
};
