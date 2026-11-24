import { describe, expect, it } from "vitest";
import { marginImpact, purchaseTotals } from "./domain";

describe("marginImpact", () => {
  it("suggests the price that keeps the previous margin", () => {
    const impact = marginImpact(33_600, 30_000, 45_000)!;
    expect(impact.before).toBeCloseTo(1 / 3);
    expect(impact.after).toBeCloseTo(0.2533, 3);
    expect(impact.suggested).toBe(50_400);
    expect(marginImpact(29_000, 30_000, 45_000)).toBeNull();
  });
});

describe("purchaseTotals", () => {
  it("rounds tax per line at each product's rate", () => {
    const totals = purchaseTotals([
      { quantity: 3, unitCost: 33_333, taxPercent: 15 },
      { quantity: 1, unitCost: 1_005, taxPercent: 0 },
    ]);
    expect(totals).toEqual({ subtotal: 101_004, tax: 15_000, total: 116_004 });
  });
});
