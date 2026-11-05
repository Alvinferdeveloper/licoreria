import { describe, expect, it } from "vitest";
import { computeTotals } from "./cart";
import { allocate, discountAmount, requiresAuthorization, validateDiscount, type Discount } from "./discount";
import type { CartLine } from "./types";

const percent = (value: number): Discount => ({ kind: "percent", value, authorizedBy: null });
const amount = (value: number): Discount => ({ kind: "amount", value, authorizedBy: null });

const line = (unitPrice: number, quantity = 1, taxPercent = 15): CartLine => ({
  productId: `p${unitPrice}`,
  name: "x",
  presentation: "750 ml",
  unitPrice,
  taxPercent,
  quantity,
});

describe("discount", () => {
  it("computes percent and fixed discounts, capped at the subtotal", () => {
    expect(discountAmount(percent(10), 85000)).toBe(8500);
    expect(discountAmount(amount(5000), 85000)).toBe(5000);
    expect(discountAmount(amount(99999), 85000)).toBe(85000);
    expect(discountAmount(null, 85000)).toBe(0);
  });

  it("validates input", () => {
    expect(validateDiscount(percent(0), 1000)).toBe("invalid");
    expect(validateDiscount(percent(120), 1000)).toBe("invalid");
    expect(validateDiscount(amount(2000), 1000)).toBe("exceeds-subtotal");
    expect(validateDiscount(percent(10), 1000)).toBeNull();
  });

  it("requires authorization above the cashier limit", () => {
    expect(requiresAuthorization(percent(5), 10000, 5)).toBe(false);
    expect(requiresAuthorization(percent(6), 10000, 5)).toBe(true);
    expect(requiresAuthorization(amount(600), 10000, 5)).toBe(true);
  });

  it("allocates without losing cents", () => {
    const shares = allocate(100, [1, 1, 1]);
    expect(shares.reduce((a, b) => a + b, 0)).toBe(100);
    expect(shares).toEqual([34, 33, 33]);
    expect(allocate(50, [0, 0])).toEqual([0, 0]);
  });

  it("charges VAT on the discounted base", () => {
    const totals = computeTotals([line(60000), line(40000)], percent(10));
    expect(totals).toEqual({ subtotal: 100000, discount: 10000, tax: 13500, total: 103500, itemCount: 2 });
  });

  it("respects per-line tax rates when discounting", () => {
    const totals = computeTotals([line(50000, 1, 15), line(50000, 1, 0)], amount(10000));
    expect(totals.tax).toBe(6750);
    expect(totals.total).toBe(96750);
  });
});
