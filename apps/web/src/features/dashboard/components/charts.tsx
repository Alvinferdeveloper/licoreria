"use client";

import { useState } from "react";
import { Bar, BarChart, CartesianGrid, Cell, Line, LineChart, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { formatDate } from "@/lib/format/date";
import { amountToCents, formatAmount } from "@/lib/money";
import type { Dashboard } from "../api";
import { dashboardCopy as copy } from "../copy";
import { ChartCard, ChartTooltip, DataList, LegendItem, SwatchItem } from "./chart-card";

const CHART = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)", "var(--chart-6)"];
const axis = { stroke: "var(--ink-subtle)", fontSize: 11, tickLine: false, axisLine: false } as const;
const grid = { stroke: "var(--line)", strokeDasharray: "0", vertical: false } as const;
const toNumber = (amount: string) => amountToCents(amount) / 100;
const money = (amount: string) => `C$ ${formatAmount(amountToCents(amount))}`;
const shortDay = (iso: string) => formatDate(iso).slice(0, 5);
const compact = (v: number) => (v >= 1000 ? `${Math.round(v / 100) / 10}k` : String(Math.round(v)));

export function DailyChart({ data }: { data: Dashboard["daily"] }) {
  const points = data.map((d) => ({ day: shortDay(d.date), current: toNumber(d.total), previous: toNumber(d.previous) }));
  return (
    <ChartCard
      title={copy.daily.title}
      subtitle={copy.daily.subtitle}
      className="xl:col-span-2"
      legend={
        <div className="flex gap-4">
          <LegendItem color="var(--chart-1)" label={copy.daily.current} />
          <LegendItem color="var(--ink-subtle)" label={copy.daily.previous} dashed />
        </div>
      }
      table={<DataList head={[copy.date, copy.daily.current, copy.daily.previous]} rows={data.map((d) => [formatDate(d.date), money(d.total), money(d.previous)])} />}
    >
      <ResponsiveContainer width="100%" height={240}>
        <LineChart data={points} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
          <CartesianGrid {...grid} />
          <XAxis dataKey="day" {...axis} interval="preserveStartEnd" minTickGap={24} />
          <YAxis {...axis} width={40} tickFormatter={compact} />
          <Tooltip content={<ChartTooltip />} cursor={{ stroke: "var(--line-strong)" }} />
          <Line type="monotone" dataKey="previous" name={copy.daily.previous} stroke="var(--ink-subtle)" strokeWidth={2} strokeDasharray="4 4" dot={false} activeDot={{ r: 4 }} />
          <Line type="monotone" dataKey="current" name={copy.daily.current} stroke="var(--chart-1)" strokeWidth={2} dot={false} activeDot={{ r: 5, stroke: "var(--surface)", strokeWidth: 2 }} />
        </LineChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function TopProductsChart({ data }: { data: Dashboard["top"] }) {
  const [by, setBy] = useState<"revenue" | "units">("revenue");
  const points = [...data]
    .map((d) => ({ name: d.name, value: by === "units" ? d.units : toNumber(d.revenue) }))
    .sort((a, b) => b.value - a.value)
    .slice(0, 10);
  const format = (v: number) => (by === "units" ? `${v} u` : `C$ ${formatAmount(Math.round(v * 100))}`);
  return (
    <ChartCard
      title={copy.top.title}
      subtitle={copy.top.subtitle}
      controls={<SegmentedControl label={copy.top.by} value={by} options={copy.top.options} onChange={setBy} className="w-48" />}
      table={<DataList head={["Producto", copy.top.units, copy.top.revenue]} rows={data.slice(0, 10).map((d) => [d.name, d.units, money(d.revenue)])} />}
    >
      <ResponsiveContainer width="100%" height={Math.max(160, points.length * 28)}>
        <BarChart data={points} layout="vertical" margin={{ top: 0, right: 12, bottom: 0, left: 0 }} barCategoryGap={6}>
          <XAxis type="number" hide />
          <YAxis
            type="category"
            dataKey="name"
            {...axis}
            width={190}
            tick={{ fill: "var(--ink-muted)", fontSize: 11 }}
            tickFormatter={(v: string) => (v.length > 26 ? `${v.slice(0, 25)}…` : v)}
          />
          <Tooltip content={<ChartTooltip format={format} />} cursor={{ fill: "var(--sunken)" }} />
          <Bar dataKey="value" name={by === "units" ? copy.top.units : copy.top.revenue} fill="var(--chart-1)" radius={[0, 4, 4, 0]} maxBarSize={18} />
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function CategoryChart({ data }: { data: Dashboard["categories"] }) {
  // Five largest categories keep their color; the rest fold into "Otras".
  const top = data.slice(0, 5).map((d) => ({ name: d.name, value: toNumber(d.total) }));
  const rest = data.slice(5).reduce((sum, d) => sum + toNumber(d.total), 0);
  const slices = rest > 0 ? [...top, { name: copy.categories.other, value: rest }] : top;
  const total = slices.reduce((s, d) => s + d.value, 0);
  const pct = (v: number) => (total > 0 ? `${Math.round((v / total) * 1000) / 10} %` : "0 %");
  return (
    <ChartCard
      title={copy.categories.title}
      subtitle={copy.categories.subtitle}
      table={<DataList head={["Categoría", copy.total, copy.categories.share]} rows={slices.map((s) => [s.name, `C$ ${formatAmount(Math.round(s.value * 100))}`, pct(s.value)])} />}
    >
      <div className="flex flex-wrap items-center gap-6">
        <div className="size-44 shrink-0">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie data={slices} dataKey="value" nameKey="name" innerRadius="62%" outerRadius="100%" stroke="var(--surface)" strokeWidth={2} isAnimationActive={false}>
                {slices.map((s, i) => (
                  <Cell key={s.name} fill={CHART[i]} />
                ))}
              </Pie>
              <Tooltip content={<ChartTooltip />} />
            </PieChart>
          </ResponsiveContainer>
        </div>
        <ul className="min-w-40 flex-1 space-y-1.5">
          {slices.map((s, i) => (
            <li key={s.name} className="flex items-center justify-between gap-3">
              <SwatchItem color={CHART[i]} label={s.name} />
              <span className="font-mono text-caption tabular text-ink-muted">{pct(s.value)}</span>
            </li>
          ))}
        </ul>
      </div>
    </ChartCard>
  );
}

const METHOD_SERIES = [
  { key: "cashNio", color: "var(--chart-1)" },
  { key: "cashUsd", color: "var(--chart-2)" },
  { key: "card", color: "var(--chart-3)" },
  { key: "transfer", color: "var(--chart-4)" },
] as const;

export function MethodsChart({ data }: { data: Dashboard["methods"] }) {
  const points = data.map((d) => ({ day: shortDay(d.date), cashNio: toNumber(d.cashNio), cashUsd: toNumber(d.cashUsd), card: toNumber(d.card), transfer: toNumber(d.transfer) }));
  return (
    <ChartCard
      title={copy.methods.title}
      subtitle={copy.methods.subtitle}
      legend={
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {METHOD_SERIES.map((s) => (
            <SwatchItem key={s.key} color={s.color} label={copy.methods.series[s.key]} />
          ))}
        </div>
      }
      table={
        <DataList
          head={[copy.date, ...METHOD_SERIES.map((s) => copy.methods.series[s.key])]}
          rows={data.map((d) => [formatDate(d.date), money(d.cashNio), money(d.cashUsd), money(d.card), money(d.transfer)])}
        />
      }
    >
      <ResponsiveContainer width="100%" height={240}>
        <BarChart data={points} margin={{ top: 4, right: 8, bottom: 0, left: 0 }} barCategoryGap="22%">
          <CartesianGrid {...grid} />
          <XAxis dataKey="day" {...axis} interval="preserveStartEnd" minTickGap={16} />
          <YAxis {...axis} width={40} tickFormatter={compact} />
          <Tooltip content={<ChartTooltip />} cursor={{ fill: "var(--sunken)" }} />
          {METHOD_SERIES.map((s, i) => (
            <Bar
              key={s.key}
              dataKey={s.key}
              name={copy.methods.series[s.key]}
              stackId="m"
              fill={s.color}
              stroke="var(--surface)"
              strokeWidth={1}
              radius={i === METHOD_SERIES.length - 1 ? [4, 4, 0, 0] : 0}
              maxBarSize={28}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

const STEPS = [8, 22, 40, 62, 85];

export function PeakHours({ data }: { data: Dashboard["heatmap"] }) {
  const hours = Array.from({ length: 16 }, (_, i) => i + 8);
  const byCell = new Map(data.map((d) => [`${d.weekday}-${d.hour}`, d]));
  const max = Math.max(1, ...data.map((d) => d.sales));
  const shade = (sales: number) => (sales === 0 ? "var(--sunken)" : `color-mix(in oklch, var(--chart-1) ${STEPS[Math.min(4, Math.floor((sales / max) * 4.999))]}%, var(--surface))`);

  return (
    <ChartCard
      title={copy.heatmap.title}
      subtitle={copy.heatmap.subtitle}
      table={
        <DataList
          head={["Día", "Hora", "Ventas", copy.total]}
          rows={data.map((d) => [copy.heatmap.days[d.weekday - 1], `${d.hour}:00`, d.sales, money(d.total)])}
        />
      }
    >
      <div className="overflow-x-auto">
        <div className="inline-grid min-w-full gap-0.5" style={{ gridTemplateColumns: `2.5rem repeat(${hours.length}, minmax(1.25rem, 1fr))` }}>
          <span />
          {hours.map((h) => (
            <span key={h} className="text-center font-mono text-[10px] text-ink-subtle">
              {h}
            </span>
          ))}
          {copy.heatmap.days.map((day, i) => (
            <div key={day} className="contents">
              <span className="pr-2 text-right text-caption text-ink-muted">{day}</span>
              {hours.map((h) => {
                const cell = byCell.get(`${i + 1}-${h}`);
                const sales = cell?.sales ?? 0;
                return (
                  <span
                    key={h}
                    title={copy.heatmap.cell(day, h, sales)}
                    aria-label={copy.heatmap.cell(day, h, sales)}
                    className="h-6 rounded-xs transition-[outline] hover:outline-2 hover:outline-offset-1 hover:outline-ink"
                    style={{ backgroundColor: shade(sales) }}
                  />
                );
              })}
            </div>
          ))}
        </div>
        <div className="mt-3 flex items-center justify-end gap-1.5 text-caption text-ink-muted">
          {copy.heatmap.less}
          {[0, ...STEPS].map((s) => (
            <span key={s} className="size-3 rounded-xs" style={{ backgroundColor: s === 0 ? "var(--sunken)" : `color-mix(in oklch, var(--chart-1) ${s}%, var(--surface))` }} />
          ))}
          {copy.heatmap.more}
        </div>
      </div>
    </ChartCard>
  );
}
