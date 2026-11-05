import type { CategoryId, Product } from "../domain";
import { CATEGORIES } from "./catalog";

const categoryLabel = new Map(CATEGORIES.map((category) => [category.id, category.label]));

const normalize = (value: string) =>
  value.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase().trim();

/** Exact barcode or SKU match, used to add scanned products directly. */
export function findByCode(products: readonly Product[], code: string): Product | undefined {
  const query = code.trim().toUpperCase();
  if (!query) return undefined;
  return products.find((product) => product.barcode === query || product.sku === query);
}

export function searchProducts(
  products: readonly Product[],
  query: string,
  categoryId: CategoryId | null,
): Product[] {
  const terms = normalize(query).split(/\s+/).filter(Boolean);
  return products.filter((product) => {
    if (categoryId && product.categoryId !== categoryId) return false;
    if (terms.length === 0) return true;
    const haystack = normalize(
      `${product.name} ${product.brand} ${categoryLabel.get(product.categoryId)} ${product.presentation} ${product.sku} ${product.barcode}`,
    );
    return terms.every((term) => haystack.includes(term));
  });
}
