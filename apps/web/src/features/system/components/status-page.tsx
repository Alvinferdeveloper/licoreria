"use client";

import { ArrowClockwise, Check, Copy } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ChartCard, ChartTooltip, DataList, SwatchItem } from "@/features/dashboard/components/chart-card";
import { cn } from "@/lib/cn";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { formatHour } from "@/features/backups/copy";
import { systemApi, systemKeys, type CheckStatus, type SystemStatus } from "../api";
import { systemCopy as copy } from "../copy";

const REFRESH_MS = 15_000;
const TONE = { ok: "success", warn: "warning", fail: "danger", unknown: "neutral" } as const;
const DOT = { ok: "bg-success", warn: "bg-warning", fail: "bg-danger", unknown: "bg-ink-subtle" } satisfies Record<CheckStatus, string>;
const hourLabel = new Intl.DateTimeFormat("es-NI", { hour: "numeric", timeZone: "America/Managua" });

function formatBytes(bytes: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function formatUptime(startedAt: string, now: number) {
  const minutes = Math.max(0, Math.floor((now - new Date(startedAt).getTime()) / 60_000));
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  if (days > 0) return `${days} d ${hours} h`;
  if (hours > 0) return `${hours} h ${minutes % 60} min`;
  return `${minutes} min`;
}

const ms = (value: number) => (value < 1 ? "<1 ms" : `${Math.round(value)} ms`);

export function StatusPage() {
  const status = useQuery({ queryKey: systemKeys.status, queryFn: systemApi.status, refetchInterval: REFRESH_MS });
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 5_000);
    return () => clearInterval(timer);
  }, []);

  const data = status.data;
  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          <>
            {data && <span className="text-caption text-ink-muted">{copy.updated(formatRelative(data.checkedAt, now))}</span>}
            <Button variant="secondary" loading={status.isFetching} onClick={() => status.refetch()}>
              <ArrowClockwise size={16} aria-hidden />
              {copy.refresh}
            </Button>
          </>
        }
      />
      <div className="space-y-6 px-8 pb-10">
        {status.isError && <Alert tone="danger">{copy.error}</Alert>}
        {data && <Report data={data} now={now} />}
        {!data && !status.isError && <div className="h-64 animate-pulse rounded-lg bg-sunken" />}
      </div>
    </>
  );
}

function Report({ data, now }: { data: SystemStatus; now: number }) {
  const checks: [string, CheckStatus][] = [
    [copy.database.title, data.database.status],
    [copy.backups.title, data.backups.status],
    [copy.disk.title, data.disk.status],
    [copy.email.title, data.email.status],
    [copy.storage.title, data.storage.status],
    [copy.rate.title, data.exchangeRate.status],
    [copy.traffic.title, data.traffic.status],
  ];
  const flagged = checks.filter(([, s]) => s === "warn" || s === "fail").map(([name]) => name);

  return (
    <>
      <section aria-live="polite" className="flex items-center gap-4 rounded-lg border border-line bg-surface px-5 py-4">
        <span className={cn("size-3 shrink-0 rounded-full", DOT[data.status])} aria-hidden />
        <div>
          <p className="font-semibold">{copy.overall[data.status]}</p>
          <p className="text-label text-ink-muted">{copy.overallHint(flagged)}</p>
        </div>
      </section>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <DatabaseCard data={data.database} />
        <BackupsCard data={data.backups} now={now} />
        <RateCard data={data.exchangeRate} />
        <EmailCard data={data.email} now={now} />
        <DiskCard data={data.disk} />
        <StorageCard data={data.storage} />
        <ApiCard data={data.api} now={now} />
        <ActivityCard data={data.activity} />
      </div>

      <TrafficChart hours={data.traffic.hours} />
      <Failures items={data.traffic.failed} now={now} />
    </>
  );
}

function Card({ title, status, value, note, children }: { title: string; status: CheckStatus; value: ReactNode; note?: ReactNode; children?: ReactNode }) {
  return (
    <section className="flex flex-col rounded-lg border border-line bg-surface p-5">
      <header className="flex items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-label font-medium">
          <span className={cn("size-2 rounded-full", DOT[status])} aria-hidden />
          {title}
        </h2>
        <Badge tone={TONE[status]}>{copy.status[status]}</Badge>
      </header>
      <p className="mt-3 text-title font-semibold tabular">{value}</p>
      {note && <p className={cn("mt-1 text-caption", status === "fail" ? "text-danger" : status === "warn" ? "text-warning" : "text-ink-muted")}>{note}</p>}
      {children && <dl className="mt-4 space-y-1.5 border-t border-line pt-3 text-label">{children}</dl>}
    </section>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-ink-muted">{label}</dt>
      <dd className="text-right font-mono tabular">{children}</dd>
    </div>
  );
}

