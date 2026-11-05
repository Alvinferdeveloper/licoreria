"use client";

import { ArrowsLeftRight, LockSimple } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { NoticeBanner, useNotice } from "@/components/data/toolbar";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { amountToCents, parseAmount, toAmountString } from "@/lib/money";
import { cashApi, cashKeys, type Shift } from "../api";
import { cashCopy as copy } from "../copy";
import { CloseShiftSheet } from "./close-shift-sheet";
import { MovementSheet } from "./movement-sheet";
import { RateBanner } from "./rate-banner";
import { ShiftBreakdown } from "./shift-breakdown";

export function CashPage() {
  const current = useQuery({ queryKey: cashKeys.current, queryFn: cashApi.current });
  const [closed, setClosed] = useState<Shift | null>(null);

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-5 px-8 pb-10">
        <RateBanner />
        {closed ? (
          <CloseResult shift={closed} onAgain={() => setClosed(null)} />
        ) : current.isPending ? (
          <p className="text-ink-muted">Cargando…</p>
        ) : current.isError ? (
          <Alert>{current.error instanceof ApiError ? current.error.message : "No pudimos cargar tu caja."}</Alert>
        ) : current.data ? (
          <ShiftPanel shift={current.data} onClosed={setClosed} />
        ) : (
          <OpenShiftForm />
        )}
      </div>
    </>
  );
}

