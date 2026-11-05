"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { DataTable, RowActions, type Column } from "@/components/data/data-table";
import { NoticeBanner, Pagination, useNotice } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Sheet } from "@/components/ui/sheet";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { saveBlob } from "@/lib/api/client";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { backupKeys, backupsApi, type Backup } from "../api";
import { backupsCopy as copy } from "../copy";
import { RestoreSheet } from "./restore-sheet";

function formatSize(bytes: number | null) {
  if (bytes === null) return "—";
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function StatusBadge({ backup }: { backup: Backup }) {
  if (backup.status === "running") return <Badge tone="amber">{copy.status.running}</Badge>;
  if (backup.status === "failed") return <Badge tone="danger">{copy.status.failed}</Badge>;
  if (!backup.available) return <Badge tone="warning">{copy.status.missing}</Badge>;
  if (!backup.verification) return <Badge>{copy.status.unverified}</Badge>;
  return backup.verification.ok ? <Badge tone="success">{copy.status.verified}</Badge> : <Badge tone="danger">{copy.status.verifyFailed}</Badge>;
}

export function BackupsPage() {
  const me = useSession((s) => s.user);
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [restoring, setRestoring] = useState<Backup | null>(null);
  const [result, setResult] = useState<Backup | null>(null);
  const { notice, success, failure } = useNotice();

  const status = useQuery({ queryKey: backupKeys.status, queryFn: backupsApi.status });
  const list = useQuery({ queryKey: backupKeys.list(page), queryFn: () => backupsApi.list(page), placeholderData: keepPreviousData });
  const refresh = () => queryClient.invalidateQueries({ queryKey: backupKeys.all });

  const create = useMutation({
    mutationFn: backupsApi.create,
    onSuccess: (b) => success(copy.created(b.fileName)),
    onError: failure,
    onSettled: refresh,
  });
  const verify = useMutation({
    mutationFn: backupsApi.verify,
    onSuccess: setResult,
    onError: failure,
    onSettled: refresh,
  });
  const download = useMutation({
    mutationFn: async (b: Backup) => saveBlob(await backupsApi.download(b.id), b.fileName),
    onError: failure,
  });

  const canCreate = can(me, "backups:create");
  const canRestore = can(me, "backups:authorize");
  const rows = list.data?.data;
  const latest = page === 1 ? rows?.find((b) => b.status === "completed") : undefined;
  const lastDrill = rows?.filter((b) => b.verifiedAt).sort((a, b) => b.verifiedAt!.localeCompare(a.verifiedAt!))[0];

  const columns: Column<Backup>[] = [
    { header: copy.columns.date, skeleton: 40, cell: (b) => <span className="font-mono tabular text-label">{formatDateTime(b.createdAt)}</span> },
    { header: copy.columns.kind, skeleton: 30, cell: (b) => <span className="text-label">{copy.kinds[b.kind]}</span> },
    { header: copy.columns.size, className: "text-right", skeleton: 30, cell: (b) => <span className="font-mono tabular text-label">{formatSize(b.sizeBytes)}</span> },
    { header: copy.columns.by, skeleton: 40, cell: (b) => <span className="text-label text-ink-muted">{b.createdBy?.name ?? copy.system}</span> },
    {
      header: copy.columns.status,
      skeleton: 30,
      cell: (b) => (
        <span className="flex flex-wrap gap-1">
          <StatusBadge backup={b} />
          {b.offsite && <Badge>{copy.status.offsite}</Badge>}
        </span>
      ),
    },
    {
      header: "",
      skeleton: 0,
      cell: (b) =>
        b.available && (
          <RowActions>
            {b.verification && (
              <Button size="sm" variant="ghost" onClick={() => setResult(b)}>
                {copy.actions.result}
              </Button>
            )}
            {canCreate && (
              <>
                <Button size="sm" variant="ghost" loading={verify.isPending && verify.variables === b.id} disabled={verify.isPending} onClick={() => verify.mutate(b.id)}>
                  {copy.actions.verify}
                </Button>
                <Button size="sm" variant="ghost" loading={download.isPending && download.variables?.id === b.id} onClick={() => download.mutate(b)}>
                  {copy.actions.download}
                </Button>
              </>
            )}
            {canRestore && (
              <Button size="sm" onClick={() => setRestoring(b)}>
                {copy.actions.restore}
              </Button>
            )}
          </RowActions>
        ),
    },
  ];

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          canCreate && (
            <Button variant="primary" loading={create.isPending} onClick={() => create.mutate()}>
              {copy.create}
            </Button>
          )
        }
      />
      <div className="space-y-6 px-8 pb-10">
        <dl className="grid divide-line rounded-lg border border-line bg-surface sm:grid-cols-2 sm:divide-x xl:grid-cols-4">
          <Fact label={copy.summary.last}>{latest ? formatRelative(latest.createdAt) : page === 1 && rows ? copy.summary.none : "—"}</Fact>
          <Fact label={copy.summary.automatic}>
            {status.data ? copy.summary.schedule(status.data.scheduleHour, status.data.dailyRetention, status.data.weeklyRetention) : "—"}
          </Fact>
          <Fact label={copy.summary.offsite} warn={status.data && !status.data.offsite}>
            {status.data ? (status.data.offsite ? copy.summary.offsiteOn : copy.summary.offsiteOff) : "—"}
          </Fact>
          <Fact label={copy.summary.drill}>{lastDrill?.verifiedAt ? formatRelative(lastDrill.verifiedAt) : copy.summary.drillNone}</Fact>
        </dl>
        <NoticeBanner notice={notice} />
        <DataTable columns={columns} rows={rows} rowKey={(b) => b.id} loading={list.isPending} error={list.isError} onRetry={() => list.refetch()} empty={copy.empty} minWidth={920} />
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </div>
      <RestoreSheet backup={restoring} onClose={() => setRestoring(null)} />
      <DrillSheet backup={result} onClose={() => setResult(null)} />
    </>
  );
}

function Fact({ label, warn, children }: { label: string; warn?: boolean; children: React.ReactNode }) {
  return (
    <div className="px-5 py-4">
      <dt className="text-caption text-ink-muted">{label}</dt>
      <dd className={warn ? "mt-1 text-label text-warning" : "mt-1 text-label"}>{children}</dd>
    </div>
  );
}

function DrillSheet({ backup, onClose }: { backup: Backup | null; onClose: () => void }) {
  const drill = copy.drill;
  const verification = backup?.verification;
  return (
    <Sheet open={backup !== null} onClose={onClose} title={drill.title} description={backup?.verifiedAt ? drill.description(formatDateTime(backup.verifiedAt)) : undefined}>
      {verification && (
        <div className="space-y-4">
          <Alert tone={verification.ok ? "success" : "danger"}>{verification.ok ? drill.ok : drill.failed}</Alert>
          {verification.tables && (
            <table className="w-full text-label">
              <thead className="text-caption text-ink-muted">
                <tr>
                  <th className="py-1.5 text-left font-medium">{drill.table}</th>
                  <th className="py-1.5 text-right font-medium">{drill.rows}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {verification.tables.map((t) => (
                  <tr key={t.table}>
                    <td className="py-2">{drill.tables[t.table] ?? t.table}</td>
                    <td className="py-2 text-right font-mono tabular">{t.rows.toLocaleString("es-NI")}</td>
                  </tr>
                ))}
                <tr>
                  <td className="py-2 text-ink-muted">{drill.schema}</td>
                  <td className="py-2 text-right font-mono tabular text-ink-muted">{verification.schemaVersion}</td>
                </tr>
              </tbody>
            </table>
          )}
        </div>
      )}
    </Sheet>
  );
}
