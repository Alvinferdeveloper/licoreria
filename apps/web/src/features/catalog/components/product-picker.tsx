"use client";

import { MagnifyingGlass } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import { useDeferredValue, useId, useState, type KeyboardEvent } from "react";
import { inputClassName } from "@/components/ui/field";
import { cn } from "@/lib/cn";
import { catalogApi, catalogKeys, type Product } from "../api";

interface ProductPickerProps {
  onPick: (product: Product) => void;
  placeholder: string;
  /** Products already chosen are shown but cannot be picked again. */
  excludeIds?: readonly string[];
  autoFocus?: boolean;
  className?: string;
}

/** Search-as-you-type combobox over active products; Enter picks the highlighted one. */
export function ProductPicker({ onPick, placeholder, excludeIds = [], autoFocus, className }: ProductPickerProps) {
  const listId = useId();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const search = useDeferredValue(query.trim());
  const filters = { search, categoryId: "", active: "true", lowStock: false, page: 1, pageSize: 8 } as const;
  const results = useQuery({ queryKey: catalogKeys.productList(filters), queryFn: () => catalogApi.products(filters), enabled: search.length > 0 });
  const options = search ? (results.data?.data ?? []) : [];

  const pick = (product: Product | undefined) => {
    if (!product || excludeIds.includes(product.id)) return;
    onPick(product);
    setQuery("");
    setOpen(false);
    setActive(0);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, options.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      pick(options[active]);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  return (
    <div className={cn("relative", className)}>
      <MagnifyingGlass size={16} className="pointer-events-none absolute top-3 left-3 text-ink-subtle" aria-hidden />
      <input
        type="search"
        role="combobox"
        aria-expanded={open && options.length > 0}
        aria-controls={listId}
        aria-activedescendant={options[active] ? `${listId}-${active}` : undefined}
        aria-label={placeholder}
        placeholder={placeholder}
        autoFocus={autoFocus}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          setActive(0);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 120)}
        onKeyDown={onKeyDown}
        className={cn(inputClassName, "pl-9")}
      />
      {open && options.length > 0 && (
        <ul id={listId} role="listbox" className="absolute z-20 mt-1 max-h-80 w-full overflow-y-auto rounded-md border border-line bg-surface py-1 shadow-(--shadow-float-low)">
          {options.map((p, i) => {
            const taken = excludeIds.includes(p.id);
            return (
              <li
                key={p.id}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={i === active}
                aria-disabled={taken}
                onMouseDown={(e) => {
                  e.preventDefault();
                  pick(p);
                }}
                onMouseEnter={() => setActive(i)}
                className={cn("flex cursor-pointer items-center justify-between gap-3 px-3 py-2", i === active && "bg-sunken", taken && "cursor-default opacity-50")}
              >
                <span className="min-w-0">
                  <span className="block truncate text-label font-medium">{p.name}</span>
                  <span className="block font-mono text-caption tracking-wide text-ink-muted">{p.sku}</span>
                </span>
                <span className="shrink-0 font-mono text-caption tabular text-ink-muted">{p.stock}</span>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
