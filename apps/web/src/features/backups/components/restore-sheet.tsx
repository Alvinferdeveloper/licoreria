"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, PasswordInput } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { useSession } from "@/features/auth/session-store";
import { ApiError } from "@/lib/api/client";
import { formatDateTime } from "@/lib/format/date";
import { backupsApi, CONFIRMATION_PHRASE, type Backup } from "../api";
import { backupsCopy } from "../copy";

const copy = backupsCopy.restore;
const FORM_ID = "restore-backup";

export function RestoreSheet({ backup, onClose }: { backup: Backup | null; onClose: () => void }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [password, setPassword] = useState("");
  const [phrase, setPhrase] = useState("");
  const mutation = useMutation({
    mutationFn: () => backupsApi.restore(backup!.id, { currentPassword: password, confirmation: phrase }),
    onSettled: () => setPassword(""),
  });

  const close = () => {
    if (mutation.isPending) return;
    setPhrase("");
    mutation.reset();
    onClose();
  };

  const signIn = () => {
    useSession.getState().clear();
    queryClient.clear();
    router.replace("/login");
  };

  const when = backup ? formatDateTime(backup.createdAt) : "";
  const error = mutation.error;
  const fieldError = error instanceof ApiError ? error.fieldMessage("currentPassword") : undefined;
  const phraseError = error instanceof ApiError ? error.fieldMessage("confirmation") : undefined;
  const generalError = error && !fieldError && !phraseError ? (error instanceof ApiError ? error.message : "Ocurrió un error inesperado.") : null;

  if (mutation.isSuccess) {
    return (
      <Sheet open={backup !== null} onClose={signIn} title={copy.doneTitle} footer={<Button variant="primary" onClick={signIn}>{copy.signIn}</Button>}>
        <div className="space-y-3">
          <Alert tone="success">{copy.done(when)}</Alert>
          <p className="font-mono text-caption break-all text-ink-muted">{copy.safety(mutation.data.safetyCopy.fileName)}</p>
        </div>
      </Sheet>
    );
  }

  return (
    <Sheet
      open={backup !== null}
      onClose={close}
      title={copy.title}
      description={copy.description(when)}
      footer={
        <>
          <Button onClick={close} disabled={mutation.isPending}>
            {copy.cancel}
          </Button>
          <Button type="submit" form={FORM_ID} variant="danger" loading={mutation.isPending} disabled={phrase !== CONFIRMATION_PHRASE || password === ""}>
            {copy.submit}
          </Button>
        </>
      }
    >
      <form
        id={FORM_ID}
        className="space-y-5"
        onSubmit={(e) => {
          e.preventDefault();
          if (phrase === CONFIRMATION_PHRASE && password !== "") mutation.mutate();
        }}
        noValidate
      >
        <ul className="list-disc space-y-1.5 pl-4 text-label text-ink-muted">
          {copy.consequences.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
        {generalError && <Alert>{generalError}</Alert>}
        {mutation.isPending && <Alert tone="info">{copy.running}</Alert>}
        <Field label={copy.password} error={fieldError}>
          <PasswordInput value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" disabled={mutation.isPending} />
        </Field>
        <Field label={copy.phrase(CONFIRMATION_PHRASE)} error={phraseError}>
          <Input
            value={phrase}
            onChange={(e) => setPhrase(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            placeholder={CONFIRMATION_PHRASE}
            className="font-mono"
            disabled={mutation.isPending}
          />
        </Field>
      </form>
    </Sheet>
  );
}
