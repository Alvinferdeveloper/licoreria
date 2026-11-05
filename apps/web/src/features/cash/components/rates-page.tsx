"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { NoticeBanner, Pagination, useNotice } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { ApiError } from "@/lib/api/client";
import { formatDate, formatDateTime, todayISO } from "@/lib/format/date";
import { formatRate } from "@/lib/money";
import { cashApi, cashKeys, type Rate } from "../api";
import { ratesCopy as copy } from "../copy";

const ratePattern = /^\d{1,4}(\.\d{1,6})?$/;

export function RatesPage() {
  const me = useSession((s) => s.user);
  const [page, setPage] = useState(1);
  const today = useQuery({ queryKey: cashKeys.rateToday, queryFn: cashApi.rateToday });
  const history = useQuery({ queryKey: cashKeys.rates(page), queryFn: () => cashApi.rates(page), placeholderData: keepPreviousData });
  const { notice, success } = useNotice();

  const columns: Column<Rate>[] = [
    { header: copy.columns.date, skeleton: 30, cell: (r) => <span className="font-mono tabular">{formatDate(r.date)}</span> },
    { header: copy.columns.rate, className: "text-right", skeleton: 30, cell: (r) => <span className="font-mono tabular font-medium">{formatRate(Number(r.rate))}</span> },
    { header: copy.columns.by, skeleton: 40, cell: (r) => <span className="text-label">{r.createdBy.name}</span> },
    { header: copy.columns.at, skeleton: 40, cell: (r) => <span className="font-mono tabular text-label text-ink-muted">{formatDateTime(r.createdAt)}</span> },
  ];

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-6 px-8 pb-10">
        <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,28rem)]">
          <section className="rounded-lg border border-line bg-surface p-5">
            <p className="text-label text-ink-muted">{copy.today}</p>
            {today.data ? (
              <p className="mt-1 text-total font-semibold tracking-headline font-mono tabular">{formatRate(Number(today.data.rate))}</p>
            ) : (
              <p className="mt-2 font-medium text-warning">{today.isPending ? "…" : copy.missing}</p>
            )}
          </section>
          {can(me, "exchange_rates:create") && <RateForm onSaved={(r) => success(copy.form.saved(formatDate(r.date)))} />}
        </div>
        <NoticeBanner notice={notice} />
        <DataTable columns={columns} rows={history.data?.data} rowKey={(r) => r.date} loading={history.isPending} error={history.isError} onRetry={() => history.refetch()} empty={copy.empty} minWidth={560} />
        <Pagination meta={history.data?.meta} onPage={setPage} />
      </div>
    </>
  );
}

function RateForm({ onSaved }: { onSaved: (r: Rate) => void }) {
  const queryClient = useQueryClient();
  const [rate, setRate] = useState("");
  const [date, setDate] = useState(todayISO());
  const [error, setError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: cashApi.setRate,
    onSuccess: (saved) => {
      void queryClient.invalidateQueries({ queryKey: ["rates"] });
      setRate("");
      onSaved(saved);
    },
    onError: (e) => setError(e instanceof ApiError ? (e.fieldMessage("rate") ?? e.fieldMessage("date") ?? e.message) : "Ocurrió un error inesperado."),
  });

  return (
    <form
      className="grid gap-4 rounded-lg border border-line bg-surface p-5 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
      onSubmit={(e) => {
        e.preventDefault();
        const value = rate.trim();
        if (!ratePattern.test(value) || Number(value) <= 0) return setError(copy.form.invalid);
        setError(null);
        mutation.mutate({ rate: value, date });
      }}
      noValidate
    >
      <Field label={copy.form.rate} error={error ?? undefined}>
        <Input value={rate} onChange={(e) => setRate(e.target.value)} inputMode="decimal" placeholder="36.6243" className="font-mono tabular" />
      </Field>
      <Field label={copy.form.date}>
        <Input type="date" value={date} min={todayISO()} onChange={(e) => setDate(e.target.value)} />
      </Field>
      <Button type="submit" variant="primary" loading={mutation.isPending}>
        {copy.form.submit}
      </Button>
      <p className="text-caption text-ink-muted sm:col-span-3">{copy.form.dateHint}</p>
    </form>
  );
}
