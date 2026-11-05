import { percentOf, sumCents, type Cents } from "@/lib/money";
import type { CartLine, CartTotals, Product, StockLevel } from "./types";

export const lineSubtotal = (line: CartLine): Cents => line.unitPrice * line.quantity;

export const lineTax = (line: CartLine): Cents =>
  percentOf(lineSubtotal(line), line.taxPercent);

export function computeTotals(lines: readonly CartLine[]): CartTotals {
  const subtotal = sumCents(lines.map(lineSubtotal));
  const tax = sumCents(lines.map(lineTax));
  return {
    subtotal,
    tax,
    total: subtotal + tax,
    itemCount: lines.reduce((count, line) => count + line.quantity, 0),
  };
}

export function lineFromProduct(product: Product, quantity = 1): CartLine {
  return {
    productId: product.id,
    name: product.name,
    presentation: product.presentation,
    unitPrice: product.price,
    taxPercent: product.taxPercent,
    quantity,
  };
}

/** Adds a product to the cart, merging with an existing line and capping at stock. */
export function addProduct(lines: readonly CartLine[], product: Product): CartLine[] {
  const existing = lines.find((line) => line.productId === product.id);
  if (!existing) return [...lines, lineFromProduct(product)];
  return lines.map((line) =>
    line.productId === product.id
      ? { ...line, quantity: Math.min(line.quantity + 1, product.stock) }
      : line,
  );
}

export function setQuantity(
  lines: readonly CartLine[],
  productId: string,
  quantity: number,
): CartLine[] {
  if (quantity <= 0) return lines.filter((line) => line.productId !== productId);
  return lines.map((line) => (line.productId === productId ? { ...line, quantity } : line));
}

export function stockLevel(product: Pick<Product, "stock" | "minStock">): StockLevel {
  if (product.stock <= 0) return "out";
  if (product.stock <= product.minStock) return "low";
  return "ok";
}

/** Units still available for a product after what is already in the cart. */
export function availableStock(product: Product, lines: readonly CartLine[]): number {
  const inCart = lines.find((line) => line.productId === product.id)?.quantity ?? 0;
  return product.stock - inCart;
}
