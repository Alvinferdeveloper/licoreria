"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ImageSquare } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRef, useState, type ReactNode } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { NoticeBanner, useNotice } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { cashApi, cashKeys } from "@/features/cash/api";
import { ApiError } from "@/lib/api/client";
import { applyApiError } from "@/lib/api/form-errors";
import { settingsApi, settingsKeys, type PrintFormat, type Settings, type SettingsInput } from "../api";
import { settingsCopy as copy } from "../copy";

const { validation: v } = copy;
const ph = copy.business.placeholders;

const percent = z.string().trim().regex(/^\d{1,3}(\.\d{1,2})?$/, v.percent).refine((s) => Number(s) <= 100, v.percent);

const schema = z.object({
  name: z.string().trim().min(1, v.name).max(120),
  taxId: z.string().trim().max(30),
  address: z.string().trim().max(200),
  phone: z.string().trim().max(40),
  email: z.string().trim().max(120).refine((s) => s === "" || z.email().safeParse(s).success, v.email),
  taxPercent: percent,
  discountLimitPercent: percent,
  defaultPrintFormat: z.enum(["ticket", "invoice"]),
  footerNote: z.string().trim().max(160),
});

type Values = z.infer<typeof schema>;

const FIELDS = ["name", "taxId", "address", "phone", "email", "footerNote"] as const;

function toValues(s: Settings): Values {
  return {
    name: s.business.name,
    taxId: s.business.taxId,
    address: s.business.address,
    phone: s.business.phone,
    email: s.business.email,
    taxPercent: String(s.tax.defaultPercent),
    discountLimitPercent: String(s.pos.discountLimitPercent),
    defaultPrintFormat: s.pos.defaultPrintFormat === "invoice" ? "invoice" : "ticket",
    footerNote: s.invoice.footerNote,
  };
}

function toInput(v: Values): SettingsInput {
  return {
    business: { name: v.name, taxId: v.taxId, address: v.address, phone: v.phone, email: v.email },
    tax: { defaultPercent: Number(v.taxPercent) },
    pos: { discountLimitPercent: Number(v.discountLimitPercent), defaultPrintFormat: v.defaultPrintFormat as PrintFormat },
    invoice: { footerNote: v.footerNote },
  };
}

function Section({ title, hint, children }: { title: string; hint: string; children: ReactNode }) {
  return (
    <section className="grid gap-x-10 gap-y-4 border-t border-line py-8 first:border-t-0 first:pt-0 lg:grid-cols-[240px_minmax(0,1fr)]">
      <div>
        <h2 className="text-title font-semibold">{title}</h2>
        <p className="mt-1 text-ink-muted">{hint}</p>
      </div>
      <div className="min-w-0 space-y-5">{children}</div>
    </section>
  );
}

export function SettingsPage() {
  const settings = useQuery({ queryKey: settingsKeys.detail, queryFn: settingsApi.get });
  return (
    <>
      <PageHeader title={copy.title} description={copy.description} />
      <div className="max-w-5xl px-8 pb-12">
        {settings.isError && <Alert>{copy.loadError}</Alert>}
        {settings.data && <SettingsForm settings={settings.data} />}
      </div>
    </>
  );
}

