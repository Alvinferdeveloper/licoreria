"use client";

import { PencilSimple, Plus } from "@phosphor-icons/react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { NoticeBanner, Pagination, useNotice } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatDateTime } from "@/lib/format/date";
import { amountToCents } from "@/lib/money";
import { cashApi, cashKeys, type Register, type RegisterInput, type Shift } from "../api";
import { cashCopy } from "../copy";

const copy = cashCopy.shifts;
const regCopy = cashCopy.registers;

export function ShiftsPage() {
  const router = useRouter();
  const me = useSession((s) => s.user);
  const [page, setPage] = useState(1);
  const shifts = useQuery({ queryKey: cashKeys.shifts(page), queryFn: () => cashApi.shifts(page), placeholderData: keepPreviousData });

  const columns: Column<Shift>[] = [
    {
      header: copy.columns.register,
      skeleton: 40,
      cell: (s) => (
        <Link href={`/turnos/${s.id}`} className="font-medium hover:underline" onClick={(e) => e.stopPropagation()}>
          {s.register.name}
        </Link>
      ),
    },
    { header: copy.columns.user, skeleton: 50, cell: (s) => <span className="text-label">{s.user.name}</span> },
    { header: copy.columns.opened, skeleton: 40, cell: (s) => <span className="font-mono tabular text-label">{formatDateTime(s.openedAt)}</span> },
    { header: copy.columns.closed, skeleton: 40, cell: (s) => <span className="font-mono tabular text-label">{s.closedAt ? formatDateTime(s.closedAt) : "—"}</span> },
    { header: copy.columns.difference, className: "text-right", skeleton: 40, cell: (s) => <DifferenceCell shift={s} /> },
    { header: copy.columns.status, skeleton: 30, cell: (s) => (s.status === "open" ? <Badge tone="amber">{copy.status.open}</Badge> : <Badge>{copy.status.closed}</Badge>) },
  ];

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-8 px-8 pb-10">
        {can(me, "settings:update") && <RegistersSection />}
        <section className="space-y-3">
          <h2 className="font-semibold">{copy.history}</h2>
          <DataTable
            columns={columns}
            rows={shifts.data?.data}
            rowKey={(s) => s.id}
            loading={shifts.isPending}
            error={shifts.isError}
            onRetry={() => shifts.refetch()}
            empty={copy.empty}
            onRowClick={(s) => router.push(`/turnos/${s.id}`)}
          />
          <Pagination meta={shifts.data?.meta} onPage={setPage} />
        </section>
      </div>
    </>
  );
}

function DifferenceCell({ shift }: { shift: Shift }) {
  if (shift.status === "open" || shift.differenceNio === null || shift.differenceUsd === null) return <span className="text-ink-subtle">—</span>;
  const nio = amountToCents(shift.differenceNio);
  const usd = amountToCents(shift.differenceUsd);
  if (nio === 0 && usd === 0) return <Badge tone="success">{cashCopy.closing.balanced}</Badge>;
  return (
    <span className="flex flex-col items-end font-mono text-label text-danger">
      {nio !== 0 && <Money cents={nio} />}
      {usd !== 0 && <Money cents={usd} currency="USD" />}
    </span>
  );
}

function RegistersSection() {
  const queryClient = useQueryClient();
  const registers = useQuery({ queryKey: cashKeys.registers, queryFn: cashApi.registers });
  const [editing, setEditing] = useState<{ open: boolean; register: Register | null }>({ open: false, register: null });
  const { notice, success } = useNotice();

  const columns: Column<Register>[] = [
    { header: regCopy.name, skeleton: 40, cell: (r) => <span className="font-medium">{r.name}</span> },
    {
      header: regCopy.series,
      skeleton: 30,
      cell: (r) => (
        <span className="text-label">
          <span className="font-mono">{r.invoiceSeries}</span> <span className="text-ink-muted">· {regCopy.next(r.nextInvoiceNumber)}</span>
        </span>
      ),
    },
    {
      header: copy.columns.status,
      skeleton: 40,
      cell: (r) =>
        !r.active ? <Badge>{regCopy.inactive}</Badge> : r.openShift ? <Badge tone="amber">{regCopy.inUse(r.openShift.user.name)}</Badge> : <Badge tone="success">{regCopy.free}</Badge>,
    },
    {
      header: <span className="sr-only">Acciones</span>,
      className: "w-px px-2",
      skeleton: 0,
      cell: (r) => (
        <RowActions>
          <Button variant="ghost" size="icon-sm" aria-label={`${regCopy.edit} ${r.name}`} title={regCopy.edit} onClick={() => setEditing({ open: true, register: r })}>
            <PencilSimple size={16} />
          </Button>
        </RowActions>
      ),
    },
  ];

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-semibold">{regCopy.title}</h2>
        <Button size="sm" onClick={() => setEditing({ open: true, register: null })}>
          <Plus size={14} weight="bold" aria-hidden />
          {regCopy.create}
        </Button>
      </div>
      <NoticeBanner notice={notice} />
      <DataTable columns={columns} rows={registers.data} rowKey={(r) => r.id} loading={registers.isPending} error={registers.isError} onRetry={() => registers.refetch()} empty={copy.empty} minWidth={640} />
      <Sheet open={editing.open} onClose={() => setEditing({ open: false, register: null })} title={editing.register ? regCopy.edit : regCopy.create}>
        {editing.open && (
          <RegisterForm
            register={editing.register}
            onCancel={() => setEditing({ open: false, register: null })}
            onSaved={(list) => {
              queryClient.setQueryData(cashKeys.registers, list);
              setEditing({ open: false, register: null });
              success(regCopy.saved);
            }}
          />
        )}
      </Sheet>
    </section>
  );
}

function RegisterForm({ register, onCancel, onSaved }: { register: Register | null; onCancel: () => void; onSaved: (list: Register[]) => void }) {
  const [name, setName] = useState(register?.name ?? "");
  const [series, setSeries] = useState(register?.invoiceSeries ?? "");
  const [active, setActive] = useState(register?.active ?? true);
  const [error, setError] = useState<ApiError | null>(null);
  const mutation = useMutation({
    mutationFn: (input: RegisterInput) => (register ? cashApi.updateRegister(register.id, input) : cashApi.createRegister(input)),
    onSuccess: (r) => onSaved(r.data),
    onError: (e) => setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Ocurrió un error inesperado.")),
  });
  const formError = error && !error.fields.length ? error.message : null;

  return (
    <form
      className="space-y-5"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate({ name: name.trim(), invoiceSeries: series.trim().toUpperCase(), active });
      }}
    >
      {formError && <Alert>{formError}</Alert>}
      <Field label={regCopy.name} error={error?.fieldMessage("name")}>
        <Input value={name} onChange={(e) => setName(e.target.value)} autoFocus maxLength={60} placeholder={regCopy.namePlaceholder} />
      </Field>
      <Field label={regCopy.series} hint={regCopy.seriesHint} error={error?.fieldMessage("invoiceSeries")}>
        <Input
          value={series}
          onChange={(e) => setSeries(e.target.value.toUpperCase())}
          className={cn("font-mono uppercase")}
          maxLength={8}
          placeholder={regCopy.seriesPlaceholder}
        />
      </Field>
      <Checkbox checked={active} onChange={(e) => setActive(e.target.checked)} label={regCopy.active} />
      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>{regCopy.cancel}</Button>
        <Button type="submit" variant="primary" loading={mutation.isPending}>
          {regCopy.save}
        </Button>
      </div>
    </form>
  );
}
