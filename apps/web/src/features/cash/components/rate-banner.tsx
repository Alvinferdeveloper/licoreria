"use client";

import { CurrencyDollar, Warning } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { formatRate } from "@/lib/money";
import { cashApi, cashKeys } from "../api";
import { cashCopy } from "../copy";

const copy = cashCopy.rate;

/** Today's exchange rate, or a warning that sales are blocked until it is set. */
export function RateBanner() {
  const me = useSession((s) => s.user);
  const rate = useQuery({ queryKey: cashKeys.rateToday, queryFn: cashApi.rateToday });
  if (rate.isPending || rate.isError) return null;

  if (!rate.data) {
    return (
      <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-md bg-warning-tint px-4 py-3 text-label">
        <span className="flex items-center gap-2 text-ink">
          <Warning size={18} weight="bold" className="text-warning" aria-hidden />
          {copy.missing}
        </span>
        {can(me, "exchange_rates:create") && (
          <Link href="/tipo-cambio" className="font-medium text-amber-ink hover:underline">
            {copy.missingAdmin}
          </Link>
        )}
      </div>
    );
  }
  return (
    <div className="flex items-center gap-2 text-label text-ink-muted">
      <CurrencyDollar size={16} aria-hidden />
      <span>{copy.label}:</span>
      <span className="font-mono tabular font-medium text-ink">{copy.value(formatRate(Number(rate.data.rate)))}</span>
    </div>
  );
}
