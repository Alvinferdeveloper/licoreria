"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, PasswordInput } from "@/components/ui/field";
import { ApiError } from "@/lib/api/client";
import { salesApi } from "../api";
import { pinCopy as copy } from "../copy";

/** Sets the user's authorization PIN, confirmed with their password. */
export function PinForm() {
  const [password, setPassword] = useState("");
  const [pin, setPin] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const mutation = useMutation({
    mutationFn: () => salesApi.setPin(password, pin),
    onSuccess: () => {
      setSaved(true);
      setPassword("");
      setPin("");
      setConfirm("");
    },
    onError: (e) => setError(e instanceof ApiError ? (e.fieldMessage("currentPassword") ?? e.fieldMessage("pin") ?? e.message) : "Ocurrió un error inesperado."),
  });

  return (
    <form
      className="space-y-4 border-t border-line pt-6"
      onSubmit={(e) => {
        e.preventDefault();
        setSaved(false);
        if (!/^\d{4,6}$/.test(pin)) return setError(copy.invalid);
        if (pin !== confirm) return setError(copy.mismatch);
        setError(null);
        mutation.mutate();
      }}
      noValidate
    >
      <div>
        <h2 className="font-semibold">{copy.title}</h2>
        <p className="text-label text-ink-muted">{copy.description}</p>
      </div>
      {error && <Alert>{error}</Alert>}
      {saved && <Alert tone="success">{copy.saved}</Alert>}
      <Field label={copy.password}>
        <PasswordInput value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label={copy.pin}>
          <Input value={pin} onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))} type="password" inputMode="numeric" maxLength={6} autoComplete="off" />
        </Field>
        <Field label={copy.confirm}>
          <Input value={confirm} onChange={(e) => setConfirm(e.target.value.replace(/\D/g, ""))} type="password" inputMode="numeric" maxLength={6} autoComplete="off" />
        </Field>
      </div>
      <Button type="submit" loading={mutation.isPending} disabled={!password}>
        {copy.submit}
      </Button>
    </form>
  );
}