function SettingsForm({ settings }: { settings: Settings }) {
  const me = useSession((s) => s.user);
  const editable = can(me, "settings:update");
  const queryClient = useQueryClient();
  const { notice, success, failure } = useNotice();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: toValues(settings) });
  const { errors } = form.formState;

  const refresh = (next: Settings) => {
    queryClient.setQueryData(settingsKeys.detail, next);
    void queryClient.invalidateQueries({ queryKey: settingsKeys.all });
  };

  const save = useMutation({
    mutationFn: settingsApi.update,
    onSuccess: (next) => {
      refresh(next);
      form.reset(toValues(next));
      success(copy.saved);
    },
    onError: (error) => setFormError(applyApiError(error, form.setError, FIELDS)),
  });

  const submit = form.handleSubmit((values) => {
    setFormError(null);
    save.mutate(toInput(values));
  });

  return (
    <form onSubmit={submit} noValidate>
      <fieldset disabled={!editable} className="min-w-0">
        <Section title={copy.business.title} hint={copy.business.hint}>
          <Field label={copy.business.name} error={errors.name?.message}>
            <Input {...form.register("name")} maxLength={120} placeholder={ph.name} />
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={copy.business.taxId} error={errors.taxId?.message}>
              <Input {...form.register("taxId")} className="font-mono" maxLength={30} autoComplete="off" placeholder={ph.taxId} />
            </Field>
            <Field label={copy.business.phone} error={errors.phone?.message}>
              <Input {...form.register("phone")} inputMode="tel" maxLength={40} placeholder={ph.phone} />
            </Field>
          </div>
          <Field label={copy.business.address} error={errors.address?.message}>
            <Input {...form.register("address")} maxLength={200} placeholder={ph.address} />
          </Field>
          <Field label={copy.business.email} error={errors.email?.message}>
            <Input {...form.register("email")} type="email" maxLength={120} placeholder={ph.email} />
          </Field>
        </Section>

        <Section title={copy.logo.title} hint={copy.logo.hint}>
          <LogoField url={settings.business.logoUrl} onChanged={refresh} onMessage={success} onFailure={failure} />
        </Section>

        <Section title={copy.sales.title} hint={copy.sales.hint}>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={copy.sales.tax} error={errors.taxPercent?.message}>
              <Input {...form.register("taxPercent")} inputMode="decimal" className="tabular text-right font-mono" placeholder={copy.sales.taxPlaceholder} />
            </Field>
            <Field label={copy.sales.discount} error={errors.discountLimitPercent?.message} hint={copy.sales.discountHint}>
              <Input
                {...form.register("discountLimitPercent")}
                inputMode="decimal"
                className="tabular text-right font-mono"
                placeholder={copy.sales.discountPlaceholder}
              />
            </Field>
          </div>
        </Section>

        <Section title={copy.invoice.title} hint={copy.invoice.hint}>
          <div className="space-y-1.5">
            <p className="text-label font-medium text-ink">{copy.invoice.format}</p>
            <Controller
              control={form.control}
              name="defaultPrintFormat"
              render={({ field }) => <SegmentedControl label={copy.invoice.format} value={field.value} options={copy.invoice.formats} onChange={field.onChange} />}
            />
          </div>
          <Field label={copy.invoice.footer} error={errors.footerNote?.message} hint={copy.invoice.footerHint}>
            <Input {...form.register("footerNote")} maxLength={160} placeholder={copy.invoice.footerPlaceholder} />
          </Field>
        </Section>
      </fieldset>

      <Section title={copy.numbering.title} hint={copy.numbering.hint}>
        <Numbering />
      </Section>

      {formError && <Alert className="mb-4">{formError}</Alert>}
      {editable && (
        <div className="flex items-center justify-end gap-4 border-t border-line pt-6">
          <div className="min-w-0 flex-1">
            <NoticeBanner notice={notice} />
          </div>
          <Button type="submit" variant="primary" loading={save.isPending}>
            {copy.save}
          </Button>
        </div>
      )}
    </form>
  );
}

function LogoField({ url, onChanged, onMessage, onFailure }: { url: string | null; onChanged: (s: Settings) => void; onMessage: (m: string) => void; onFailure: (e: unknown) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<Settings>, message: string) => {
    setBusy(true);
    setError(null);
    try {
      onChanged(await action());
      onMessage(message);
    } catch (e) {
      if (e instanceof ApiError) setError(e.fieldMessage("logo") ?? e.message);
      else onFailure(e);
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  };

  return (
    <div className="flex items-center gap-4">
      <div className="grid size-20 shrink-0 place-items-center overflow-hidden rounded-lg border border-line bg-sunken">
        {url ? (
          // eslint-disable-next-line @next/next/no-img-element -- remote Supabase URL, sized by the container
          <img src={url} alt="" className="size-full object-contain" />
        ) : (
          <ImageSquare size={28} className="text-ink-subtle" aria-hidden />
        )}
      </div>
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap gap-2">
          <input ref={input} type="file" accept="image/jpeg,image/png" className="sr-only" aria-label={copy.logo.upload} tabIndex={-1}
            onChange={(e) => { const file = e.target.files?.[0]; if (file) void run(() => settingsApi.uploadLogo(file), copy.logo.saved); }} />
          <Button size="sm" loading={busy} onClick={() => input.current?.click()}>
            {url ? copy.logo.replace : copy.logo.upload}
          </Button>
          {url && (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => void run(settingsApi.removeLogo, copy.logo.removed)}>
              {copy.logo.remove}
            </Button>
          )}
        </div>
        {error && <p className="text-caption font-medium text-danger">{error}</p>}
      </div>
    </div>
  );
}

function Numbering() {
  const registers = useQuery({ queryKey: cashKeys.registers, queryFn: cashApi.registers });
  return (
    <>
      <ul className="divide-y divide-line">
        {registers.data?.map((r) => (
          <li key={r.id} className="flex items-center justify-between gap-4 py-2.5">
            <span>{r.name}</span>
            <span className="text-ink-muted">
              {copy.numbering.nextLabel} <span className="ml-2 font-mono text-code text-ink">{copy.numbering.next(r.invoiceSeries, r.nextInvoiceNumber)}</span>
            </span>
          </li>
        ))}
      </ul>
      <Link href="/turnos" className="inline-block text-label font-medium text-amber-ink hover:underline">
        {copy.numbering.manage}
      </Link>
    </>
  );
}