function DatabaseCard({ data }: { data: SystemStatus["database"] }) {
  const c = copy.database;
  return (
    <Card title={c.title} status={data.status} value={data.status === "fail" ? "—" : ms(data.latencyMs)} note={data.status === "fail" ? c.down : data.status === "warn" ? c.slow : c.latency}>
      {data.status !== "fail" && (
        <>
          <Row label={c.size}>{formatBytes(data.sizeBytes)}</Row>
          <Row label={c.connections}>{`${data.inUse} / ${data.maxConns}`}</Row>
          <Row label={c.version}>{data.version.split(" ")[0]}</Row>
        </>
      )}
    </Card>
  );
}

function BackupsCard({ data, now }: { data: SystemStatus["backups"]; now: number }) {
  const c = copy.backups;
  const failedLast = data.lastFailedAt && (!data.lastAt || data.lastFailedAt > data.lastAt);
  const stale = data.lastAt && now - new Date(data.lastAt).getTime() > 26 * 3_600_000;
  const note = failedLast ? c.failed : !data.lastAt || stale ? c.stale : !data.offsite ? c.local : undefined;
  return (
    <Card title={c.title} status={data.status} value={data.lastAt ? formatRelative(data.lastAt, now) : c.none} note={note ?? c.last}>
      <Row label={c.schedule}>{formatHour(data.scheduleHour)}</Row>
      <Row label={c.offsite}>{data.offsite ? c.offsiteOn : c.offsiteOff}</Row>
    </Card>
  );
}

function RateCard({ data }: { data: SystemStatus["exchangeRate"] }) {
  const c = copy.rate;
  return (
    <Card title={c.title} status={data.status} value={data.today ? c.value(data.today) : "—"} note={data.today ? undefined : c.missing}>
      {!data.today && data.status !== "unknown" && (
        <div>
          <Link href="/tipo-cambio" className="text-label font-medium text-amber-ink hover:underline">
            {c.action}
          </Link>
        </div>
      )}
    </Card>
  );
}

function EmailCard({ data, now }: { data: SystemStatus["email"]; now: number }) {
  const c = copy.email;
  const stuck = data.oldestPendingAt && now - new Date(data.oldestPendingAt).getTime() > 15 * 60_000;
  return (
    <Card title={c.title} status={data.status} value={`${data.sent24h} ${c.sent.toLowerCase()}`} note={data.failed24h > 0 ? c.failing : stuck ? c.stuck : undefined}>
      <Row label={c.via}>{c.sender[data.sender]}</Row>
      <Row label={c.pending}>{data.pending}</Row>
      <Row label={c.failed}>{data.failed24h}</Row>
    </Card>
  );
}

function DiskCard({ data }: { data: SystemStatus["disk"] }) {
  const c = copy.disk;
  if (data.status === "unknown") return <Card title={c.title} status="unknown" value="—" note={c.unknown} />;
  const used = 1 - data.freeBytes / data.totalBytes;
  return (
    <Card title={c.title} status={data.status} value={formatBytes(data.freeBytes)} note={data.status === "ok" ? `${c.free} ${c.of(formatBytes(data.totalBytes))}` : c.low}>
      <div className="h-2 overflow-hidden rounded-full bg-sunken" role="img" aria-label={`${Math.round(used * 100)} %`}>
        <div className={cn("h-full rounded-full", DOT[data.status])} style={{ width: `${Math.round(used * 100)}%` }} />
      </div>
    </Card>
  );
}

function StorageCard({ data }: { data: SystemStatus["storage"] }) {
  const c = copy.storage;
  return <Card title={c.title} status={data.status} value={data.enabled ? c.on : "—"} note={data.enabled ? c.provider : c.off} />;
}

function ApiCard({ data, now }: { data: SystemStatus["api"]; now: number }) {
  const c = copy.api;
  return (
    <Card title={c.title} status={data.status} value={formatUptime(data.startedAt, now)} note={c.uptime}>
      <Row label={c.memory}>{`${data.memoryMb.toFixed(1)} MB`}</Row>
      <Row label={c.goroutines}>{data.goroutines}</Row>
      <Row label={c.version}>{[data.revision, data.goVersion].filter(Boolean).join(" · ")}</Row>
    </Card>
  );
}

function ActivityCard({ data }: { data: SystemStatus["activity"] }) {
  const c = copy.activity;
  return (
    <Card title={c.title} status="ok" value={data.activeSessions} note={`${c.sessions} · ${c.users(data.activeUsers)}`}>
      <Row label={c.shifts}>{data.openShifts}</Row>
      <Row label={c.offline}>{data.offlineSynced24h}</Row>
    </Card>
  );
}

