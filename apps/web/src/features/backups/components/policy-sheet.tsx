"use client";

import { useMutation } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import { ApiError } from "@/lib/api/client";
import { backupsApi, type BackupStatus } from "../api";
import { backupsCopy, formatHour } from "../copy";

const copy = backupsCopy.policy;
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);

interface PolicySheetProps {
  status: BackupStatus | null;
  onClose: () => void;
  onSaved: () => void;
}

/** Lets an administrator choose the daily backup hour and how many backups are kept. */
export function PolicySheet({ status, onClose, onSaved }: PolicySheetProps) {
  return (
    <Sheet
      open={status !== null}
      onClose={onClose}
      title={copy.title}
      description={copy.description}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            {copy.cancel}
          </Button>
          <Button variant="primary" type="submit" form="backup-policy">
            {copy.save}
          </Button>
        </>
      }
    >
      {status && <PolicyForm key={`${status.scheduleHour}-${status.dailyRetention}-${status.weeklyRetention}`} status={status} onSaved={onSaved} />}
    </Sheet>
  );
}

function PolicyForm({ status, onSaved }: { status: BackupStatus; onSaved: () => void }) {
  const [hour, setHour] = useState(status.scheduleHour);
  const [daily, setDaily] = useState(String(status.dailyRetention));
  const [weekly, setWeekly] = useState(String(status.weeklyRetention));
  const save = useMutation({ mutationFn: backupsApi.setPolicy, onSuccess: onSaved });

  const dailyValue = Number(daily);
  const weeklyValue = Number(weekly);
  const dailyInvalid = !Number.isInteger(dailyValue) || dailyValue < 1 || dailyValue > 30;
  const weeklyInvalid = weekly === "" || !Number.isInteger(weeklyValue) || weeklyValue < 0 || weeklyValue > 12;
  const [touched, setTouched] = useState(false);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    if (dailyInvalid || weeklyInvalid) return;
    save.mutate({ scheduleHour: hour, dailyRetention: dailyValue, weeklyRetention: weeklyValue });
  };

  return (
    <form id="backup-policy" onSubmit={submit} className="space-y-5" noValidate>
      {save.isError && <Alert tone="danger">{save.error instanceof ApiError ? save.error.message : "No pudimos guardar la programación."}</Alert>}
      <Field label={copy.hour} hint={copy.hourHint}>
        <select className={inputClassName} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
          {HOURS.map((h) => (
            <option key={h} value={h}>
              {formatHour(h)}
            </option>
          ))}
        </select>
      </Field>
      <Field label={copy.daily} hint={copy.dailyHint} error={touched && dailyInvalid ? copy.dailyHint : undefined}>
        <Input value={daily} onChange={(e) => setDaily(e.target.value)} inputMode="numeric" className="w-28 font-mono tabular" />
      </Field>
      <Field label={copy.weekly} hint={copy.weeklyHint} error={touched && weeklyInvalid ? copy.weeklyHint : undefined}>
        <Input value={weekly} onChange={(e) => setWeekly(e.target.value)} inputMode="numeric" className="w-28 font-mono tabular" />
      </Field>
      <p className="text-caption text-ink-muted">{copy.retentionNote}</p>
    </form>
  );
}
