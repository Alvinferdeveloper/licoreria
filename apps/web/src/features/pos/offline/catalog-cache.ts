"use client";

import { create } from "zustand";
import { catalogApi, type Master, type Product } from "@/features/catalog/api";
import { idb } from "@/lib/offline/idb";

export interface CachedCatalog {
  products: Product[];
  categories: Master[];
  savedAt: string;
}

const KEY = "catalog";
const PAGE_SIZE = 100;

/** The device's copy of the catalog, mirrored in memory for instant offline search. */
export const useCachedCatalog = create<{ catalog: CachedCatalog | null }>()(() => ({ catalog: null }));
const keep = (catalog: CachedCatalog) => useCachedCatalog.setState({ catalog });

/** Downloads every active product so the POS can keep selling without connection. */
export async function refreshCatalog(): Promise<CachedCatalog> {
  const products: Product[] = [];
  for (let page = 1; ; page++) {
    const res = await catalogApi.products({ search: "", categoryId: "", active: "true", lowStock: false, page, pageSize: PAGE_SIZE });
    products.push(...res.data);
    if (products.length >= res.meta.total || res.data.length === 0) break;
  }
  const categories = await catalogApi.masters("categories");
  const catalog = { products, categories, savedAt: new Date().toISOString() };
  keep(catalog);
  await idb.put("kv", catalog, KEY);
  return catalog;
}

export async function loadCatalog() {
  const catalog = await idb.get<CachedCatalog>("kv", KEY).catch(() => undefined);
  if (catalog && !useCachedCatalog.getState().catalog) keep(catalog);
  return catalog;
}

/** Lowers the cached stock after a sale made offline, so availability stays close to reality. */
export async function discountCachedStock(units: Record<string, number>) {
  const current = useCachedCatalog.getState().catalog ?? (await loadCatalog());
  if (!current) return;
  const catalog = { ...current, products: current.products.map((p) => (units[p.id] ? { ...p, stock: p.stock - units[p.id] } : p)) };
  keep(catalog);
  await idb.put("kv", catalog, KEY);
}

const fold = (s: string) => s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();

/** Same idea as the server search: every word must appear in the name, brand, SKU or barcode. */
export function searchCatalog(products: Product[], query: string, categoryId: string | null, limit = 40): Product[] {
  const words = fold(query).split(/\s+/).filter(Boolean);
  const matches = products.filter((p) => {
    if (categoryId && p.category.id !== categoryId) return false;
    if (!words.length) return true;
    const haystack = fold([p.name, p.brand?.name ?? "", p.sku, p.barcode ?? "", p.category.name].join(" "));
    return words.every((w) => haystack.includes(w));
  });
  return matches.sort((a, b) => a.name.localeCompare(b.name, "es")).slice(0, limit);
}

/** Finds a scanned or typed code among SKUs, product barcodes and presentation barcodes. */
export function findByCode(products: Product[], code: string): { product: Product; presentationId: string | null } | null {
  const c = code.trim().toLowerCase();
  for (const p of products) {
    if (p.sku.toLowerCase() === c || p.barcode?.toLowerCase() === c) return { product: p, presentationId: null };
    const presentation = p.presentations.find((pr) => pr.active && pr.barcode?.toLowerCase() === c);
    if (presentation) return { product: p, presentationId: presentation.id };
  }
  return null;
}
