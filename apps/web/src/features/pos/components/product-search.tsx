"use client";

import { Barcode, MagnifyingGlass, WarningCircle } from "@phosphor-icons/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useDeferredValue, useEffect, useMemo, useState, type KeyboardEvent } from "react";
import { Kbd } from "@/components/ui/kbd";
import { catalogApi, catalogKeys } from "@/features/catalog/api";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { posCopy } from "../copy";
import { toPosProduct } from "../data/products";
import { availableStock, type PosPresentation, type PosProduct } from "../domain";
import { POS_SEARCH_INPUT_ID } from "../hooks/search-focus";
import { findByCode, searchCatalog, useCachedCatalog } from "../offline/catalog-cache";
import { isNetworkError, useConnectivity } from "../offline/connectivity";
import { usePosStore } from "../store/pos-store";
import { CategoryFilter } from "./category-filter";
import { ProductRow } from "./product-row";

const LISTBOX_ID = "pos-results";
const optionId = (productId: string) => `pos-option-${productId}`;

/** Scanner input and typed SKUs: a single token of letters, digits and dashes with at least one digit. */
const looksLikeCode = (value: string) => /^[A-Za-z0-9-]{4,}$/.test(value) && /\d/.test(value);

interface ProductSearchProps {
  /** Locks the catalog while a sale is being charged. */
  locked: boolean;
}

