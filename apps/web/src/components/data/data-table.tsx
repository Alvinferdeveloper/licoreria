"use client";

import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

export interface Column<T> {
  header: ReactNode;
  cell: (row: T) => ReactNode;
  className?: string;
  /** Skeleton bar width while loading, as a percentage of the cell. 0 renders nothing. */
  skeleton?: number;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[] | undefined;
  rowKey: (row: T) => string;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
  empty: { title: string; body: string };
  rowClassName?: (row: T) => string | undefined;
  /** Mouse shortcut only; keep a link or button in the row for keyboard users. */
  onRowClick?: (row: T) => void;
  minWidth?: number;
}

/** Table with the standard loading skeleton, error and empty states. */
export function DataTable<T>({ columns, rows, rowKey, loading, error, onRetry, empty, rowClassName, onRowClick, minWidth = 760 }: DataTableProps<T>) {
  return (
    <div className="overflow-x-auto rounded-lg border border-line bg-surface">
      <table className="w-full border-collapse text-left" style={{ minWidth }}>
        <thead className="bg-chrome text-caption text-ink-muted">
          <tr>
            {columns.map((col, i) => (
              <th key={i} className={cn("px-4 py-2.5 font-medium", col.className)}>
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {loading &&
            Array.from({ length: 5 }, (_, i) => (
              <tr key={i} className="h-14" aria-hidden>
                {columns.map((col, j) => (
                  <td key={j} className="px-4">
                    {(col.skeleton ?? 50) > 0 && (
                      <span className="block h-3 animate-pulse rounded-xs bg-sunken" style={{ width: `${col.skeleton ?? 50}%` }} />
                    )}
                  </td>
                ))}
              </tr>
            ))}
          {rows?.map((row) => (
            <tr
              key={rowKey(row)}
              onClick={onRowClick && (() => onRowClick(row))}
              className={cn("group h-14 transition-colors hover:bg-chrome/60", onRowClick && "cursor-pointer", rowClassName?.(row))}
            >
              {columns.map((col, j) => (
                <td key={j} className={cn("px-4", col.className)}>
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      {error && (
        <div className="flex items-center justify-between gap-4 px-4 py-6">
          <p className="text-ink-muted">No pudimos cargar la información.</p>
          <Button size="sm" onClick={onRetry}>
            Reintentar
          </Button>
        </div>
      )}
      {rows && rows.length === 0 && (
        <div className="px-4 py-12 text-center">
          <p className="font-semibold">{empty.title}</p>
          <p className="text-ink-muted">{empty.body}</p>
        </div>
      )}
    </div>
  );
}

/** Row actions that appear on hover or keyboard focus on desktop and stay visible on touch. */
export function RowActions({ children }: { children: ReactNode }) {
  return (
    <div className="flex justify-end gap-1 transition-opacity md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100">
      {children}
    </div>
  );
}
