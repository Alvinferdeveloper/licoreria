"use client";

import { useWatch, type UseFormReturn } from "react-hook-form";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, Input } from "@/components/ui/field";
import { cn } from "@/lib/cn";
import { settingsCopy } from "../copy";
import type { SettingsValues } from "../schema";

const copy = settingsCopy.schedule;
const ROW = "grid grid-cols-2 gap-x-4 gap-y-2 px-4 sm:grid-cols-[150px_repeat(2,minmax(0,8.5rem))_minmax(0,1fr)] sm:items-center";

export function ScheduleFields({ form }: { form: UseFormReturn<SettingsValues> }) {
  const { register, formState } = form;
  const errors = formState.errors.schedule;
  const days = useWatch({ control: form.control, name: "schedule.days" });
  const enabled = useWatch({ control: form.control, name: "schedule.enabled" });

  return (
    <>
      <Checkbox label={copy.enabled} {...register("schedule.enabled")} />
      <div className={cn("space-y-5 transition-opacity duration-200", !enabled && "opacity-60")}>
        <Field label={copy.tolerance} hint={copy.toleranceHint} error={errors?.tolerance?.message} className="max-w-xs">
          <Input
            {...register("schedule.tolerance")}
            inputMode="numeric"
            className="tabular text-right font-mono"
            placeholder={copy.tolerancePlaceholder}
          />
        </Field>
        <ul className="divide-y divide-line rounded-lg border border-line">
          <li aria-hidden className={cn(ROW, "hidden py-2 text-caption font-medium text-ink-muted sm:grid")}>
            <span />
            <span>{copy.open}</span>
            <span>{copy.close}</span>
          </li>
          {copy.order.map((i) => {
            const d = days?.[i];
            const error = errors?.days?.[i]?.close?.message;
            const overnight = d && !d.closed && d.open && d.close && d.close <= d.open;
            return (
              <li key={i} className={cn(ROW, "py-3")}>
                <Checkbox
                  label={copy.days[i]}
                  aria-label={copy.openDay(copy.days[i])}
                  className="col-span-2 sm:col-span-1"
                  checked={!d?.closed}
                  onChange={(e) => form.setValue(`schedule.days.${i}.closed`, !e.target.checked, { shouldDirty: true, shouldValidate: formState.isSubmitted })}
                />
                {d?.closed ? (
                  <p className="col-span-2 text-ink-muted sm:col-span-3">{copy.closed}</p>
                ) : (
                  <>
                    <label className="block space-y-1 text-caption text-ink-muted">
                      <span className="sm:sr-only">{copy.open}</span>
                      <Input type="time" {...register(`schedule.days.${i}.open`)} className="tabular font-mono" aria-invalid={!!error} />
                    </label>
                    <label className="block space-y-1 text-caption text-ink-muted">
                      <span className="sm:sr-only">{copy.close}</span>
                      <Input type="time" {...register(`schedule.days.${i}.close`)} className="tabular font-mono" aria-invalid={!!error} />
                    </label>
                    {(error || overnight) && (
                      <p className={cn("col-span-2 text-caption sm:col-span-1", error ? "font-medium text-danger" : "text-ink-muted")}>{error ?? copy.overnight}</p>
                    )}
                  </>
                )}
              </li>
            );
          })}
        </ul>
        {errors?.days?.root?.message || errors?.days?.message ? (
          <p className="text-caption font-medium text-danger">{errors.days.root?.message ?? errors.days.message}</p>
        ) : null}
      </div>
    </>
  );
}