export function ProductSearch({ locked }: ProductSearchProps) {
  const lines = usePosStore((state) => state.lines);
  const notice = usePosStore((state) => state.notice);
  const addToCart = usePosStore((state) => state.addToCart);
  const setNotice = usePosStore((state) => state.setNotice);

  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const deferredQuery = useDeferredValue(query);

  const online = useConnectivity((s) => s.online);
  const setOnline = useConnectivity((s) => s.setOnline);
  const cached = useCachedCatalog((s) => s.catalog);

  const filters = { search: deferredQuery, categoryId: category ?? "", active: "true" as const, lowStock: false, page: 1, pageSize: 40 };
  const search = useQuery({
    queryKey: catalogKeys.productList(filters),
    queryFn: () => catalogApi.products(filters),
    placeholderData: keepPreviousData,
    enabled: online,
    retry: (count, error) => !isNetworkError(error) && count < 2,
  });
  const categories = useQuery({ queryKey: catalogKeys.masters("categories"), queryFn: () => catalogApi.masters("categories"), enabled: online });

  useEffect(() => {
    if (search.isError && isNetworkError(search.error)) setOnline(false);
  }, [search.isError, search.error, setOnline]);

  // Offline, the device's copy of the catalog answers instead of the server.
  const localResults = useMemo(
    () => (online || !cached ? null : searchCatalog(cached.products, deferredQuery, category)),
    [online, cached, deferredQuery, category],
  );
  const results = useMemo(() => (localResults ?? search.data?.data ?? []).map(toPosProduct), [localResults, search.data]);
  const resultTotal = localResults ? localResults.length : (search.data?.meta.total ?? 0);
  const categoryList = online ? categories.data : cached?.categories;
  const active = results[Math.min(activeIndex, results.length - 1)];

  useEffect(() => {
    if (!active) return;
    document.getElementById(optionId(active.id))?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const select = (product: PosProduct, presentation: PosPresentation | null = null) => {
    if (addToCart(product, presentation)) {
      setQuery("");
      setActiveIndex(0);
    } else {
      setNotice(posCopy.search.noMoreStock(product.name));
    }
  };

  const addScannedLocally = (code: string) => {
    const found = cached && findByCode(cached.products, code);
    if (!found) return false;
    const pos = toPosProduct(found.product);
    select(pos, pos.presentations.find((p) => p.id === found.presentationId) ?? null);
    return true;
  };

  const addScanned = async (code: string) => {
    if (!online) return addScannedLocally(code);
    try {
      const { product, presentationId } = await catalogApi.lookup(code);
      const pos = toPosProduct(product);
      select(pos, pos.presentations.find((p) => p.id === presentationId) ?? null);
      return true;
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) return false;
      if (isNetworkError(error)) {
        setOnline(false);
        return addScannedLocally(code);
      }
      setNotice(error instanceof ApiError ? error.message : posCopy.search.notFound(code));
      return true;
    }
  };

  const onKeyDown = async (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex((index) => Math.max(0, Math.min(index + step, results.length - 1)));
      return;
    }
    const value = query.trim();
    if (event.key !== "Enter" || !value) return;
    event.preventDefault();
    if (looksLikeCode(value) && (await addScanned(value))) return;
    // The list may still show the previous search while the new one loads: never add from it.
    const stale = deferredQuery !== query || (online && (search.isPlaceholderData || search.isPending));
    if (stale) return;
    if (active) return select(active);
    setNotice(posCopy.search.notFound(value));
  };

  return (
    <section
      inert={locked}
      aria-label={posCopy.search.label}
      className={cn("flex min-h-0 flex-col gap-4 p-5 transition-opacity duration-200", locked && "opacity-50")}
    >
      <div className="relative">
        <label htmlFor={POS_SEARCH_INPUT_ID} className="sr-only">
          {posCopy.search.label}
        </label>
        <MagnifyingGlass
          size={20}
          aria-hidden
          className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-ink-subtle"
        />
        <input
          id={POS_SEARCH_INPUT_ID}
          role="combobox"
          aria-expanded
          aria-controls={LISTBOX_ID}
          aria-activedescendant={active ? optionId(active.id) : undefined}
          autoComplete="off"
          autoFocus
          spellCheck={false}
          value={query}
          placeholder={posCopy.search.placeholder}
          onChange={(event) => {
            setQuery(event.target.value);
            setActiveIndex(0);
            if (notice) setNotice(null);
          }}
          onKeyDown={onKeyDown}
          className="h-13 w-full rounded-lg border border-field bg-surface pr-24 pl-12 text-body-lg text-ink transition-[border-color,box-shadow] duration-150 placeholder:text-ink-subtle focus:border-amber focus:shadow-[0_0_0_3px_color-mix(in_oklch,var(--amber)_25%,transparent)] focus:outline-none"
        />
        <span className="pointer-events-none absolute top-1/2 right-4 flex -translate-y-1/2 items-center gap-2 text-ink-subtle">
          <Barcode size={18} aria-hidden />
          <Kbd>{posCopy.shortcuts.search}</Kbd>
        </span>
      </div>

      <div className="flex items-center justify-between gap-4">
        <CategoryFilter
          categories={(categoryList ?? []).filter((c) => c.active && c.productCount > 0)}
          value={category}
          onChange={(next) => {
            setCategory(next);
            setActiveIndex(0);
          }}
        />
        <span className="shrink-0 text-caption text-ink-subtle" aria-live="polite">
          {posCopy.search.resultsCount(resultTotal)}
        </span>
      </div>

      <div role="status" className="empty:hidden">
        {notice && (
          <p className="flex items-center gap-2 rounded-md bg-danger-tint px-3 py-2 text-label text-danger">
            <WarningCircle size={16} weight="bold" aria-hidden />
            {notice}
          </p>
        )}
      </div>

      {results.length > 0 ? (
        <ul
          id={LISTBOX_ID}
          role="listbox"
          aria-label={posCopy.search.label}
          className="-mx-2 min-h-0 flex-1 space-y-0.5 overflow-y-auto px-2"
        >
          {results.map((product, index) => (
            <ProductRow
              key={product.id}
              id={optionId(product.id)}
              product={product}
              available={availableStock(product, lines)}
              active={product.id === active?.id}
              onSelect={(presentation) => select(product, presentation)}
              onHover={() => setActiveIndex(index)}
            />
          ))}
        </ul>
      ) : !online && !cached ? (
        <div className="grid flex-1 place-content-center gap-1 text-center">
          <p className="max-w-xs text-ink-muted">{posCopy.offline.noCatalog}</p>
        </div>
      ) : online && search.isPending ? (
        <ul className="-mx-2 space-y-0.5 px-2" aria-hidden>
          {Array.from({ length: 8 }, (_, i) => (
            <li key={i} className="flex items-center justify-between rounded-md px-3 py-3.5">
              <span className="h-3 w-1/2 animate-pulse rounded-xs bg-sunken" />
              <span className="h-3 w-20 animate-pulse rounded-xs bg-sunken" />
            </li>
          ))}
        </ul>
      ) : (
        <div className="grid flex-1 place-content-center gap-1 text-center">
          <p className="text-title font-semibold">{online && search.isError ? posCopy.search.errorTitle : posCopy.search.emptyTitle}</p>
          <p className="max-w-xs text-ink-muted">{online && search.isError ? posCopy.search.errorBody : posCopy.search.emptyBody}</p>
        </div>
      )}
    </section>
  );
}
