"use client";

import { Clock, Key, Percent, Prohibit, Wallet, type Icon } from "@phosphor-icons/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Badge } from "@/components/ui/badge";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { cn } from "@/lib/cn";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { amountToCents } from "@/lib/money";
import { riskApi, riskKeys, type CashierRisk, type Flag, type Period, type RiskEvent } from "../api";
import { riskCopy as copy } from "../copy";

const LEVEL_TONE = { high: "danger", medium: "warning", low: "neutral" } as const;
const EVENT_ICON: Record<RiskEvent["kind"], Icon> = { void: Prohibit, shortage: Wallet, discount: Percent, pin: Key, outside_hours: Clock };
const rate = (part: number, whole: number) => (whole ? part / whole : 0);

export function RiskPage() {
  const [period, setPeriod] = useState<Period>("30");
  const report = useQuery({ queryKey: riskKeys.report(period), queryFn: () => riskApi.report(period), placeholderData: keepPreviousData });
  const data = report.data;
  const flagged = data?.cashiers.filter((c) => c.level !== "low").length ?? 0;
  const has = (c: CashierRisk, flag: Flag) => c.flags.includes(flag);

  const columns: Column<CashierRisk>[] = [
    {
      header: copy.columns.cashier,
      skeleton: 60,
      cell: (c) => (
        <div className="min-w-0 space-y-1">
          <span className="flex items-center gap-2">
            <span className="font-medium">{c.user.name}</span>
            <Badge tone={LEVEL_TONE[c.level]}>{copy.level[c.level]}</Badge>
          </span>
          {c.flags.length > 0 && <span className="block text-caption text-ink-muted">{c.flags.map((f) => copy.flags[f]).join(" · ")}</span>}
        </div>
      ),
    },
    {
      header: copy.columns.sales,
      className: "text-right",
      skeleton: 30,
      cell: (c) => (
        <Cell main={c.sales.toLocaleString("es-NI")}>
          <Money cents={amountToCents(c.total)} />
        </Cell>
      ),
    },
    {
      header: copy.columns.voids,
      className: "text-right",
      skeleton: 30,
      cell: (c) => (
        <Cell main={c.voids} warn={has(c, "voids")}>
          {c.voids > 0 && (
            <>
              {copy.pct(rate(c.voids, c.sales))} · <Money cents={amountToCents(c.voidedAmount)} />
            </>
          )}
        </Cell>
      ),
    },
    {
      header: copy.columns.discounts,
      className: "text-right",
      skeleton: 30,
      cell: (c) => (
        <Cell main={c.nearLimit + c.overLimit} warn={has(c, "discounts")}>
          {c.nearLimit + c.overLimit > 0 && copy.pct(rate(c.nearLimit + c.overLimit, c.sales))}
        </Cell>
      ),
    },
    {
      header: copy.columns.shortages,
      className: "text-right",
      skeleton: 30,
      cell: (c) => (
        <Cell main={c.shifts ? copy.shiftsOf(c.shortShifts, c.shifts) : "—"} warn={has(c, "shortages")}>
          {c.shortShifts > 0 && <Money cents={-amountToCents(c.shortage)} />}
        </Cell>
      ),
    },
    {
      header: copy.columns.other,
      className: "text-right",
      skeleton: 30,
      cell: (c) => <Cell main={`${c.pinFailures} · ${c.outsideHours}`} warn={has(c, "pin") || has(c, "outside_hours")} />,
    },
  ];

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={<SegmentedControl label={copy.period} value={period} options={copy.periods} onChange={setPeriod} className="w-64" />}
      />
      <div className="space-y-6 px-8 pb-10">
        {data && (
          <dl className="grid divide-line rounded-lg border border-line bg-surface sm:grid-cols-2 sm:divide-x xl:grid-cols-4">
            <Fact label={copy.summary.voids} value={copy.pct(data.store.void)} />
            <Fact label={copy.summary.discounts(data.discountLimitPercent)} value={copy.pct(data.store.discount)} />
            <Fact label={copy.summary.shortages} value={copy.pct(data.store.short)} />
            <Fact label={copy.summary.flagged} value={copy.summary.flaggedValue(flagged, data.cashiers.length)} warn={flagged > 0} />
          </dl>
        )}

        <DataTable
          columns={columns}
          rows={data?.cashiers}
          rowKey={(c) => c.user.id}
          loading={report.isPending}
          error={report.isError}
          onRetry={() => report.refetch()}
          empty={copy.empty}
          minWidth={960}
        />

        <div className="grid gap-6 xl:grid-cols-3">
          <Events events={data?.events} />
          <section className="rounded-lg border border-line bg-surface p-5">
            <h2 className="font-semibold">{copy.how.title}</h2>
            <ul className="mt-3 space-y-2 text-label text-ink-muted">
              {copy.how.items.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </section>
        </div>
      </div>
    </>
  );
}

function Cell({ main, warn, children }: { main: ReactNode; warn?: boolean; children?: ReactNode }) {
  return (
    <span className="block text-right">
      <span className={cn("font-mono tabular", warn && "font-semibold text-warning")}>{main}</span>
      {children && <span className="block font-mono text-caption tabular text-ink-muted">{children}</span>}
    </span>
  );
}

function Fact({ label, value, warn }: { label: string; value: ReactNode; warn?: boolean }) {
  return (
    <div className="px-5 py-4">
      <dt className="text-caption text-ink-muted">{label}</dt>
      <dd className={cn("mt-1 text-title font-semibold tabular", warn && "text-warning")}>{value}</dd>
    </div>
  );
}

function eventDetail(e: RiskEvent) {
  const c = copy.events;
  if (e.kind === "discount") return c.authorizedBy(e.detail);
  if (e.kind === "pin") return c.pinOf(e.detail);
  return e.detail;
}

const VISIBLE_EVENTS = 8;

function Events({ events }: { events: RiskEvent[] | undefined }) {
  const c = copy.events;
  const [all, setAll] = useState(false);
  const shown = all ? events : events?.slice(0, VISIBLE_EVENTS);
  return (
    <section className="rounded-lg border border-line bg-surface p-5 xl:col-span-2">
      <h2 className="font-semibold">{c.title}</h2>
      <p className="text-caption text-ink-muted">{c.subtitle}</p>
      {events && events.length === 0 && <p className="mt-4 text-label text-ink-muted">{c.empty}</p>}
      <ul className="mt-3 divide-y divide-line">
        {shown?.map((e, i) => {
          const IconFor = EVENT_ICON[e.kind];
          return (
            <li key={`${e.kind}-${e.at}-${i}`} className="flex items-start gap-3 py-2.5">
              <IconFor size={18} className="mt-0.5 shrink-0 text-ink-subtle" aria-hidden />
              <div className="min-w-0 flex-1">
                <p className="text-label">
                  <span className="font-medium">{e.user}</span> · {c.kinds[e.kind]}
                </p>
                {eventDetail(e) && <p className="truncate text-caption text-ink-muted">{eventDetail(e)}</p>}
              </div>
              <div className="shrink-0 text-right">
                {e.amount && <Money cents={amountToCents(e.amount)} className="block font-mono text-label" />}
                <span className="text-caption text-ink-muted" title={formatDateTime(e.at)}>
                  {formatRelative(e.at)}
                </span>
              </div>
            </li>
          );
        })}
      </ul>
      {events && events.length > VISIBLE_EVENTS && (
        <button type="button" className="mt-2 text-label font-medium text-amber-ink hover:underline" onClick={() => setAll(!all)}>
          {all ? c.fewer : c.more(events.length)}
        </button>
      )}
    </section>
  );
}
