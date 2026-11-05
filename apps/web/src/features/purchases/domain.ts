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

/** Relative change against the previous cost, e.g. 0.05 for 5 % more expensive. */
export function costChange(current: Cents, previous: Cents | null): number | null {
  if (previous === null || previous === 0) return null;
  return (current - previous) / previous;
}
