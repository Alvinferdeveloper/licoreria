import { describe, expect, it } from "vitest";
import type { SaleInput } from "@/features/sales/api";
import type { CartLine } from "../domain";
import { buildOfflineSale, unitsByProduct, type CartSnapshot, type OfflineContext } from "./offline-sale";

const line: CartLine = { key: "p1:unit", productId: "p1", presentationId: null, name: "Toña", detail: "355 ml", unitPrice: 4000, taxPercent: 15, quantity: 3, unitsPerItem: 1 };
const sixPack: CartLine = { ...line, key: "p1:six", presentationId: "six", unitPrice: 22000, quantity: 1, unitsPerItem: 6 };
const input = { customerId: null, ageVerified: true, notes: "", discount: null, authorization: null, lines: [], payments: [] } as SaleInput;
const ctx: OfflineContext = { userId: "u1", cashierName: "Ana", registerName: "Caja 1", discountLimitPercent: 5, canAuthorizeDiscounts: false };
const cart = (over: Partial<CartSnapshot> = {}): CartSnapshot => ({
  key: "abcdef12-0000-0000-0000-000000000000",
  lines: [line],
  discount: null,
  customer: null,
  payments: [{ id: "1", method: "cash", currency: "NIO", amount: 20000 }],
  rate: 36.6,
  input,
  ...over,
});

describe("buildOfflineSale", () => {
  it("queues a cash sale with its real time and the change owed", () => {
    const now = new Date("2026-11-20T16:00:00Z");
    const result = buildOfflineSale(cart(), ctx, now);
    if (!("sale" in result)) throw new Error(result.error);
    // 3 × C$ 40 = 120 + 15 % IVA = 138; paid 200 → change 62.
    expect(result.sale.receipt.total).toBe(13800);
    expect(result.sale.receipt.change).toBe(6200);
    expect(result.sale.receipt.code).toBe("P-ABCDEF12");
    expect(result.sale.input.offlineAt).toBe(now.toISOString());
  });

  it("refuses cards and discounts that would need an administrator PIN", () => {
    expect(buildOfflineSale(cart({ payments: [{ id: "1", method: "card", currency: "NIO", amount: 13800 }] }), ctx, new Date())).toEqual({ error: "cash-only" });
    const discount = { kind: "percent", value: 10, authorizedBy: null } as const;
    expect(buildOfflineSale(cart({ discount }), ctx, new Date())).toEqual({ error: "discount-needs-pin" });
    expect("sale" in buildOfflineSale(cart({ discount }), { ...ctx, canAuthorizeDiscounts: true }, new Date())).toBe(true);
  });
});

it("adds up base units per product across units and packs", () => {
  expect(unitsByProduct([line, sixPack])).toEqual({ p1: 9 });
});
