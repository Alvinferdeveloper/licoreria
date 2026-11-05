"use client";

import { MagnifyingGlass } from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { inputClassName } from "@/components/ui/field";
import { ApiError, type Page } from "@/lib/api/client";
import { cn } from "@/lib/cn";

export function SearchInput({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <div className="relative w-full max-w-xs">
      <MagnifyingGlass size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-ink-subtle" aria-hidden />
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className={cn(inputClassName, "h-9 pl-9")}
      />
    </div>
  );
}

interface FilterChipsProps<T extends string> {
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (value: T) => void;
}

export function FilterChips<T extends string>({ label, value, options, onChange }: FilterChipsProps<T>) {
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label={label}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          onClick={() => onChange(option.value)}
          className={cn(
            "press h-7 rounded-full px-3 text-caption font-medium transition-colors duration-150",
            value === option.value ? "bg-amber-tint text-amber-ink" : "bg-sunken text-ink-muted hover:text-ink",
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export const STATUS_OPTIONS = [
  { value: "all", label: "Todos" },
  { value: "true", label: "Activos" },
  { value: "false", label: "Inactivos" },
] as const;

export type StatusFilter = (typeof STATUS_OPTIONS)[number]["value"];

export function Pagination({ meta, onPage }: { meta: Page<unknown>["meta"] | undefined; onPage: (page: number) => void }) {
  if (!meta || meta.total <= meta.pageSize) return null;
  const from = (meta.page - 1) * meta.pageSize + 1;
  const to = Math.min(meta.page * meta.pageSize, meta.total);
  return (
    <div className="flex items-center justify-end gap-3 text-label text-ink-muted">
      <span className="tabular">
        {from}–{to} de {meta.total}
      </span>
      <Button size="sm" disabled={meta.page <= 1} onClick={() => onPage(meta.page - 1)}>
        Anterior
      </Button>
      <Button size="sm" disabled={to >= meta.total} onClick={() => onPage(meta.page + 1)}>
        Siguiente
      </Button>
    </div>
  );
}

export type Notice = { tone: "success" | "danger"; text: string };

/** Transient page-level message that clears itself after 5 seconds. */
export function useNotice() {
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 5000);
    return () => clearTimeout(timer);
  }, [notice]);

  const success = (text: string) => setNotice({ tone: "success", text });
  const failure = (error: unknown) =>
    setNotice({ tone: "danger", text: error instanceof ApiError ? error.message : "Ocurrió un error inesperado." });

  return { notice, success, failure, clear: () => setNotice(null) };
}

export function NoticeBanner({ notice }: { notice: Notice | null }) {
  return <div aria-live="polite">{notice && <Alert tone={notice.tone}>{notice.text}</Alert>}</div>;
}