function OpenShiftForm() {
  const queryClient = useQueryClient();
  const registers = useQuery({ queryKey: cashKeys.registers, queryFn: cashApi.registers });
  const free = (registers.data ?? []).filter((r) => r.active && !r.openShift);
  const [registerId, setRegisterId] = useState("");
  const [nio, setNio] = useState("0");
  const [usd, setUsd] = useState("0");
  const [errors, setErrors] = useState<Partial<Record<"register" | "nio" | "usd" | "form", string>>>({});
  const selected = registerId || (free.length === 1 ? free[0].id : "");

  const open = useMutation({
    mutationFn: cashApi.open,
    onSuccess: (shift) => {
      queryClient.setQueryData(cashKeys.current, shift);
      void queryClient.invalidateQueries({ queryKey: cashKeys.registers });
    },
    onError: (e) => setErrors({ form: e instanceof ApiError ? e.message : "Ocurrió un error inesperado." }),
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const nioCents = parseAmount(nio);
    const usdCents = parseAmount(usd);
    const next: typeof errors = {};
    if (!selected) next.register = copy.validation.register;
    if (nioCents === null) next.nio = copy.validation.amount;
    if (usdCents === null) next.usd = copy.validation.amount;
    setErrors(next);
    if (Object.keys(next).length || nioCents === null || usdCents === null) return;
    open.mutate({ registerId: selected, openingNio: toAmountString(nioCents), openingUsd: toAmountString(usdCents) });
  };

  return (
    <form onSubmit={submit} className="max-w-xl space-y-5 rounded-lg border border-line bg-surface p-6" noValidate>
      <h2 className="text-title font-semibold">{copy.open.title}</h2>
      {errors.form && <Alert>{errors.form}</Alert>}
      {registers.data && free.length === 0 ? (
        <Alert tone="info">{copy.open.noRegisters}</Alert>
      ) : (
        <Field label={copy.open.register} error={errors.register}>
          <select value={selected} onChange={(e) => setRegisterId(e.target.value)} className={inputClassName}>
            <option value="">{copy.open.selectRegister}</option>
            {(registers.data ?? [])
              .filter((r) => r.active)
              .map((r) => (
                <option key={r.id} value={r.id} disabled={!!r.openShift}>
                  {r.name} ({r.invoiceSeries}){r.openShift ? ` · ${copy.open.busy(r.openShift.user.name)}` : ""}
                </option>
              ))}
          </select>
        </Field>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={copy.open.openingNio} error={errors.nio}>
          <Input value={nio} onChange={(e) => setNio(e.target.value)} inputMode="decimal" className="font-mono tabular" onFocus={(e) => e.target.select()} />
        </Field>
        <Field label={copy.open.openingUsd} error={errors.usd}>
          <Input value={usd} onChange={(e) => setUsd(e.target.value)} inputMode="decimal" className="font-mono tabular" onFocus={(e) => e.target.select()} />
        </Field>
      </div>
      <Button type="submit" variant="primary" size="lg" loading={open.isPending} disabled={free.length === 0}>
        {copy.open.submit}
      </Button>
    </form>
  );
}

function ShiftPanel({ shift, onClosed }: { shift: Shift; onClosed: (s: Shift) => void }) {
  const queryClient = useQueryClient();
  const [sheet, setSheet] = useState<"movement" | "close" | null>(null);
  const { notice, success } = useNotice();
  const update = (next: Shift) => queryClient.setQueryData(cashKeys.current, next);

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-label text-ink-muted" title={formatDateTime(shift.openedAt)}>
          {copy.shift.openSince(shift.register.name, formatRelative(shift.openedAt))}
        </p>
        <div className="flex gap-2">
          <Button onClick={() => setSheet("movement")}>
            <ArrowsLeftRight size={16} weight="bold" aria-hidden />
            {copy.shift.addMovement}
          </Button>
          <Button variant="primary" onClick={() => setSheet("close")}>
            <LockSimple size={16} weight="bold" aria-hidden />
            {copy.shift.close}
          </Button>
        </div>
      </div>
      <NoticeBanner notice={notice} />
      {shift.summary && <ShiftBreakdown shift={shift} summary={shift.summary} />}
      <MovementList shift={shift} />

      <MovementSheet
        open={sheet === "movement"}
        shiftId={shift.id}
        onClose={() => setSheet(null)}
        onDone={(next) => {
          update(next);
          setSheet(null);
          success(copy.movement.done);
        }}
      />
      {shift.summary && (
        <CloseShiftSheet
          open={sheet === "close"}
          shift={shift}
          summary={shift.summary}
          onClose={() => setSheet(null)}
          onDone={(closedShift) => {
            setSheet(null);
            queryClient.setQueryData(cashKeys.current, null);
            void queryClient.invalidateQueries({ queryKey: cashKeys.registers });
            onClosed(closedShift);
          }}
        />
      )}
    </div>
  );
}

export function MovementList({ shift }: { shift: Shift }) {
  const items = shift.movements ?? [];
  return (
    <section className="space-y-2">
      <h2 className="font-semibold">{copy.shift.movements}</h2>
      {items.length === 0 ? (
        <p className="text-label text-ink-muted">{copy.shift.noMovements}</p>
      ) : (
        <ul className="divide-y divide-line rounded-lg border border-line bg-surface">
          {items.map((m) => (
            <li key={m.id} className="flex items-center justify-between gap-4 px-4 py-3">
              <div className="min-w-0">
                <p className="truncate font-medium">{m.reason}</p>
                <p className="text-caption text-ink-muted">
                  {copy.movement.typeLabel[m.type]} · {m.user.name} · {formatDateTime(m.createdAt)}
                </p>
              </div>
              <span className={cn("font-mono tabular font-medium", m.type === "expense" && "text-danger")}>
                {m.type === "expense" ? "− " : "+ "}
                <Money cents={amountToCents(m.amount)} currency={m.currency} />
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function CloseResult({ shift, onAgain }: { shift: Shift; onAgain?: () => void }) {
  const balanced = shift.differenceNio === "0.00" && shift.differenceUsd === "0.00";
  const rows = [
    { label: copy.shift.nio, currency: "NIO" as const, expected: shift.expectedNio, counted: shift.countedNio, diff: shift.differenceNio },
    { label: copy.shift.usd, currency: "USD" as const, expected: shift.expectedUsd, counted: shift.countedUsd, diff: shift.differenceUsd },
  ];
  return (
    <div className="max-w-2xl space-y-4">
      <Alert tone={balanced ? "success" : "danger"}>{balanced ? copy.result.balanced : copy.result.unbalanced}</Alert>
      <table className="w-full overflow-hidden rounded-lg border border-line bg-surface text-left">
        <thead className="bg-chrome text-caption text-ink-muted">
          <tr>
            <th className="px-4 py-2.5 font-medium" />
            <th className="px-4 py-2.5 text-right font-medium">{copy.result.expected}</th>
            <th className="px-4 py-2.5 text-right font-medium">{copy.result.counted}</th>
            <th className="px-4 py-2.5 text-right font-medium">{copy.result.difference}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {rows.map((r) => (
            <tr key={r.currency}>
              <th className="px-4 py-3 font-medium">{r.label}</th>
              <td className="px-4 py-3 text-right"><Money cents={amountToCents(r.expected ?? "0")} currency={r.currency} className="font-mono" /></td>
              <td className="px-4 py-3 text-right"><Money cents={amountToCents(r.counted ?? "0")} currency={r.currency} className="font-mono" /></td>
              <td className={cn("px-4 py-3 text-right font-medium", r.diff !== "0.00" && "text-danger")}>
                <Money cents={amountToCents(r.diff ?? "0")} currency={r.currency} className="font-mono" />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {shift.notes && <p className="text-label text-ink-muted">{copy.shifts.notes(shift.notes)}</p>}
      {onAgain && <Button onClick={onAgain}>{copy.result.again}</Button>}
    </div>
  );
}
