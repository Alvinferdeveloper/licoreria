"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { cn } from "@/lib/cn";
import { amountToCents } from "@/lib/money";
import { dashboardApi, dashboardKeys, type Dashboard } from "../api";
import { dashboardCopy as copy } from "../copy";
import { CategoryChart, DailyChart, MethodsChart, PeakHours, TopProductsChart } from "./charts";

export function DashboardPage() {
  const dashboard = useQuery({ queryKey: dashboardKeys.all, queryFn: dashboardApi.get, refetchInterval: 60_000 });
  const d = dashboard.data;

  return (
    <>
      <PageHeader title={copy.title} description={d?.scope === "self" ? copy.descriptionSelf : copy.description} />
      <div className="space-y-6 px-8 pb-10">
        {dashboard.isError && <Alert>No pudimos cargar el panel.</Alert>}
        {!d ? (
          <div className="h-96 animate-pulse rounded-lg bg-sunken" aria-hidden />
        ) : (
          <>
            <KpiStrip kpis={d.kpis} />
            <div className="grid gap-4 xl:grid-cols-2">
              <DailyChart data={d.daily} />
              <TopProductsChart data={d.top} />
              <CategoryChart data={d.categories} />
              <MethodsChart data={d.methods} />
              <PeakHours data={d.heatmap} />
            </div>
          </>
        )}
      </div>
    </>
  );
}

function KpiStrip({ kpis }: { kpis: Dashboard["kpis"] }) {
  const today = amountToCents(kpis.salesToday);
  const yesterday = amountToCents(kpis.salesYesterday);
  const change = yesterday > 0 ? ((today - yesterday) / yesterday) * 100 : null;
  const changeText = change === null ? copy.kpis.noYesterday : copy.kpis.vsYesterday(`${change >= 0 ? "+" : "−"}${Math.abs(Math.round(change))} %`);

  return (
    <dl className="grid divide-line rounded-lg border border-line bg-surface sm:grid-cols-2 sm:divide-x lg:grid-cols-4">
      <Kpi label={copy.kpis.salesToday} value={<Money cents={today} />} hint={changeText} hintTone={change === null ? undefined : change >= 0 ? "up" : "down"} />
      <Kpi label={copy.kpis.averageTicket} value={<Money cents={amountToCents(kpis.averageTicket)} />} hint={copy.kpis.salesCount(kpis.salesCount)} />
      {kpis.profitToday !== undefined && <Kpi label={copy.kpis.profitToday} value={<Money cents={amountToCents(kpis.profitToday)} />} hint={copy.kpis.profitHint} />}
      <Kpi
        label={copy.kpis.lowStock}
        value={<span className="tabular">{kpis.lowStock}</span>}
        hint={
          <Link href="/inventario" className="text-amber-ink hover:underline">
            {copy.kpis.lowStockHint}
          </Link>
        }
      />
    </dl>
  );
}

function Kpi({ label, value, hint, hintTone }: { label: string; value: React.ReactNode; hint: React.ReactNode; hintTone?: "up" | "down" }) {
  return (
    <div className="px-5 py-4">
      <dt className="text-label text-ink-muted">{label}</dt>
      <dd className="mt-1 font-mono text-headline font-semibold tracking-headline">{value}</dd>
      <dd className={cn("mt-0.5 text-caption text-ink-muted", hintTone === "up" && "text-success", hintTone === "down" && "text-danger")}>{hint}</dd>
    </div>
  );
}