function TrafficChart({ hours }: { hours: SystemStatus["traffic"]["hours"] }) {
  const c = copy.traffic;
  const points = hours.map((h) => ({
    hour: hourLabel.format(new Date(h.hour)),
    ok: h.requests - h.clientErrors - h.serverErrors,
    client: h.clientErrors,
    server: h.serverErrors,
  }));
  const total = hours.reduce((sum, h) => sum + h.requests, 0);
  const server = hours.reduce((sum, h) => sum + h.serverErrors, 0);
  const avg = total ? hours.reduce((sum, h) => sum + h.avgMs * h.requests, 0) / total : 0;
  const slowest = Math.max(0, ...hours.map((h) => h.maxMs));

  return (
    <ChartCard
      title={c.title}
      subtitle={c.subtitle}
      legend={
        <div className="flex flex-wrap gap-x-6 gap-y-2">
          <SwatchItem color="var(--chart-2)" label={c.ok} />
          <SwatchItem color="var(--warning)" label={c.clientErrors} />
          <SwatchItem color="var(--danger)" label={c.serverErrors} />
          <span className="ml-auto flex flex-wrap gap-x-5 text-caption text-ink-muted">
            <span>
              {c.total} <span className="font-mono tabular text-ink">{total.toLocaleString("es-NI")}</span>
            </span>
            <span>
              {c.errorRate} <span className="font-mono tabular text-ink">{server}</span>
            </span>
            <span>
              {c.avg} <span className="font-mono tabular text-ink">{ms(avg)}</span>
            </span>
            <span>
              {c.slowest} <span className="font-mono tabular text-ink">{ms(slowest)}</span>
            </span>
          </span>
        </div>
      }
      table={
        <DataList
          head={[c.hour, c.requests, c.clientErrors, c.serverErrors, c.avg]}
          rows={hours.map((h) => [hourLabel.format(new Date(h.hour)), h.requests, h.clientErrors, h.serverErrors, ms(h.avgMs)])}
        />
      }
    >
      <ResponsiveContainer width="100%" height={220}>
        <BarChart data={points} margin={{ top: 4, right: 8, bottom: 0, left: 0 }} barCategoryGap={3}>
          <CartesianGrid stroke="var(--line)" vertical={false} />
          <XAxis dataKey="hour" stroke="var(--ink-subtle)" fontSize={11} tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={16} />
          <YAxis stroke="var(--ink-subtle)" fontSize={11} tickLine={false} axisLine={false} width={36} allowDecimals={false} />
          <Tooltip content={<ChartTooltip format={(v) => v.toLocaleString("es-NI")} />} cursor={{ fill: "var(--sunken)" }} />
          <Bar dataKey="ok" name={c.ok} stackId="r" fill="var(--chart-2)" />
          <Bar dataKey="client" name={c.clientErrors} stackId="r" fill="var(--warning)" />
          <Bar dataKey="server" name={c.serverErrors} stackId="r" fill="var(--danger)" radius={[3, 3, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

function Failures({ items, now }: { items: SystemStatus["traffic"]["failed"]; now: number }) {
  const c = copy.failures;
  return (
    <section className="rounded-lg border border-line bg-surface p-5">
      <h2 className="font-semibold">{c.title}</h2>
      <p className="text-caption text-ink-muted">{c.subtitle}</p>
      {items.length === 0 ? (
        <p className="mt-4 flex items-center gap-2 text-label text-ink-muted">
          <Check size={16} className="text-success" aria-hidden />
          {c.empty}
        </p>
      ) : (
        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[640px] text-left text-label">
            <thead className="text-caption text-ink-muted">
              <tr>
                <th className="pb-1.5 font-medium">{c.when}</th>
                <th className="pb-1.5 font-medium">{c.request}</th>
                <th className="pb-1.5 font-medium">{c.status}</th>
                <th className="pb-1.5 font-medium">{c.id}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.map((f) => (
                <tr key={f.requestId + f.at}>
                  <td className="py-2" title={formatDateTime(f.at)}>
                    {formatRelative(f.at, now)}
                  </td>
                  <td className="py-2 font-mono">{`${f.method} ${f.path}`}</td>
                  <td className="py-2 font-mono tabular text-danger">{f.status}</td>
                  <td className="py-2">
                    <CopyId id={f.requestId} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function CopyId({ id }: { id: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      className="inline-flex items-center gap-1.5 font-mono text-caption text-ink-muted hover:text-ink"
      onClick={() => {
        void navigator.clipboard?.writeText(id).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
      aria-label={`${copy.failures.copy} ${id}`}
    >
      {id.slice(0, 8)}
      {copied ? <Check size={14} className="text-success" aria-hidden /> : <Copy size={14} aria-hidden />}
    </button>
  );
}
