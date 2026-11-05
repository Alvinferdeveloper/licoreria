import { describe, expect, it } from "vitest";
import { PRODUCTS } from "./catalog";
import { findByCode, searchProducts } from "./search";

describe("product search", () => {
  it("matches ignoring accents and case", () => {
    expect(searchProducts(PRODUCTS, "TONA", null).length).toBeGreaterThan(0);
  });

  it("matches by category name", () => {
    const results = searchProducts(PRODUCTS, "whisky", null);
    expect(results.length).toBeGreaterThan(0);
    expect(results.every((product) => product.categoryId === "whisky")).toBe(true);
  });

  it("requires every term to match", () => {
    expect(searchProducts(PRODUCTS, "flor 12", null).map((product) => product.sku)).toEqual(["RON-FC12-750"]);
  });

  it("finds products by exact barcode or SKU", () => {
    expect(findByCode(PRODUCTS, "7430011001073")?.sku).toBe("RON-FC7-750");
    expect(findByCode(PRODUCTS, "ron-fc7-750")?.sku).toBe("RON-FC7-750");
    expect(findByCode(PRODUCTS, "000")).toBeUndefined();
  });
});
