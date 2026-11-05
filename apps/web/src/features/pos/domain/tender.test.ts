import { describe, expect, it } from "vitest";
import { amountDue, capNonCashPayment, summarizeTender } from "./tender";
import type { Payment } from "./types";

const RATE = 36.6;

const pay = (partial: Partial<Payment>): Payment => ({
  id: "x",
  method: "cash",
  currency: "NIO",
  amount: 0,
  ...partial,
});

describe("tender", () => {
  it("returns change in córdobas when paying in dollars", () => {
    const summary = summarizeTender(85000, [pay({ currency: "USD", amount: 3000 })], RATE);
    expect(summary).toEqual({ paid: 109800, remaining: 0, change: 24800, isCovered: true });
  });

  it("supports mixed payments", () => {
    const summary = summarizeTender(
      85000,
      [pay({ currency: "USD", amount: 2000 }), pay({ amount: 12000 })],
      RATE,
    );
    expect(summary.paid).toBe(85200);
    expect(summary.change).toBe(200);
  });

  it("reports the remaining balance while underpaid", () => {
    const summary = summarizeTender(85000, [pay({ amount: 50000 })], RATE);
    expect(summary).toMatchObject({ remaining: 35000, change: 0, isCovered: false });
  });

  it("rounds the USD amount due up so it always covers the sale", () => {
    const usd = amountDue(85000, "USD", RATE);
    expect(usd).toBe(2323);
    expect(Math.round(usd * RATE)).toBeGreaterThanOrEqual(85000);
  });

  it("caps card payments at the remaining balance", () => {
    const capped = capNonCashPayment(pay({ method: "card", amount: 99999 }), 85000, RATE);
    expect(capped.amount).toBe(85000);
  });
});
