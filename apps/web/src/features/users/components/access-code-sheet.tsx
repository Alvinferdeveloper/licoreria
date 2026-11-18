"use client";

import { Check, Copy } from "@phosphor-icons/react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Sheet } from "@/components/ui/sheet";
import { formatDateTime } from "@/lib/format/date";
import type { Access, User } from "../api";
import { usersCopy } from "../copy";

const copy = usersCopy.code;

/** Shows a counter access code once, with the steps the person follows to activate. */
export function AccessCodeSheet({ grant, onClose }: { grant: { user: User; access: Access } | null; onClose: () => void }) {
  return (
    <Sheet
      open={grant !== null}
      onClose={onClose}
      title={copy.title}
      description={grant ? copy.description(grant.user.fullName) : undefined}
    >
      {grant?.access.code && <CodeContent user={grant.user} access={grant.access} code={grant.access.code} onClose={onClose} />}
    </Sheet>
  );
}

function CodeContent({ user, access, code, onClose }: { user: User; access: Access; code: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const origin = typeof window === "undefined" ? "" : window.location.host;

  const copyCode = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-line bg-paper px-5 py-6 text-center">
        <p className="font-mono text-[2rem] leading-none font-semibold tracking-[0.12em] text-ink tabular-nums" aria-live="polite">
          {code}
        </p>
        <Button variant="ghost" size="sm" className="mt-4" onClick={copyCode}>
          {copied ? <Check size={16} aria-hidden /> : <Copy size={16} aria-hidden />}
          {copied ? copy.copied : copy.copy}
        </Button>
      </div>

      <div>
        <h3 className="text-label font-semibold text-ink">{copy.stepsTitle}</h3>
        <ol className="mt-2 list-decimal space-y-1 pl-5 text-body text-ink-muted">
          {copy.steps(origin, user.username).map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </div>

      <p className="text-caption text-ink-muted">
        {copy.expires(formatDateTime(access.expiresAt))} {copy.replaces}
      </p>

      <div className="flex justify-end border-t border-line pt-5">
        <Button variant="primary" onClick={onClose}>
          {copy.done}
        </Button>
      </div>
    </div>
  );
}
