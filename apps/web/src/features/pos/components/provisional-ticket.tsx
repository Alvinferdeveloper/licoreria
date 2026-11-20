"use client";

import { createPortal } from "react-dom";
import type { StoreSettings } from "@/features/sales/api";
import { invoiceCopy } from "@/features/sales/copy";
import { formatDateTime } from "@/lib/format/date";
import { formatMoney, formatRate } from "@/lib/money";
import { posCopy } from "../copy";
import type { OfflineReceipt } from "../offline/outbox";

const copy = invoiceCopy.ticketView;

/**
 * Receipt for a sale saved without connection. It renders into <body> and shows only when
 * printing, because without connection the ticket page can't be opened in a new window.
 */
export function ProvisionalTicket({ receipt, store }: { receipt: OfflineReceipt; store: StoreSettings | undefined }) {
  const row = "flex justify-between gap-2";
  return createPortal(
    <article className="hidden bg-white px-3 py-4 font-mono text-black print:block" style={{ width: "80mm", fontSize: "11.5px", lineHeight: 1.35 }}>
      <style>{"@media print { @page { size: 80mm auto; margin: 0; } }"}</style>
      <header className="text-center">
        {store && <p className="font-sans font-bold" style={{ fontSize: "15px" }}>{store.business.name}</p>}
        {store?.business.taxId && <p>{copy.taxId(store.business.taxId)}</p>}
        <p className="mt-1 font-bold">{posCopy.offline.ticket.title}</p>
      </header>
      <Rule />
      <p className="font-bold">{receipt.code}</p>
      <p>{formatDateTime(receipt.createdAt)}</p>
      <p>{copy.cashier(receipt.cashierName)}</p>
      {receipt.customer?.taxId && <p>{copy.customer(`${receipt.customer.name} · ${receipt.customer.taxId}`)}</p>}
      <Rule />
      <ul className="space-y-1">
        {receipt.lines.map((l, i) => (
          <li key={i}>
            <p>{l.name}</p>
            <p className={row}>
              <span>
                {l.quantity}
                {l.unitsPerItem > 1 ? `×${l.unitsPerItem}u` : ""} × {formatMoney(l.unitPrice).replace("C$ ", "")}
              </span>
              <span>{formatMoney(l.subtotal)}</span>
            </p>
          </li>
        ))}
      </ul>
      <Rule />
      <p className={row}><span>{copy.subtotal}</span><span>{formatMoney(receipt.subtotal)}</span></p>
      {receipt.discount > 0 && <p className={row}><span>{copy.discount}</span><span>-{formatMoney(receipt.discount)}</span></p>}
      <p className={row}><span>{copy.tax}</span><span>{formatMoney(receipt.tax)}</span></p>
      <p className={`${row} font-bold`} style={{ fontSize: "14px" }}><span>{copy.total}</span><span>{formatMoney(receipt.total)}</span></p>
      <Rule />
      {receipt.payments.map((p, i) => (
        <p key={i} className={row}><span>{copy.paid}</span><span>{formatMoney(p.amount, p.currency)}</span></p>
      ))}
      <p className={row}><span>{copy.change}</span><span>{formatMoney(receipt.change)}</span></p>
      {receipt.payments.some((p) => p.currency === "USD") && <p>{copy.rate(formatRate(receipt.rate))}</p>}
      <Rule />
      <footer className="text-center">
        <p>{posCopy.offline.ticket.note}</p>
        <p className="font-bold">{copy.thanks}</p>
        <p>{copy.age}</p>
      </footer>
    </article>,
    document.body,
  );
}

function Rule() {
  return <hr className="my-2 border-0 border-t border-dashed border-black" />;
}
