"use client";

import { FilePlus, Truck } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { purchaseKeys, purchasesApi } from "@/features/purchases/api";
import { todayISO } from "@/lib/format/date";
import { amountToCents, sumCents } from "@/lib/money";
import { inventoryApi, inventoryKeys, type Coverage, type ForecastItem } from "../api";
import { inventoryCopy } from "../copy";
import type { ProductFilter } from "./movements-view";

const copy = inventoryCopy.forecast;
const URGENCY_TONE = { out: "danger", critical: "warning", soon: "amber", min: "neutral" } as const;

const cost = (f: ForecastItem) => (f.lastCost ? amountToCents(f.lastCost) * f.suggested : 0);

interface ForecastViewProps {
  canBuy: boolean;
  onOpenKardex: (p: ProductFilter) => void;
  onFailure: (error: unknown) => void;
}

/** What will run out at the current sales rhythm, and how much to order, grouped by supplier. */
export function ForecastView({ canBuy, onOpenKardex, onFailure }: ForecastViewProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [coverage, setCoverage] = useState<Coverage>("14");
  const forecast = useQuery({ queryKey: inventoryKeys.forecast(coverage), queryFn: () => inventoryApi.forecast(coverage) });
  const items = forecast.data?.data;
  const showCost = items?.some((f) => f.lastCost !== undefined) ?? false;

  const draft = useMutation({
    mutationFn: (group: { supplierId: string; items: ForecastItem[] }) =>
      purchasesApi.create({
        supplierId: group.supplierId,
        supplierInvoiceNumber: "",
        purchaseDate: todayISO(),
        notes: copy.draftNote(Number(coverage)),
        items: group.items.map((f) => ({ productId: f.product.id, quantity: f.suggested, unitCost: Number(f.lastCost ?? 0).toFixed(2) })),
      }),
    onSuccess: (purchase) => {
      void queryClient.invalidateQueries({ queryKey: purchaseKeys.all });
      router.push(`/compras/${purchase.id}`);
    },
    onError: onFailure,
  });

  const groups = new Map<string, { supplier: ForecastItem["supplier"]; items: ForecastItem[] }>();
  for (const f of items ?? []) {
    const key = f.supplier?.id ?? "none";
    const group = groups.get(key) ?? { supplier: f.supplier, items: [] };
    group.items.push(f);
    groups.set(key, group);
  }
  const count = (u: ForecastItem["urgency"]) => items?.filter((f) => f.urgency === u).length ?? 0;

  const columns: Column<ForecastItem>[] = [
    {
      header: copy.columns.product,
      skeleton: 70,
      cell: (f) => (
        <button type="button" className="min-w-0 text-left" onClick={() => onOpenKardex(f.product)}>
          <span className="block truncate font-medium hover:underline">{f.product.name}</span>
          <span className="block font-mono text-caption tracking-wide text-ink-muted">{f.product.sku}</span>
        </button>
      ),
    },
    { header: copy.columns.trend, skeleton: 40, cell: (f) => <Sparkline values={f.history} /> },
    { header: copy.columns.rate, className: "text-right", skeleton: 20, cell: (f) => <span className="font-mono tabular text-label">{copy.rate(f.dailyRate)}</span> },
    { header: copy.columns.stock, className: "text-right", skeleton: 20, cell: (f) => <span className="font-mono tabular text-label">{f.stock}</span> },
    {
      header: copy.columns.left,
      skeleton: 40,
      cell: (f) => (
        <span className="flex flex-col items-start gap-0.5">
          <Badge tone={URGENCY_TONE[f.urgency]}>{copy.urgency[f.urgency]}</Badge>
          <span className="text-caption text-ink-muted">{f.urgency === "out" ? "" : copy.left(f.daysLeft)}</span>
        </span>
      ),
    },
    {
      header: copy.columns.suggested,
      className: "text-right",
      skeleton: 30,
      cell: (f) => (
        <span className="block text-right">
          <span className="font-mono font-semibold tabular">{f.suggested}</span>
          {f.packSize > 1 && <span className="block text-caption text-ink-muted">{copy.packs(f.suggested, f.packSize, f.packName)}</span>}
        </span>
      ),
    },
    ...(showCost
      ? [{ header: copy.columns.cost, className: "text-right", skeleton: 30, cell: (f: ForecastItem) => (f.lastCost ? <Money cents={cost(f)} className="font-mono text-label" /> : <span className="text-ink-subtle">—</span>) }]
      : []),
  ];

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <p className="max-w-2xl text-label text-ink-muted">{copy.intro(forecast.data?.historyDays ?? 28)}</p>
        <SegmentedControl label={copy.coverage} value={coverage} options={copy.coverages} onChange={setCoverage} className="w-64" />
      </div>

      {items && items.length > 0 && (
        <dl className="grid divide-line rounded-lg border border-line bg-surface sm:grid-cols-2 sm:divide-x xl:grid-cols-5">
          <Fact label={copy.summary.out} value={count("out")} />
          <Fact label={copy.summary.critical} value={count("critical")} />
          <Fact label={copy.summary.soon} value={count("soon")} />
          <Fact label={copy.summary.min} value={count("min")} />
          {showCost && <Fact label={copy.summary.cost} value={<Money cents={sumCents(items.map(cost))} />} />}
        </dl>
      )}

      {forecast.isPending || forecast.isError || groups.size === 0 ? (
        <DataTable columns={columns} rows={items} rowKey={(f) => f.product.id} loading={forecast.isPending} error={forecast.isError} onRetry={() => forecast.refetch()} empty={copy.empty} />
      ) : (
        [...groups.entries()].map(([key, group]) => {
          const supplier = group.supplier;
          return (
            <section key={key} className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h2 className="flex items-center gap-2 font-semibold">
                  <Truck size={16} className="text-ink-subtle" aria-hidden />
                  {supplier?.name ?? copy.noSupplier}
                  <span className="text-label font-normal text-ink-muted">· {inventoryCopy.alerts.count(group.items.length)}</span>
                </h2>
                <div className="flex items-center gap-4">
                  {showCost && (
                    <span className="text-label text-ink-muted">
                      {copy.total} <Money cents={sumCents(group.items.map(cost))} className="font-mono font-semibold text-ink" />
                    </span>
                  )}
                  {canBuy && supplier && (
                    <Button
                      size="sm"
                      loading={draft.isPending && draft.variables?.supplierId === supplier.id}
                      disabled={draft.isPending}
                      onClick={() => draft.mutate({ supplierId: supplier.id, items: group.items })}
                    >
                      <FilePlus size={14} weight="bold" aria-hidden />
                      {copy.draft}
                    </Button>
                  )}
                </div>
              </div>
              <DataTable columns={columns} rows={group.items} rowKey={(f) => f.product.id} loading={false} error={false} onRetry={() => undefined} empty={copy.empty} minWidth={860} />
            </section>
          );
        })
      )}
    </div>
  );
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="px-5 py-4">
      <dt className="text-caption text-ink-muted">{label}</dt>
      <dd className="mt-1 text-title font-semibold tabular">{value}</dd>
    </div>
  );
}

/** Daily units for the last 28 days as a tiny line; the last week is drawn in the accent. */
function Sparkline({ values }: { values: number[] }) {
  const width = 96;
  const height = 24;
  const top = Math.max(1, ...values);
  const step = width / Math.max(1, values.length - 1);
  const points = values.map((v, i) => `${(i * step).toFixed(1)},${(height - 2 - (v / top) * (height - 4)).toFixed(1)}`);
  const split = Math.max(0, values.length - 7);
  const total = values.reduce((a, b) => a + b, 0);
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label={copy.trendLabel(total)} className="overflow-visible">
      <polyline points={points.slice(0, split + 1).join(" ")} fill="none" stroke="var(--ink-subtle)" strokeWidth={1.5} strokeLinejoin="round" />
      <polyline points={points.slice(split).join(" ")} fill="none" stroke="var(--amber)" strokeWidth={1.75} strokeLinejoin="round" />
    </svg>
  );
}
