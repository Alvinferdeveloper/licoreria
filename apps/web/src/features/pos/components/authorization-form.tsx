"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import type { Authorization } from "@/features/sales/api";
import { posCopy } from "../copy";

const copy = posCopy.tender.authorization;

interface AuthorizationFormProps {
  busy: boolean;
  onSubmit: (authorization: Authorization) => void;
  title?: string;
  body?: string;
  submitLabel?: string;
}

/** Administrator username + PIN, typed at the counter to approve an action. */
export function AuthorizationForm({ busy, onSubmit, title = copy.title, body = copy.body, submitLabel = copy.submit }: AuthorizationFormProps) {
  const [username, setUsername] = useState("");
  const [pin, setPin] = useState("");
  return (
    <form
      className="space-y-3 rounded-lg border border-amber/40 bg-amber-tint/50 p-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (username.trim() && pin) onSubmit({ username: username.trim(), pin });
      }}
    >
      <div>
        <p className="font-semibold">{title}</p>
        <p className="text-caption text-ink-muted">{body}</p>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label={copy.username}>
          <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" autoCapitalize="none" placeholder={copy.usernamePlaceholder} autoFocus />
        </Field>
        <Field label={copy.pin}>
          <Input value={pin} onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))} type="password" inputMode="numeric" maxLength={6} autoComplete="off" />
        </Field>
      </div>
      <Button type="submit" variant="primary" className="w-full" loading={busy} disabled={!username.trim() || pin.length < 4}>
        {submitLabel}
      </Button>
    </form>
  );
}
