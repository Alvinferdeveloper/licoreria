"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { formatDate } from "@/lib/format/date";
import { amountToCents, sumCents } from "@/lib/money";
import { inventoryApi, inventoryKeys, type ExpiringLot, type ExpiryWindow } from "../api";
import { inventoryCopy } from "../copy";
import type { ProductFilter } from "./movements-view";

const copy = inventoryCopy.expiry;
const value = (l: ExpiringLot) => (l.unitCost ? amountToCents(l.unitCost) * l.remaining : 0);

interface ExpiryViewProps {
  canAdjust: boolean;
  onOpenKardex: (p: ProductFilter) => void;
  onDone: (message: string) => void;
  onFailure: (error: unknown) => void;
}

/** Lots expired or about to, with what is left of each and a one-step write-off as shrinkage. */
export function ExpiryView({ canAdjust, onOpenKardex, onDone, onFailure }: ExpiryViewProps) {
  const queryClient = useQueryClient();
  const [range, setRange] = useState<ExpiryWindow>("30");
  const lots = useQuery({ queryKey: inventoryKeys.expiring(range), queryFn: () => inventoryApi.expiring(range) });
  const items = lots.data;
  const showValue = items?.some((l) => l.unitCost !== undefined) ?? false;

  const writeOff = useMutation({
    mutationFn: (l: ExpiringLot) =>
      inventoryApi.adjust({ productId: l.product.id, kind: "shrinkage", quantity: l.remaining, reason: copy.reason(formatDate(l.expiresOn)) }).then(() => l),
    onSuccess: (l) => {
      onDone(copy.done(l.product.name, l.remaining));
      void queryClient.invalidateQueries({ queryKey: inventoryKeys.all });
    },
    onError: onFailure,
  });

  const columns: Column<ExpiringLot>[] = [
    {
      header: copy.columns.product,
      skeleton: 70,
      cell: (l) => (
        <button type="button" className="min-w-0 text-left" onClick={() => onOpenKardex(l.product)}>
          <span className="block truncate font-medium hover:underline">{l.product.name}</span>
          <span className="block font-mono text-caption tracking-wide text-ink-muted">{l.product.sku}</span>
        </button>
      ),
    },
    {
      header: copy.columns.lot,
      skeleton: 50,
      cell: (l) => <span className="text-label text-ink-muted">{copy.lot(l.supplier.name, formatDate(l.receivedAt.slice(0, 10)))}</span>,
    },
    {
      header: copy.columns.expires,
      skeleton: 40,
      cell: (l) => (
        <span className="flex flex-col items-start gap-0.5">
          <Badge tone={l.daysLeft < 0 ? "danger" : l.daysLeft <= 7 ? "warning" : "neutral"}>{copy.status(l.daysLeft)}</Badge>
          <span className="font-mono text-caption tabular text-ink-muted">{formatDate(l.expiresOn)}</span>
        </span>
      ),
    },
    { header: copy.columns.remaining, className: "text-right", skeleton: 20, cell: (l) => <span className="font-mono font-semibold tabular">{l.remaining}</span> },
    ...(showValue
      ? [{ header: copy.columns.value, className: "text-right", skeleton: 30, cell: (l: ExpiringLot) => <Money cents={value(l)} className="font-mono text-label" /> }]
      : []),
    ...(canAdjust
      ? [
          {
            header: copy.columns.action,
            className: "text-right",
            skeleton: 0,
            cell: (l: ExpiringLot) =>
              l.daysLeft < 0 && (
                <ConfirmButton size="sm" confirmLabel={copy.confirm} loading={writeOff.isPending && writeOff.variables?.lineId === l.lineId} onConfirm={() => writeOff.mutate(l)}>
                  {copy.writeOff}
                </ConfirmButton>
              ),
          },
        ]
      : []),
  ];

  const expired = items?.filter((l) => l.daysLeft < 0) ?? [];
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <p className="max-w-2xl text-label text-ink-muted">{copy.intro}</p>
        <SegmentedControl label={copy.window} value={range} options={copy.windows} onChange={setRange} className="w-64" />
      </div>
      {items && items.length > 0 && (
        <dl className="grid divide-line rounded-lg border border-line bg-surface sm:grid-cols-2 sm:divide-x xl:grid-cols-4">
          <Fact label={copy.summary.expired} value={expired.length} warn={expired.length > 0} />
          <Fact label={copy.summary.soon} value={items.length - expired.length} />
          <Fact label={copy.summary.units} value={items.reduce((n, l) => n + l.remaining, 0)} />
          {showValue && <Fact label={copy.summary.value} value={<Money cents={sumCents(items.map(value))} />} />}
        </dl>
      )}
      <DataTable columns={columns} rows={items} rowKey={(l) => l.lineId} loading={lots.isPending} error={lots.isError} onRetry={() => lots.refetch()} empty={copy.empty} minWidth={860} />
    </div>
  );
}

function Fact({ label, value, warn }: { label: string; value: React.ReactNode; warn?: boolean }) {
  return (
    <div className="px-5 py-4">
      <dt className="text-caption text-ink-muted">{label}</dt>
      <dd className={warn ? "mt-1 text-title font-semibold tabular text-danger" : "mt-1 text-title font-semibold tabular"}>{value}</dd>
    </div>
  );
}
