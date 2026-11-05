"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { DataTable, type Column } from "@/components/data/data-table";
import { FilterChips, Pagination, SearchInput } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { formatDateTime } from "@/lib/format/date";
import { auditApi, auditKeys, type AuditEntry, type AuditFilters } from "../api";
import { auditCopy as copy } from "../copy";

const actionLabel = (action: string) => copy.actions[action] ?? action;

export function AuditPage() {
  const [filters, setFilters] = useState<AuditFilters>({ search: "", module: "", from: "", to: "", page: 1 });
  const [selected, setSelected] = useState<AuditEntry | null>(null);
  const list = useQuery({ queryKey: auditKeys.list(filters), queryFn: () => auditApi.list(filters), placeholderData: keepPreviousData });
  const update = (patch: Partial<AuditFilters>) => setFilters((f) => ({ ...f, page: 1, ...patch }));

  const columns: Column<AuditEntry>[] = [
    { header: copy.columns.date, skeleton: 50, cell: (e) => <span className="font-mono tabular text-label">{formatDateTime(e.createdAt)}</span> },
    { header: copy.columns.user, skeleton: 50, cell: (e) => <span className="text-label">{e.userName ?? copy.system}</span> },
    {
      header: copy.columns.action,
      skeleton: 60,
      cell: (e) => (
        <Button size="sm" variant="ghost" className="-ml-2.5" onClick={() => setSelected(e)}>
          {actionLabel(e.action)}
        </Button>
      ),
    },
    { header: copy.columns.ip, skeleton: 30, cell: (e) => <span className="font-mono tabular text-label text-ink-muted">{e.ip ?? "—"}</span> },
  ];

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-4 px-8 pb-10">
        <div className="flex flex-wrap items-end gap-3">
          <SearchInput value={filters.search} onChange={(search) => update({ search })} placeholder={copy.search} className="w-72" />
          <Field label={copy.from}>
            <Input type="date" value={filters.from} max={filters.to || undefined} onChange={(e) => update({ from: e.target.value })} />
          </Field>
          <Field label={copy.to}>
            <Input type="date" value={filters.to} min={filters.from || undefined} onChange={(e) => update({ to: e.target.value })} />
          </Field>
        </div>
        <FilterChips label={copy.moduleLabel} value={filters.module} options={copy.modules} onChange={(module) => update({ module })} />
        <DataTable
          columns={columns}
          rows={list.data?.data}
          rowKey={(e) => e.id}
          loading={list.isPending}
          error={list.isError}
          onRetry={() => list.refetch()}
          empty={copy.empty}
          onRowClick={setSelected}
          minWidth={720}
        />
        <Pagination meta={list.data?.meta} onPage={(page) => setFilters((f) => ({ ...f, page }))} />
      </div>
      <AuditDetail entry={selected} onClose={() => setSelected(null)} />
    </>
  );
}

function show(value: unknown) {
  if (value === undefined || value === null || value === "") return "—";
  return typeof value === "object" ? JSON.stringify(value) : String(value);
}

function AuditDetail({ entry, onClose }: { entry: AuditEntry | null; onClose: () => void }) {
  const d = copy.detail;
  const before = entry?.before ?? {};
  const after = entry?.after ?? {};
  const changed = [...new Set([...Object.keys(before), ...Object.keys(after)])].filter((key) => show(before[key]) !== show(after[key]));
  const compare = entry?.before != null;

  return (
    <Sheet open={entry !== null} onClose={onClose} title={d.title} description={entry ? actionLabel(entry.action) : undefined} size="lg">
      {entry && (
        <div className="space-y-6">
          <dl className="grid grid-cols-[8rem_minmax(0,1fr)] gap-x-4 gap-y-2 text-label">
            <dt className="text-ink-muted">{d.who}</dt>
            <dd>{entry.userName ?? copy.system}</dd>
            <dt className="text-ink-muted">{d.when}</dt>
            <dd className="font-mono tabular">{formatDateTime(entry.createdAt)}</dd>
            <dt className="text-ink-muted">{d.where}</dt>
            <dd className="font-mono tabular">{entry.ip ?? "—"}</dd>
            <dt className="text-ink-muted">{d.device}</dt>
            <dd className="break-words text-ink-muted">{entry.userAgent ?? "—"}</dd>
            <dt className="text-ink-muted">{d.code}</dt>
            <dd className="font-mono">{entry.action}</dd>
          </dl>
          {changed.length === 0 ? (
            <p className="text-label text-ink-muted">{d.noData}</p>
          ) : (
            <table className="w-full table-fixed text-label">
              <thead className="text-caption text-ink-muted">
                <tr>
                  <th className="w-1/4 py-1.5 text-left font-medium">{d.field}</th>
                  {compare && <th className="py-1.5 text-left font-medium">{d.before}</th>}
                  <th className="py-1.5 text-left font-medium">{d.after}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line align-top">
                {changed.map((key) => (
                  <tr key={key}>
                    <td className="py-2 pr-3 text-caption text-ink-muted">{copy.fields[key] ?? key}</td>
                    {compare && <td className="py-2 pr-3 font-mono text-caption break-words">{show(before[key])}</td>}
                    <td className="py-2 font-mono text-caption break-words">{show(after[key])}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </Sheet>
  );
}
