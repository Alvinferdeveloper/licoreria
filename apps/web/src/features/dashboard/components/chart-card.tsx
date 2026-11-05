"use client";

import { useState, type ReactNode } from "react";
import { cn } from "@/lib/cn";
import { formatAmount } from "@/lib/money";
import { dashboardCopy as copy } from "../copy";

interface ChartCardProps {
  title: string;
  subtitle: string;
  controls?: ReactNode;
  legend?: ReactNode;
  /** The same data as a table, for screen readers and for reading exact values. */
  table: ReactNode;
  children: ReactNode;
  className?: string;
}

export function ChartCard({ title, subtitle, controls, legend, table, children, className }: ChartCardProps) {
  const [showTable, setShowTable] = useState(false);
  return (
    <section className={cn("flex flex-col rounded-lg border border-line bg-surface p-5", className)}>
      <header className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="font-semibold">{title}</h2>
          <p className="text-caption text-ink-muted">{subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          {controls}
          <button type="button" className="rounded-sm px-1.5 py-0.5 text-caption font-medium text-ink-muted hover:text-ink" onClick={() => setShowTable(!showTable)}>
            {showTable ? copy.table.hide : copy.table.show}
          </button>
        </div>
      </header>
      {legend && !showTable && <div className="mb-3">{legend}</div>}
      <div className="min-h-0 flex-1">{showTable ? <div className="max-h-72 overflow-y-auto">{table}</div> : children}</div>
    </section>
  );
}

export function LegendItem({ color, label, dashed }: { color: string; label: string; dashed?: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-caption text-ink-muted">
      <span
        className="inline-block h-0.5 w-4 rounded-full"
        style={dashed ? { backgroundImage: `linear-gradient(90deg, ${color} 60%, transparent 0)`, backgroundSize: "6px 2px" } : { backgroundColor: color }}
        aria-hidden
      />
      {label}
    </span>
  );
}

export function SwatchItem({ color, label, value }: { color: string; label: string; value?: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-caption text-ink-muted">
      <span className="inline-block size-2.5 rounded-xs" style={{ backgroundColor: color }} aria-hidden />
      <span className="text-ink">{label}</span>
      {value && <span className="font-mono tabular">{value}</span>}
    </span>
  );
}

/** Compact two-or-more column table used as each chart's table view. */
export function DataList({ head, rows }: { head: string[]; rows: (string | number)[][] }) {
  return (
    <table className="w-full text-left text-label">
      <thead className="text-caption text-ink-muted">
        <tr>
          {head.map((h, i) => (
            <th key={h} className={cn("pb-1.5 font-medium", i > 0 && "text-right")}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="divide-y divide-line">
        {rows.map((r, i) => (
          <tr key={i}>
            {r.map((v, j) => (
              <td key={j} className={cn("py-1.5", j > 0 && "text-right font-mono tabular")}>
                {v}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

interface TooltipPayload {
  name?: string | number;
  value?: number | string;
  color?: string;
  dataKey?: string | number;
}

/** Recharts tooltip in the app's tokens; values are córdoba amounts unless `format` says otherwise. */
export function ChartTooltip({ active, payload, label, format = (v) => `C$ ${formatAmount(Math.round(v * 100))}` }: {
  active?: boolean;
  payload?: TooltipPayload[];
  label?: string | number;
  format?: (v: number) => string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-line bg-surface px-3 py-2 text-caption shadow-(--shadow-float-low)">
      {label !== undefined && <p className="mb-1 font-medium text-ink">{label}</p>}
      {payload.map((p) => (
        <p key={String(p.dataKey ?? p.name)} className="flex items-center justify-between gap-4 text-ink-muted">
          <span className="flex items-center gap-1.5">
            <span className="inline-block size-2 rounded-xs" style={{ backgroundColor: p.color }} aria-hidden />
            {p.name}
          </span>
          <span className="font-mono tabular text-ink">{format(Number(p.value))}</span>
        </p>
      ))}
    </div>
  );
}
