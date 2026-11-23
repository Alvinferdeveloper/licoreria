"use client";

import { Printer } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import QRCode from "qrcode";
import { useEffect, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { ApiError } from "@/lib/api/client";
import { formatDateTime } from "@/lib/format/date";
import { amountToCents, formatMoney, formatRate, nioToUsd } from "@/lib/money";
import { salesApi, salesKeys, type Sale, type StoreSettings } from "../api";
import { invoiceCopy, paymentReference, salesCopy } from "../copy";

const copy = invoiceCopy.ticketView;
type Width = "80" | "58";
const WIDTHS = [
  { value: "80", label: "80 mm" },
  { value: "58", label: "58 mm" },
] as const;

/** Thermal-style receipt; printing uses the browser dialog with a page as wide as the paper. */
export function TicketView({ id }: { id: string }) {
  const sale = useQuery({ queryKey: salesKeys.detail(id), queryFn: () => salesApi.get(id) });
  const store = useQuery({ queryKey: salesKeys.store, queryFn: salesApi.store });
  const [width, setWidth] = useState<Width>("80");
  const printed = useRef(false);
  const ready = sale.data && store.data;

  useEffect(() => {
    if (!ready || printed.current || !new URLSearchParams(window.location.search).has("print")) return;
    printed.current = true;
    const timer = setTimeout(() => window.print(), 300);
    return () => clearTimeout(timer);
  }, [ready]);

  if (sale.isError) return <Alert className="m-6">{sale.error instanceof ApiError ? sale.error.message : "No pudimos cargar la venta."}</Alert>;
  if (!sale.data || !store.data) return <p className="p-6 text-ink-muted">Cargando…</p>;

  return (
    <div className="min-h-dvh bg-sunken py-6 print:bg-white print:py-0">
      <style>{`@media print { @page { size: ${width}mm auto; margin: 0; } }`}</style>
      <div className="mx-auto mb-4 flex max-w-sm items-center justify-between gap-3 px-4 print:hidden">
        <SegmentedControl label={copy.width} value={width} options={WIDTHS} onChange={setWidth} className="w-44" />
        <Button variant="primary" onClick={() => window.print()}>
          <Printer size={16} aria-hidden />
          {copy.print}
        </Button>
      </div>
      <Ticket sale={sale.data} store={store.data} width={width} />
    </div>
  );
}

function Ticket({ sale, store, width }: { sale: Sale; store: StoreSettings; width: Width }) {
  const [qr, setQr] = useState("");
  useEffect(() => {
    QRCode.toString(sale.invoice, { type: "svg", margin: 0, errorCorrectionLevel: "M" }).then(setQr, () => setQr(""));
  }, [sale.invoice]);

  const rate = Number(sale.exchangeRate);
  const nio = (amount: string) => formatMoney(amountToCents(amount));
  const row = "flex justify-between gap-2";
  const small = width === "58";

  return (
    <article
      className="mx-auto bg-white px-3 py-4 font-mono text-black shadow-(--shadow-float-low) print:shadow-none"
      style={{ width: `${width}mm`, fontSize: small ? "10px" : "11.5px", lineHeight: 1.35 }}
    >
      <header className="text-center">
        <p className="font-sans font-bold" style={{ fontSize: small ? "13px" : "15px" }}>{store.business.name}</p>
        {store.business.taxId && <p>{copy.taxId(store.business.taxId)}</p>}
        {store.business.phone && <p>{store.business.phone}</p>}
      </header>
      <Rule />
      <p className="font-bold">{sale.invoice}</p>
      <p>{formatDateTime(sale.createdAt)}</p>
      <p>{copy.cashier(sale.cashier.name)}</p>
      {sale.customer.taxId && <p>{copy.customer(`${sale.customer.name} · ${sale.customer.taxId}`)}</p>}
      {sale.status === "voided" && <p className="mt-1 text-center font-bold">*** {copy.voided} ***</p>}
      <Rule />
      <ul className="space-y-1">
        {sale.items?.map((i) => (
          <li key={i.id}>
            <p>{i.name}</p>
            <p className={row}>
              <span>
                {i.quantity}
                {i.unitsPerItem > 1 ? `×${i.unitsPerItem}u` : ""} × {formatMoney(amountToCents(i.unitPrice)).replace("C$ ", "")}
              </span>
              <span>{nio(i.subtotal)}</span>
            </p>
          </li>
        ))}
      </ul>
      <Rule />
      <p className={row}><span>{copy.subtotal}</span><span>{nio(sale.subtotal)}</span></p>
      {amountToCents(sale.discount) > 0 && <p className={row}><span>{copy.discount}</span><span>-{nio(sale.discount)}</span></p>}
      <p className={row}><span>{copy.tax}</span><span>{nio(sale.tax)}</span></p>
      <p className={`${row} font-bold`} style={{ fontSize: small ? "12px" : "14px" }}>
        <span>{copy.total}</span>
        <span>{nio(sale.total)}</span>
      </p>
      {rate > 0 && <p className="text-right">{copy.usd(formatMoney(nioToUsd(amountToCents(sale.total), rate), "USD"))}</p>}
      <Rule />
      {sale.payments?.map((p, i) => (
        <div key={i}>
          <p className={row}>
            <span>{p.method === "cash" ? copy.paid : salesCopy.detail.methods[p.method]}</span>
            <span>{formatMoney(amountToCents(p.amount), p.currency)}</span>
          </p>
          {p.reference && <p>{paymentReference(p.method, p.reference)}</p>}
        </div>
      ))}
      <p className={row}><span>{copy.change}</span><span>{nio(sale.changeNio)}</span></p>
      {sale.payments?.some((p) => p.currency === "USD") && <p>{copy.rate(formatRate(rate))}</p>}
      <Rule />
      {qr && <div className="mx-auto my-2 w-20" aria-label={sale.invoice} dangerouslySetInnerHTML={{ __html: qr }} />}
      <footer className="text-center">
        <p className="font-bold">{copy.thanks}</p>
        {store.invoice.footerNote && <p>{store.invoice.footerNote}</p>}
        <p>{copy.age}</p>
        <p>{copy.fiscal}</p>
      </footer>
    </article>
  );
}

function Rule() {
  return <hr className="my-2 border-0 border-t border-dashed border-black" />;
}
