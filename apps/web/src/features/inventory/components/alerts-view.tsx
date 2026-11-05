"use client";

import { Plus, Truck, Warning, XCircle } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { DataTable, type Column } from "@/components/data/data-table";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { amountToCents } from "@/features/catalog/api";
import { inventoryApi, inventoryKeys, type StockAlert } from "../api";
import { inventoryCopy } from "../copy";
import type { ProductFilter } from "./movements-view";

const copy = inventoryCopy.alerts;

/** Products to reorder, grouped by the supplier of their last received purchase. */
export function AlertsView({ canBuy, onOpenKardex }: { canBuy: boolean; onOpenKardex: (p: ProductFilter) => void }) {
  const alerts = useQuery({ queryKey: inventoryKeys.alerts, queryFn: inventoryApi.alerts });
  const showCost = alerts.data?.some((a) => a.lastCost !== undefined) ?? false;

  const groups = new Map<string, { name: string | null; items: StockAlert[] }>();
  for (const alert of alerts.data ?? []) {
    const key = alert.supplier?.id ?? "none";
    const group = groups.get(key) ?? { name: alert.supplier?.name ?? null, items: [] };
    group.items.push(alert);
    groups.set(key, group);
  }

  const columns: Column<StockAlert>[] = [
    {
      header: copy.columns.product,
      skeleton: 70,
      cell: (a) => (
        <button type="button" className="min-w-0 text-left" onClick={() => onOpenKardex(a.product)}>
          <span className="block truncate font-medium hover:underline">{a.product.name}</span>
          <span className="block font-mono text-caption tracking-wide text-ink-muted">{a.product.sku}</span>
        </button>
      ),
    },
    { header: copy.columns.category, skeleton: 40, cell: (a) => <span className="text-label">{a.category}</span> },
    { header: copy.columns.stock, className: "text-right", skeleton: 30, cell: (a) => <LevelBadge alert={a} /> },
    { header: copy.columns.min, className: "text-right", skeleton: 20, cell: (a) => <span className="font-mono tabular text-label text-ink-muted">{a.minStock}</span> },
    { header: copy.columns.reorder, className: "text-right", skeleton: 20, cell: (a) => <span className="font-mono tabular text-label text-ink-muted">{a.reorderPoint}</span> },
    ...(showCost
      ? [{ header: copy.columns.lastCost, className: "text-right", skeleton: 30, cell: (a: StockAlert) => (a.lastCost ? <Money cents={amountToCents(a.lastCost)} className="font-mono text-label" /> : <span className="text-ink-subtle">—</span>) }]
      : []),
  ];

  if (alerts.isPending || alerts.isError || groups.size === 0) {
    return <DataTable columns={columns} rows={alerts.data} rowKey={(a) => a.product.id} loading={alerts.isPending} error={alerts.isError} onRetry={() => alerts.refetch()} empty={copy.empty} />;
  }

  return (
    <div className="space-y-6">
      {[...groups.entries()].map(([key, group]) => (
        <section key={key} className="space-y-2">
          <div className="flex items-center justify-between gap-3">
            <h2 className="flex items-center gap-2 font-semibold">
              <Truck size={16} className="text-ink-subtle" aria-hidden />
              {group.name ?? copy.noSupplier}
              <span className="text-label font-normal text-ink-muted">· {copy.count(group.items.length)}</span>
            </h2>
            {canBuy && (
              <Link href="/compras/nueva" className={buttonVariants({ size: "sm" })}>
                <Plus size={14} weight="bold" aria-hidden />
                {copy.buy}
              </Link>
            )}
          </div>
          <DataTable columns={columns} rows={group.items} rowKey={(a) => a.product.id} loading={false} error={false} onRetry={() => undefined} empty={copy.empty} minWidth={720} />
        </section>
      ))}
    </div>
  );
}

function LevelBadge({ alert }: { alert: StockAlert }) {
  if (alert.level === "reorder") return <span className="font-mono tabular text-label">{alert.stock}</span>;
  const out = alert.level === "out";
  return (
    <Badge tone={out ? "danger" : "warning"} title={copy.level[alert.level]}>
      {out ? <XCircle size={12} weight="bold" aria-hidden /> : <Warning size={12} weight="bold" aria-hidden />}
      <span className="tabular">{alert.stock}</span>
      <span className="sr-only">{copy.level[alert.level]}</span>
    </Badge>
  );
}
