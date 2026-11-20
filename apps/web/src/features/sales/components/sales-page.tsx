"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { FilterChips, NoticeBanner, Pagination, SearchInput, useNotice } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { catalogKeys } from "@/features/catalog/api";
import { AuthorizationForm } from "@/features/pos/components/authorization-form";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatDateTime } from "@/lib/format/date";
import { amountToCents, formatRate } from "@/lib/money";
import { salesApi, salesKeys, type Authorization, type Sale, type SaleFilters } from "../api";
import { salesCopy as copy } from "../copy";
import { InvoiceActions } from "./invoice-actions";

export function SalesPage() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<SaleFilters["status"]>("all");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [page, setPage] = useState(1);
  const [openId, setOpenId] = useState<string | null>(null);
  const { notice, success } = useNotice();
  const filters: SaleFilters = { search: useDeferredValue(search), status, from, to, page };
  const sales = useQuery({ queryKey: salesKeys.list(filters), queryFn: () => salesApi.list(filters), placeholderData: keepPreviousData });
  const reset = <T,>(set: (v: T) => void) => (v: T) => {
    set(v);
    setPage(1);
  };

  const columns: Column<Sale>[] = [
    {
      header: copy.columns.invoice,
      skeleton: 30,
      cell: (s) => (
        <button type="button" className="font-mono font-medium tracking-wide hover:underline" onClick={(e) => { e.stopPropagation(); setOpenId(s.id); }}>
          {s.invoice}
        </button>
      ),
    },
    { header: copy.columns.date, skeleton: 40, cell: (s) => <span className="font-mono tabular text-label">{formatDateTime(s.createdAt)}</span> },
    { header: copy.columns.customer, skeleton: 40, cell: (s) => <span className="text-label">{s.customer.name}</span> },
    { header: copy.columns.cashier, skeleton: 40, cell: (s) => <span className="text-label">{s.cashier.name}</span> },
    { header: copy.columns.total, className: "text-right", skeleton: 30, cell: (s) => <Money cents={amountToCents(s.total)} className={cn("font-mono", s.status === "voided" && "line-through")} /> },
    { header: copy.columns.status, skeleton: 30, cell: (s) => (s.status === "voided" ? <Badge tone="danger">{copy.status.voided}</Badge> : <Badge tone="success">{copy.status.completed}</Badge>) },
  ];

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={reset(setSearch)} placeholder={copy.search} />
          <FilterChips label={copy.statusLabel} value={status} options={copy.statusFilters} onChange={reset(setStatus)} />
          <label className="flex items-center gap-2 text-label text-ink-muted">
            {copy.from}
            <Input type="date" value={from} max={to || undefined} onChange={(e) => reset(setFrom)(e.target.value)} className="h-9 w-auto" />
          </label>
          <label className="flex items-center gap-2 text-label text-ink-muted">
            {copy.to}
            <Input type="date" value={to} min={from || undefined} onChange={(e) => reset(setTo)(e.target.value)} className="h-9 w-auto" />
          </label>
        </div>
        <NoticeBanner notice={notice} />
        <DataTable
          columns={columns}
          rows={sales.data?.data}
          rowKey={(s) => s.id}
          loading={sales.isPending}
          error={sales.isError}
          onRetry={() => sales.refetch()}
          empty={copy.empty}
          rowClassName={(s) => (s.status === "voided" ? "text-ink-muted" : undefined)}
          onRowClick={(s) => setOpenId(s.id)}
        />
        <Pagination meta={sales.data?.meta} onPage={setPage} />
      </div>
      <SaleSheet id={openId} onClose={() => setOpenId(null)} onVoided={(s) => success(copy.void.done(s.invoice))} />
    </>
  );
}

function SaleSheet({ id, onClose, onVoided }: { id: string | null; onClose: () => void; onVoided: (s: Sale) => void }) {
  const sale = useQuery({ queryKey: salesKeys.detail(id ?? ""), queryFn: () => salesApi.get(id!), enabled: id !== null });
  return (
    <Sheet open={id !== null} onClose={onClose} title={sale.data ? copy.detail.title(sale.data.invoice) : "…"} size="lg">
      {sale.isError && <Alert>{sale.error instanceof ApiError ? sale.error.message : "No pudimos cargar la venta."}</Alert>}
      {sale.data && <SaleDetail sale={sale.data} onVoided={onVoided} />}
    </Sheet>
  );
}

