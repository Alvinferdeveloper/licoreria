"use client";

import { cn } from "@/lib/cn";
import { posCopy } from "../copy";
import type { Category, CategoryId } from "../domain";

interface CategoryFilterProps {
  categories: readonly Category[];
  value: CategoryId | null;
  onChange: (value: CategoryId | null) => void;
}

export function CategoryFilter({ categories, value, onChange }: CategoryFilterProps) {
  const options = [{ id: null, label: posCopy.search.allCategories }, ...categories];
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label="Categorías">
      {options.map((option) => {
        const selected = option.id === value;
        return (
          <button
            key={option.id ?? "all"}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(option.id)}
            className={cn(
              "press h-7 rounded-full px-3 text-caption font-medium transition-colors duration-150",
              selected ? "bg-amber-tint text-amber-ink" : "bg-sunken text-ink-muted hover:text-ink",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
