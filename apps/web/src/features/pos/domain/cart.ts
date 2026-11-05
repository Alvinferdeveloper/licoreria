import { percentOf, sumCents, type Cents } from "@/lib/money";
import { allocate, discountAmount, type Discount } from "./discount";
import type { CartLine, CartTotals, PosPresentation, PosProduct, StockLevel } from "./types";

export const lineSubtotal = (line: CartLine): Cents => line.unitPrice * line.quantity;

/** Totals with the sale discount spread across lines, so VAT is charged on the discounted base. */
export function computeTotals(lines: readonly CartLine[], discount: Discount | null = null): CartTotals {
  const subtotals = lines.map(lineSubtotal);
  const subtotal = sumCents(subtotals);
  const discountTotal = discountAmount(discount, subtotal);
  const shares = allocate(discountTotal, subtotals);
  const tax = sumCents(lines.map((line, index) => percentOf(subtotals[index] - shares[index], line.taxPercent)));
  return {
    subtotal,
    discount: discountTotal,
    tax,
    total: subtotal - discountTotal + tax,
    itemCount: lines.reduce((count, line) => count + line.quantity, 0),
  };
}

export const lineKey = (productId: string, presentationId: string | null) => `${productId}:${presentationId ?? "unit"}`;

export function lineFromProduct(product: PosProduct, presentation: PosPresentation | null = null): CartLine {
  return {
    key: lineKey(product.id, presentation?.id ?? null),
    productId: product.id,
    presentationId: presentation?.id ?? null,
    name: product.name,
    detail: presentation ? `${presentation.name} · ${presentation.factor} u` : product.detail,
    unitPrice: presentation?.price ?? product.price,
    taxPercent: product.taxPercent,
    quantity: 1,
    unitsPerItem: presentation?.factor ?? 1,
  };
}

/** Base units of a product already in the cart, across its unit and pack lines. */
export const unitsInCart = (productId: string, lines: readonly CartLine[]) =>
  lines.filter((line) => line.productId === productId).reduce((units, line) => units + line.quantity * line.unitsPerItem, 0);

/** Base units still available for a product after what is already in the cart. */
export const availableStock = (product: PosProduct, lines: readonly CartLine[]) => product.stock - unitsInCart(product.id, lines);

/** Whether `units` more base units fit in stock; products allowed to sell negative always fit. */
export const fitsInStock = (product: PosProduct, lines: readonly CartLine[], units: number) =>
  product.allowNegativeStock || availableStock(product, lines) >= units;

/** Adds one unit or pack, merging with its existing line. Returns null when stock doesn't allow it. */
export function addProduct(lines: readonly CartLine[], product: PosProduct, presentation: PosPresentation | null = null): CartLine[] | null {
  if (!fitsInStock(product, lines, presentation?.factor ?? 1)) return null;
  const key = lineKey(product.id, presentation?.id ?? null);
  if (!lines.some((line) => line.key === key)) return [...lines, lineFromProduct(product, presentation)];
  return lines.map((line) => (line.key === key ? { ...line, quantity: line.quantity + 1 } : line));
}

export function setQuantity(lines: readonly CartLine[], key: string, quantity: number): CartLine[] {
  if (quantity <= 0) return lines.filter((line) => line.key !== key);
  return lines.map((line) => (line.key === key ? { ...line, quantity } : line));
}

export function stockLevel(product: Pick<PosProduct, "stock" | "minStock">): StockLevel {
  if (product.stock <= 0) return "out";
  if (product.stock <= product.minStock) return "low";
  return "ok";
}
