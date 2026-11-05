import { describe, expect, it } from "vitest";
import { addProduct, computeTotals, setQuantity, stockLevel } from "./cart";
import type { Product } from "./types";

const rum: Product = {
  id: "p1",
  sku: "RUM-7-750",
  barcode: "7401000000017",
  name: "Flor de Caña 7 años",
  brand: "Flor de Caña",
  categoryId: "rum",
  presentation: "750 ml",
  price: 42000,
  taxPercent: 15,
  stock: 2,
  minStock: 3,
};

describe("cart", () => {
  it("merges repeated products into one line", () => {
    const lines = addProduct(addProduct([], rum), rum);
    expect(lines).toHaveLength(1);
    expect(lines[0].quantity).toBe(2);
  });

  it("never exceeds available stock", () => {
    const lines = addProduct(addProduct(addProduct([], rum), rum), rum);
    expect(lines[0].quantity).toBe(2);
  });

  it("removes a line when quantity drops to zero", () => {
    expect(setQuantity(addProduct([], rum), rum.id, 0)).toEqual([]);
  });

  it("computes subtotal, tax and total in cents", () => {
    const totals = computeTotals(addProduct(addProduct([], rum), rum));
    expect(totals).toEqual({ subtotal: 84000, tax: 12600, total: 96600, itemCount: 2 });
  });

  it("classifies stock levels", () => {
    expect(stockLevel({ stock: 0, minStock: 3 })).toBe("out");
    expect(stockLevel({ stock: 3, minStock: 3 })).toBe("low");
    expect(stockLevel({ stock: 10, minStock: 3 })).toBe("ok");
  });
});
