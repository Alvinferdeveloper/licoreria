"use client";

import { ArrowRight, Receipt } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { reportKeys, reportsApi, type ReportDefinition } from "../api";
import { reportsCopy as copy } from "../copy";

export function ReportsPage() {
  const catalog = useQuery({ queryKey: reportKeys.catalog, queryFn: reportsApi.catalog });
  const groups = new Map<string, ReportDefinition[]>();
  for (const r of catalog.data?.reports ?? []) groups.set(r.group, [...(groups.get(r.group) ?? []), r]);

  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="space-y-8 px-8 pb-10">
        {catalog.isError && <Alert>No pudimos cargar los reportes.</Alert>}
        {catalog.isPending && <p className="text-ink-muted">Cargando…</p>}
        {[...groups.entries()].map(([group, reports]) => (
          <section key={group} className="space-y-2">
            <h2 className="text-label font-medium text-ink-muted">{group}</h2>
            <ul className="divide-y divide-line rounded-lg border border-line bg-surface">
              {group === "Ventas" && (
                <ReportLink href="/ventas" title={copy.invoice.title} description={copy.invoice.description} icon />
              )}
              {reports.map((r) => (
                <ReportLink key={r.key} href={`/reportes/${r.key}`} title={r.title} description={r.description} />
              ))}
            </ul>
          </section>
        ))}
      </div>
    </>
  );
}

function ReportLink({ href, title, description, icon }: { href: string; title: string; description: string; icon?: boolean }) {
  return (
    <li>
      <Link href={href} className="group flex items-center justify-between gap-4 px-5 py-4 transition-colors hover:bg-chrome/60">
        <span className="min-w-0">
          <span className="flex items-center gap-2 font-medium">
            {icon && <Receipt size={16} className="text-ink-subtle" aria-hidden />}
            {title}
          </span>
          <span className="block text-label text-ink-muted">{description}</span>
        </span>
        <ArrowRight size={16} className="shrink-0 text-ink-subtle transition-transform group-hover:translate-x-0.5" aria-hidden />
      </Link>
    </li>
  );
}
