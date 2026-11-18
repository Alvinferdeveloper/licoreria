"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Alert } from "@/components/ui/alert";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { applyApiError } from "@/lib/api/form-errors";
import { authApi } from "../api";
import { authCopy } from "../copy";
import { codeActivationSchema, resetPasswordSchema, type CodeActivationValues, type ResetPasswordValues } from "../schemas";
import { NewPasswordFields } from "./new-password-fields";

const copy = authCopy.activate;
const linkClass = "text-label font-medium text-amber-ink hover:underline";

/** First access: with the emailed link's token, or with username plus the counter code. */
export function ActivationForm({ token }: { token: string | null }) {
  const [done, setDone] = useState(false);

  if (done) {
    return (
      <div className="space-y-6">
        <Alert tone="success">{copy.done}</Alert>
        <Link href="/login" className={buttonVariants({ variant: "primary", size: "lg", className: "w-full" })}>
          {copy.goToLogin}
        </Link>
      </div>
    );
  }
  return token ? <TokenForm token={token} onDone={() => setDone(true)} /> : <CodeForm onDone={() => setDone(true)} />;
}

function TokenForm({ token, onDone }: { token: string; onDone: () => void }) {
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { newPassword: "", confirmPassword: "" },
  });

  const onSubmit = form.handleSubmit(async ({ newPassword }) => {
    setFormError(null);
    try {
      await authApi.activate({ token }, newPassword);
      onDone();
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["newPassword"]));
    }
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <NewPasswordFields form={form} autoFocus />
      <Button type="submit" variant="primary" size="lg" className="w-full" loading={form.formState.isSubmitting}>
        {copy.submit}
      </Button>
      <Link href="/activar" className={`${linkClass} block`}>
        {copy.useCode}
      </Link>
    </form>
  );
}

function CodeForm({ onDone }: { onDone: () => void }) {
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<CodeActivationValues>({
    resolver: zodResolver(codeActivationSchema),
    defaultValues: { username: "", code: "", newPassword: "", confirmPassword: "" },
  });
  const { errors, isSubmitting } = form.formState;

  const onSubmit = form.handleSubmit(async ({ username, code, newPassword }) => {
    setFormError(null);
    try {
      await authApi.activate({ username, code }, newPassword);
      onDone();
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["username", "code", "newPassword"]));
    }
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label={copy.username} error={errors.username?.message}>
        <Input
          autoComplete="username"
          autoFocus
          autoCapitalize="none"
          spellCheck={false}
          placeholder={copy.usernamePlaceholder}
          {...form.register("username")}
        />
      </Field>
      <Field label={copy.code} hint={copy.codeHint} error={errors.code?.message}>
        <Input
          autoComplete="one-time-code"
          autoCapitalize="characters"
          spellCheck={false}
          maxLength={16}
          placeholder={copy.codePlaceholder}
          className="font-mono uppercase tracking-wider"
          {...form.register("code")}
        />
      </Field>
      <NewPasswordFields form={form} />
      <Button type="submit" variant="primary" size="lg" className="w-full" loading={isSubmitting}>
        {copy.submit}
      </Button>
      <Link href="/login" className={`${linkClass} block`}>
        {copy.back}
      </Link>
    </form>
  );
}
