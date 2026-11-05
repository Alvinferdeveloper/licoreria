import { describe, expect, it } from "vitest";
import { purchaseTotals } from "./domain";

describe("purchaseTotals", () => {
  it("rounds tax per line at each product's rate", () => {
    const totals = purchaseTotals([
      { quantity: 3, unitCost: 33_333, taxPercent: 15 },
      { quantity: 1, unitCost: 1_005, taxPercent: 0 },
    ]);
    expect(totals).toEqual({ subtotal: 101_004, tax: 15_000, total: 116_004 });
  });
});
