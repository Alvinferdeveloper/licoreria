"use client";

import { cn } from "@/lib/cn";
import { posCopy } from "../copy";

interface CategoryFilterProps {
  categories: readonly { id: string; name: string }[];
  value: string | null;
  onChange: (value: string | null) => void;
}

export function CategoryFilter({ categories, value, onChange }: CategoryFilterProps) {
  const options = [{ id: null, name: posCopy.search.allCategories }, ...categories];
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label="Categorías">
      {options.map((option) => (
        <button
          key={option.id ?? "all"}
          type="button"
          aria-pressed={option.id === value}
          onClick={() => onChange(option.id)}
          className={cn(
            "press h-7 rounded-full px-3 text-caption font-medium transition-colors duration-150",
            option.id === value ? "bg-amber-tint text-amber-ink" : "bg-sunken text-ink-muted hover:text-ink",
          )}
        >
          {option.name}
        </button>
      ))}
    </div>
  );
}
