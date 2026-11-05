import { Money } from "@/components/money";
import { nioToUsd } from "@/lib/money";
import { posCopy } from "../copy";
import type { CartTotals } from "../domain";

interface BillTotalsProps {
  totals: CartTotals;
  rate: number;
  discountLabel?: string;
}

export function BillTotals({ totals, rate, discountLabel }: BillTotalsProps) {
  return (
    <div className="space-y-4">
      <dl className="space-y-1.5 text-ink-muted">
        <div className="flex justify-between">
          <dt>{posCopy.bill.subtotal}</dt>
          <dd><Money cents={totals.subtotal} className="font-mono" /></dd>
        </div>
        {totals.discount > 0 && (
          <div className="flex justify-between">
            <dt>{discountLabel ?? posCopy.bill.discount}</dt>
            <dd className="font-mono">− <Money cents={totals.discount} /></dd>
          </div>
        )}
        <div className="flex justify-between">
          <dt>{posCopy.bill.tax}</dt>
          <dd><Money cents={totals.tax} className="font-mono" /></dd>
        </div>
      </dl>
      <div className="border-t border-line pt-4">
        <p className="text-label text-ink-muted">{posCopy.bill.total}</p>
        <Money cents={totals.total} className="block text-total font-semibold tracking-display" />
        {rate > 0 && <Money cents={nioToUsd(totals.total, rate)} currency="USD" className="mt-1 block text-title text-ink-muted" />}
      </div>
    </div>
  );
}
