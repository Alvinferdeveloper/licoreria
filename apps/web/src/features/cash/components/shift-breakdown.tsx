"use client";

import { useState } from "react";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { formatDateTime } from "@/lib/format/date";
import { amountToCents, type CurrencyCode } from "@/lib/money";
import { cn } from "@/lib/cn";
import { paymentReference } from "@/features/sales/copy";
import type { NonCashPayment, Shift, ShiftSummary } from "../api";
import { cashCopy } from "../copy";

const copy = cashCopy.shift;

/** What the drawer should hold per currency and how it got there. */
export function ShiftBreakdown({ shift, summary }: { shift: Shift; summary: ShiftSummary }) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <CurrencyColumn
          title={copy.nio}
          currency="NIO"
          expected={summary.expectedNio}
          rows={[
            [copy.breakdown.opening, shift.openingNio, 1],
            [copy.breakdown.cashSales, summary.cashSalesNio, 1],
            [copy.breakdown.change, summary.changeNio, -1],
            [copy.breakdown.income, summary.incomeNio, 1],
            [copy.breakdown.expense, summary.expenseNio, -1],
          ]}
        />
        <CurrencyColumn
          title={copy.usd}
          currency="USD"
          expected={summary.expectedUsd}
          rows={[
            [copy.breakdown.opening, shift.openingUsd, 1],
            [copy.breakdown.cashSales, summary.cashSalesUsd, 1],
            [copy.breakdown.change, summary.changeUsd, -1],
            [copy.breakdown.income, summary.incomeUsd, 1],
            [copy.breakdown.expense, summary.expenseUsd, -1],
          ]}
        />
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-1 rounded-md bg-sunken px-4 py-3 text-label">
        <span className="text-ink-muted">{copy.sales(summary.salesCount)}</span>
        <span className="text-ink-muted">{copy.other}:</span>
        <span>
          {copy.card} <Money cents={amountToCents(summary.cardNio)} className="font-mono" />
        </span>
        <span>
          {copy.transfer} <Money cents={amountToCents(summary.transferNio)} className="font-mono" />
        </span>
      </div>
      {shift.nonCash && shift.nonCash.length > 0 && <NonCashList payments={shift.nonCash} />}
    </div>
  );
}

function NonCashList({ payments }: { payments: NonCashPayment[] }) {
  const [open, setOpen] = useState(false);
  return (
    <section className="rounded-lg border border-line bg-surface">
      <header className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
        <div>
          <h3 className="text-label font-semibold">{copy.nonCash.title}</h3>
          <p className="text-caption text-ink-muted">{copy.nonCash.hint}</p>
        </div>
        <Button variant="ghost" size="sm" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
          {open ? copy.nonCash.hide : copy.nonCash.show(payments.length)}
        </Button>
      </header>
      {open && (
        <ul className="divide-y divide-line border-t border-line text-label">
          {payments.map((p, i) => (
            <li key={`${p.saleId}-${i}`} className="grid grid-cols-[6rem_1fr_auto] items-center gap-3 px-4 py-2">
              <span className="text-ink-muted">{p.method === "card" ? copy.card : copy.transfer}</span>
              <span className="min-w-0 truncate font-mono">
                {p.reference ? paymentReference(p.method, p.reference) : <span className="text-ink-subtle">{copy.nonCash.noReference}</span>}
                <span className="text-ink-subtle"> · {p.invoice} · {formatDateTime(p.createdAt)}</span>
              </span>
              <Money cents={amountToCents(p.amount)} currency={p.currency} className="font-mono" />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CurrencyColumn({ title, currency, expected, rows }: { title: string; currency: CurrencyCode; expected: string; rows: [string, string, 1 | -1][] }) {
  return (
    <section className="rounded-lg border border-line bg-surface p-5">
      <p className="text-label text-ink-muted">
        {copy.expected} · {title}
      </p>
      <p className="mt-1 text-headline font-semibold tracking-headline">
        <Money cents={amountToCents(expected)} currency={currency} className="font-mono" />
      </p>
      <dl className="mt-4 space-y-1.5 border-t border-line pt-3 text-label">
        {rows.map(([label, amount, sign]) => (
          <div key={label} className="flex justify-between gap-4">
            <dt className="text-ink-muted">{label}</dt>
            <dd className={cn("font-mono tabular", amountToCents(amount) === 0 && "text-ink-subtle")}>
              {sign < 0 && amountToCents(amount) > 0 ? "− " : ""}
              <Money cents={amountToCents(amount)} currency={currency} />
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
