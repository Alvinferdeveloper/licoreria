import { percentOf, type Cents } from "@/lib/money";

export type Discount =
  | { kind: "percent"; value: number; authorizedBy: string | null }
  | { kind: "amount"; value: Cents; authorizedBy: string | null };

export type DiscountError = "invalid" | "exceeds-subtotal";

/** Discount amount in cents for a given subtotal. */
export function discountAmount(discount: Discount | null, subtotal: Cents): Cents {
  if (!discount) return 0;
  const raw = discount.kind === "percent" ? percentOf(subtotal, discount.value) : discount.value;
  return Math.min(Math.max(raw, 0), subtotal);
}

/** Effective percentage of the subtotal, used to check the cashier's limit. */
export function discountPercent(discount: Discount, subtotal: Cents): number {
  if (discount.kind === "percent") return discount.value;
  return subtotal > 0 ? (discount.value / subtotal) * 100 : 0;
}

export function validateDiscount(discount: Discount, subtotal: Cents): DiscountError | null {
  if (!Number.isFinite(discount.value) || discount.value <= 0) return "invalid";
  if (discount.kind === "percent" && discount.value > 100) return "invalid";
  if (discount.kind === "amount" && discount.value > subtotal) return "exceeds-subtotal";
  return null;
}

export const requiresAuthorization = (discount: Discount, subtotal: Cents, limitPercent: number) =>
  discountPercent(discount, subtotal) > limitPercent;

/** Splits an amount across weights proportionally; remainders go to the largest fractions. */
export function allocate(amount: Cents, weights: readonly Cents[]): Cents[] {
  const totalWeight = weights.reduce((sum, weight) => sum + weight, 0);
  if (totalWeight === 0) return weights.map(() => 0);
  const exact = weights.map((weight) => (amount * weight) / totalWeight);
  const shares = exact.map(Math.floor);
  let remainder = amount - shares.reduce((sum, share) => sum + share, 0);
  const byFraction = exact
    .map((value, index) => ({ index, fraction: value - Math.floor(value) }))
    .sort((a, b) => b.fraction - a.fraction);
  for (const { index } of byFraction) {
    if (remainder <= 0) break;
    shares[index] += 1;
    remainder -= 1;
  }
  return shares;
}