function SaleDetail({ sale, onVoided }: { sale: Sale; onVoided: (s: Sale) => void }) {
  const d = copy.detail;
  const row = "flex justify-between gap-4";
  const [message, setMessage] = useState<{ text: string; tone: "success" | "danger" } | null>(null);
  return (
    <div className="space-y-6">
      <p className="text-label text-ink-muted">
        {d.meta(sale.register.name, sale.cashier.name, formatDateTime(sale.createdAt))} · {sale.customer.name}
      </p>
      {sale.offlineAt && <p className="text-caption text-amber-ink">{d.offline(formatDateTime(sale.offlineAt))}</p>}
      {sale.status === "voided" && sale.voidedAt && <Alert>{d.voided(formatDateTime(sale.voidedAt), sale.voidReason ?? "")}</Alert>}

      <section className="space-y-2">
        <h3 className="text-label font-semibold">{d.items}</h3>
        <ul className="divide-y divide-line rounded-lg border border-line">
          {sale.items?.map((i) => (
            <li key={i.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
              <span className="min-w-0">
                <span className="block truncate font-medium">{i.name}</span>
                <span className="font-mono text-caption text-ink-muted">
                  {i.sku} · {d.qty(i.quantity, i.unitsPerItem)} × <Money cents={amountToCents(i.unitPrice)} />
                </span>
              </span>
              <Money cents={amountToCents(i.subtotal)} className="font-mono" />
            </li>
          ))}
        </ul>
      </section>

      <dl className="space-y-1.5 text-label">
        <div className={row}><dt className="text-ink-muted">{d.subtotal}</dt><dd><Money cents={amountToCents(sale.subtotal)} className="font-mono" /></dd></div>
        {amountToCents(sale.discount) > 0 && (
          <div className={row}><dt className="text-ink-muted">{d.discount}</dt><dd className="font-mono">− <Money cents={amountToCents(sale.discount)} /></dd></div>
        )}
        <div className={row}><dt className="text-ink-muted">{d.tax}</dt><dd><Money cents={amountToCents(sale.tax)} className="font-mono" /></dd></div>
        <div className={cn(row, "border-t border-line pt-2 text-body font-semibold")}><dt>{d.total}</dt><dd><Money cents={amountToCents(sale.total)} className="font-mono" /></dd></div>
      </dl>

      <section className="space-y-2">
        <h3 className="text-label font-semibold">{d.payments}</h3>
        <ul className="space-y-1 text-label">
          {sale.payments?.map((p, i) => (
            <li key={i} className={row}>
              <span className="text-ink-muted">
                {d.methods[p.method]} · {p.currency === "USD" ? `US$ · ${d.rate(formatRate(Number(p.exchangeRate)))}` : "C$"}
              </span>
              <Money cents={amountToCents(p.amount)} currency={p.currency} className="font-mono" />
            </li>
          ))}
          {amountToCents(sale.changeNio) > 0 && (
            <li className={row}>
              <span className="text-ink-muted">{d.change}</span>
              <Money cents={amountToCents(sale.changeNio)} className="font-mono" />
            </li>
          )}
        </ul>
      </section>

      <InvoiceActions saleId={sale.id} customerEmail={sale.customer.email} onMessage={(text, t) => setMessage({ text, tone: t })} />
      {message && <Alert tone={message.tone}>{message.text}</Alert>}

      {sale.status === "completed" && <VoidForm sale={sale} onVoided={onVoided} />}
    </div>
  );
}

function VoidForm({ sale, onVoided }: { sale: Sale; onVoided: (s: Sale) => void }) {
  const queryClient = useQueryClient();
  const me = useSession((s) => s.user);
  const canVoid = can(me, "sales:void");
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: (authorization: Authorization | null) => salesApi.void(sale.id, reason.trim(), authorization),
    onSuccess: (voided) => {
      queryClient.setQueryData(salesKeys.detail(sale.id), voided);
      void queryClient.invalidateQueries({ queryKey: salesKeys.all });
      void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
      setOpen(false);
      onVoided(voided);
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : "Ocurrió un error inesperado."),
  });
  const submit = (authorization: Authorization | null) => {
    if (reason.trim().length < 3) return setError(copy.void.reasonRequired);
    setError(null);
    mutation.mutate(authorization);
  };

  if (!open) {
    return (
      <Button variant="ghost" className="text-danger" onClick={() => setOpen(true)}>
        {copy.void.action}
      </Button>
    );
  }
  return (
    <section className="space-y-3 border-t border-line pt-4">
      <Field label={copy.void.reason} hint={copy.void.hint} error={error ?? undefined}>
        <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder={copy.void.reasonPlaceholder} maxLength={300} autoFocus />
      </Field>
      {canVoid ? (
        <Button variant="danger" loading={mutation.isPending} onClick={() => submit(null)}>
          {copy.void.submit}
        </Button>
      ) : (
        <AuthorizationForm busy={mutation.isPending} title={copy.void.action} body={copy.void.needsPin} submitLabel={copy.void.submit} onSubmit={submit} />
      )}
    </section>
  );
}
