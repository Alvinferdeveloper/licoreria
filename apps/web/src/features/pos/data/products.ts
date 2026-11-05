import { amountToCents, type Product } from "@/features/catalog/api";
import type { PosProduct } from "../domain";

/** Maps a catalog product to what the POS sells: prices in cents, only active presentations. */
export function toPosProduct(p: Product): PosProduct {
  const detail = p.volumeMl ? (p.volumeMl >= 1000 ? `${p.volumeMl / 1000} L` : `${p.volumeMl} ml`) : p.unit.name;
  return {
    id: p.id,
    sku: p.sku,
    barcode: p.barcode,
    name: p.name,
    detail,
    imageUrl: p.imageUrl,
    price: amountToCents(p.price),
    taxPercent: Number(p.taxPercent),
    stock: p.stock,
    minStock: p.minStock,
    allowNegativeStock: p.allowNegativeStock,
    presentations: p.presentations
      .filter((pr) => pr.active)
      .map((pr) => ({ id: pr.id, name: pr.name, factor: pr.factor, price: amountToCents(pr.price) })),
  };
}
