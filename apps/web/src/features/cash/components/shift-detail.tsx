"use client";

import { ArrowLeft } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { ApiError } from "@/lib/api/client";
import { formatDateTime } from "@/lib/format/date";
import { cashApi, cashKeys } from "../api";
import { cashCopy } from "../copy";
import { CloseResult, MovementList } from "./cash-page";
import { ShiftBreakdown } from "./shift-breakdown";

const copy = cashCopy.shifts;

export function ShiftDetail({ id }: { id: string }) {
  const shift = useQuery({ queryKey: cashKeys.shift(id), queryFn: () => cashApi.shift(id) });

  return (
    <div className="space-y-5 px-8 pt-6 pb-10">
      <Link href="/turnos" className="inline-flex items-center gap-1.5 text-label text-ink-muted transition-colors hover:text-ink">
        <ArrowLeft size={14} weight="bold" aria-hidden />
        {copy.back}
      </Link>
      {shift.isPending ? (
        <p className="text-ink-muted">Cargando…</p>
      ) : shift.isError ? (
        <Alert>{shift.error instanceof ApiError ? shift.error.message : "No pudimos cargar el turno."}</Alert>
      ) : (
        <>
          <header className="space-y-1">
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-headline font-semibold tracking-headline">{copy.detailTitle(shift.data.register.name, shift.data.user.name)}</h1>
              {shift.data.status === "open" ? <Badge tone="amber">{copy.status.open}</Badge> : <Badge>{copy.status.closed}</Badge>}
            </div>
            <p className="text-label text-ink-muted">
              {formatDateTime(shift.data.openedAt)}
              {shift.data.closedAt && ` → ${formatDateTime(shift.data.closedAt)}`}
            </p>
          </header>
          {shift.data.status === "closed" && <CloseResult shift={shift.data} />}
          {shift.data.summary && <ShiftBreakdown shift={shift.data} summary={shift.data.summary} />}
          <MovementList shift={shift.data} />
        </>
      )}
    </div>
  );
}
