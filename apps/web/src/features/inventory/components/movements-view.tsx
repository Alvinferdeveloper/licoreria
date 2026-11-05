"use client";

import { X } from "@phosphor-icons/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { Pagination } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Input, inputClassName } from "@/components/ui/field";
import { amountToCents } from "@/features/catalog/api";
import { ProductPicker } from "@/features/catalog/components/product-picker";
import { cn } from "@/lib/cn";
import { formatDateTime } from "@/lib/format/date";
import { inventoryApi, inventoryKeys, type Movement, type MovementFilters } from "../api";
import { inventoryCopy } from "../copy";

const copy = inventoryCopy.movements;

export interface ProductFilter {
  id: string;
  sku: string;
  name: string;
}

/** Kardex: every stock movement, newest first, with the balance after each one. */
export function MovementsView({ product, onProduct }: { product: ProductFilter | null; onProduct: (p: ProductFilter | null) => void }) {
  const [type, setType] = useState<MovementFilters["type"]>("all");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [page, setPage] = useState(1);
  const filters: MovementFilters = { productId: product?.id ?? "", type, from, to, page };
  const movements = useQuery({ queryKey: inventoryKeys.movements(filters), queryFn: () => inventoryApi.movements(filters), placeholderData: keepPreviousData });
  const showCost = movements.data?.data.some((m) => m.unitCost !== undefined) ?? false;
  const reset = <T,>(set: (v: T) => void) => (v: T) => {
    set(v);
    setPage(1);
  };

  const columns: Column<Movement>[] = [
    { header: copy.columns.date, skeleton: 40, cell: (m) => <span className="font-mono tabular text-label whitespace-nowrap">{formatDateTime(m.createdAt)}</span> },
    ...(product
      ? []
      : [
          {
            header: copy.columns.product,
            skeleton: 60,
            cell: (m: Movement) => (
              <button type="button" className="min-w-0 text-left" onClick={() => reset(onProduct)(m.product)}>
                <span className="block truncate font-medium hover:underline">{m.product.name}</span>
                <span className="block font-mono text-caption tracking-wide text-ink-muted">{m.product.sku}</span>
              </button>
            ),
          },
        ]),
    { header: copy.columns.type, skeleton: 40, cell: (m) => <span className="text-label whitespace-nowrap">{inventoryCopy.types[m.type]}</span> },
    {
      header: copy.columns.quantity,
      className: "text-right",
      skeleton: 20,
      cell: (m) => <span className={cn("font-mono tabular font-medium", m.quantity < 0 && "text-danger")}>{m.quantity > 0 ? `+${m.quantity}` : `−${Math.abs(m.quantity)}`}</span>,
    },
    { header: copy.columns.stock, className: "text-right", skeleton: 20, cell: (m) => <span className="font-mono tabular">{m.stockAfter}</span> },
    ...(showCost
      ? [{ header: copy.columns.cost, className: "text-right", skeleton: 30, cell: (m: Movement) => (m.unitCost && Number(m.unitCost) > 0 ? <Money cents={amountToCents(m.unitCost)} className="font-mono text-label" /> : <span className="text-ink-subtle">—</span>) }]
      : []),
    {
      header: copy.columns.detail,
      className: "w-[32%]",
      skeleton: 60,
      cell: (m) => (
        <div className="min-w-0 text-label">
          {m.sourceType === "purchase" && m.sourceId ? (
            <Link href={`/compras/${m.sourceId}`} className="hover:underline">{m.reason}</Link>
          ) : (
            <span>{m.reason || "—"}</span>
          )}
          <span className="block text-caption text-ink-muted">{copy.by(m.user.name)}</span>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        {product ? (
          <div className="flex h-10 items-center gap-2 rounded-md border border-line bg-surface pr-1 pl-3">
            <span className="text-label font-medium">{product.name}</span>
            <span className="font-mono text-caption text-ink-muted">{product.sku}</span>
            <Button variant="ghost" size="icon-sm" aria-label={copy.clearProduct} title={copy.clearProduct} onClick={() => reset(onProduct)(null)}>
              <X size={14} />
            </Button>
          </div>
        ) : (
          <ProductPicker placeholder={copy.product} onPick={(p) => reset(onProduct)({ id: p.id, sku: p.sku, name: p.name })} className="w-full max-w-xs" />
        )}
        <select value={type} onChange={(e) => reset(setType)(e.target.value as MovementFilters["type"])} aria-label={copy.columns.type} className={cn(inputClassName, "w-auto")}>
          <option value="all">{copy.allTypes}</option>
          {Object.entries(inventoryCopy.types).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
        <label className="flex items-center gap-2 text-label text-ink-muted">
          {copy.from}
          <Input type="date" value={from} max={to || undefined} onChange={(e) => reset(setFrom)(e.target.value)} className="w-auto" />
        </label>
        <label className="flex items-center gap-2 text-label text-ink-muted">
          {copy.to}
          <Input type="date" value={to} min={from || undefined} onChange={(e) => reset(setTo)(e.target.value)} className="w-auto" />
        </label>
      </div>
      <DataTable
        columns={columns}
        rows={movements.data?.data}
        rowKey={(m) => m.id}
        loading={movements.isPending}
        error={movements.isError}
        onRetry={() => movements.refetch()}
        empty={copy.empty}
        minWidth={900}
      />
      <Pagination meta={movements.data?.meta} onPage={setPage} />
    </div>
  );
}
