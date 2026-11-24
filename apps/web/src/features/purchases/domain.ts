import { percentOf, sumCents, type Cents } from "@/lib/money";

export interface DraftLine {
  quantity: number;
  unitCost: Cents;
  taxPercent: number;
}

export interface PurchaseTotals {
  subtotal: Cents;
  tax: Cents;
  total: Cents;
}

/** Mirrors the API: each line rounds its subtotal and its tax at the product's rate. */
export function purchaseTotals(lines: readonly DraftLine[]): PurchaseTotals {
  const subtotals = lines.map((l) => l.quantity * l.unitCost);
  const subtotal = sumCents(subtotals);
  const tax = sumCents(subtotals.map((s, i) => percentOf(s, lines[i].taxPercent)));
  return { subtotal, tax, total: subtotal + tax };
}

export interface MarginImpact {
  change: number;
  before: number;
  after: number;
  /** Sale price that keeps the previous margin, rounded up to whole córdobas. */
  suggested: Cents;
}

/** How a cost increase eats the margin of the current sale price (both before tax). */
export function marginImpact(cost: Cents, previous: Cents | null, price: Cents): MarginImpact | null {
  if (previous === null || previous <= 0 || price <= 0 || cost - previous <= previous * 0.005) return null;
  const suggested = Math.ceil((price * cost) / previous / 100) * 100;
  return { change: (cost - previous) / previous, before: (price - previous) / price, after: (price - cost) / price, suggested };
}

/** Relative change against the previous cost, e.g. 0.05 for 5 % more expensive. */
export function costChange(current: Cents, previous: Cents | null): number | null {
  if (previous === null || previous === 0) return null;
  return (current - previous) / previous;
}
