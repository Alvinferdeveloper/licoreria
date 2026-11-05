import { describe, expect, it } from "vitest";
import { addProduct, computeTotals, setQuantity, unitsInCart } from "./cart";
import type { CartLine, PosProduct } from "./types";

const beer: PosProduct = {
  id: "p1",
  sku: "TON-LAT",
  barcode: "7430020003550",
  name: "Toña lata",
  detail: "355 ml",
  imageUrl: null,
  price: 3130,
  taxPercent: 15,
  stock: 8,
  minStock: 6,
  allowNegativeStock: false,
  presentations: [{ id: "six", name: "Six-pack", factor: 6, price: 18200 }],
};
const sixPack = beer.presentations[0];

describe("cart", () => {
  it("keeps units and packs of the same product on separate lines", () => {
    const lines = addProduct(addProduct([], beer)!, beer, sixPack)!;
    expect(lines).toHaveLength(2);
    expect(unitsInCart(beer.id, lines)).toBe(7);
  });

  it("refuses to exceed stock counted in base units", () => {
    const lines = addProduct([], beer, sixPack)!;
    expect(addProduct(lines, beer, sixPack)).toBeNull();
    expect(addProduct(addProduct(lines, beer)!, beer)).not.toBeNull();
    expect(addProduct(addProduct(addProduct(lines, beer)!, beer)!, beer)).toBeNull();
  });

  it("lets products allowed to go negative sell past stock", () => {
    expect(addProduct([], { ...beer, stock: 0, allowNegativeStock: true })).not.toBeNull();
  });

  it("removes a line when quantity drops to zero", () => {
    const lines = addProduct([], beer)!;
    expect(setQuantity(lines, lines[0].key, 0)).toEqual([]);
  });

  it("computes subtotal, tax and total in cents", () => {
    const lines: CartLine[] = addProduct(addProduct([], beer)!, beer, sixPack)!;
    expect(computeTotals(lines)).toEqual({ subtotal: 21330, discount: 0, tax: 3200, total: 24530, itemCount: 2 });
  });
});
