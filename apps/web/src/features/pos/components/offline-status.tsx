"use client";

import { ArrowsClockwise, CloudSlash, WarningCircle } from "@phosphor-icons/react";
import { useState } from "react";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Sheet } from "@/components/ui/sheet";
import { useSession } from "@/features/auth/session-store";
import { formatDateTime, formatRelative } from "@/lib/format/date";
import { posCopy } from "../copy";
import { useCachedCatalog } from "../offline/catalog-cache";
import { useConnectivity } from "../offline/connectivity";
import { useOutbox } from "../offline/outbox";

const copy = posCopy.offline;

/** Header pill with the connection and outbox state; opens the list of offline sales. */
export function OfflineStatus() {
  const online = useConnectivity((s) => s.online);
  const { items, syncing, synced } = useOutbox();
  const [open, setOpen] = useState(false);
  const pending = items.filter((i) => i.status === "pending").length;
  const failed = items.length - pending;

  if (online && items.length === 0 && synced.length === 0) return null;

  const pill = !online ? (
    <Badge tone="amber">
      <CloudSlash size={14} weight="bold" aria-hidden />
      {copy.badge}
      {pending > 0 && ` · ${copy.pending(pending)}`}
    </Badge>
  ) : failed > 0 ? (
    <Badge tone="danger">
      <WarningCircle size={14} weight="bold" aria-hidden />
      {copy.failed(failed)}
    </Badge>
  ) : syncing ? (
    <Badge tone="amber">
      <ArrowsClockwise size={14} weight="bold" aria-hidden className="animate-spin" />
      {copy.sending}
    </Badge>
  ) : pending > 0 ? (
    <Badge tone="amber">{copy.pending(pending)}</Badge>
  ) : (
    <Badge tone="success">{copy.sheet.allSent}</Badge>
  );

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className="press rounded-full">
        {pill}
      </button>
      <OutboxSheet open={open} onClose={() => setOpen(false)} />
    </>
  );
}

function OutboxSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const userId = useSession((s) => s.user?.id);
  const online = useConnectivity((s) => s.online);
  const { items, synced, syncing, sync, retry, discard } = useOutbox();

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={copy.sheet.title}
      description={copy.sheet.description}
      footer={
        online && items.some((i) => i.status === "pending") && userId ? (
          <Button variant="primary" loading={syncing} onClick={() => void sync(userId)}>
            {copy.sheet.sendNow}
          </Button>
        ) : undefined
      }
    >
      {items.length === 0 ? (
        <p className="text-ink-muted">{copy.sheet.empty}</p>
      ) : (
        <ul className="divide-y divide-line">
          {items.map((sale) => (
            <li key={sale.key} className="space-y-1.5 py-3">
              <div className="flex items-center justify-between gap-3">
                <span className="font-mono text-label">{sale.receipt.code}</span>
                <Money cents={sale.receipt.total} className="font-mono font-medium" />
              </div>
              <div className="flex items-center justify-between gap-3 text-caption text-ink-muted">
                <span>{formatDateTime(sale.receipt.createdAt)}</span>
                <Badge tone={sale.status === "failed" ? "danger" : "amber"}>
                  {sale.status === "failed" ? copy.sheet.failedLabel : copy.sheet.pendingLabel}
                </Badge>
              </div>
              {sale.error && <p className="text-caption font-medium text-danger">{sale.error}</p>}
              {sale.status === "failed" && (
                <div className="flex justify-end gap-2">
                  <ConfirmButton size="sm" confirmLabel={copy.sheet.confirmDiscard} onConfirm={() => void discard(sale.key)}>
                    {copy.sheet.discard}
                  </ConfirmButton>
                  <Button
                    size="sm"
                    onClick={async () => {
                      await retry(sale.key);
                      if (userId && online) await sync(userId);
                    }}
                  >
                    {copy.sheet.retry}
                  </Button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {synced.length > 0 && (
        <section className="mt-6 space-y-2">
          <h3 className="text-caption font-medium text-ink-muted">{copy.sheet.syncedTitle}</h3>
          <ul className="space-y-1 font-mono text-label">
            {synced.map((s) => (
              <li key={s.code}>{copy.sheet.synced(s.code, s.invoice)}</li>
            ))}
          </ul>
        </section>
      )}
    </Sheet>
  );
}

/** Notice shown under the header while the server can't be reached. */
export function OfflineBanner() {
  const online = useConnectivity((s) => s.online);
  const savedAt = useCachedCatalog((s) => s.catalog?.savedAt);
  if (online) return null;
  return (
    <div role="status" className="border-b border-amber/30 bg-amber-tint px-5 py-2 text-label text-amber-ink">
      {copy.banner}
      {savedAt && <span className="ml-1 text-ink-muted">{copy.catalogAge(formatRelative(savedAt))}</span>}
    </div>
  );
}
