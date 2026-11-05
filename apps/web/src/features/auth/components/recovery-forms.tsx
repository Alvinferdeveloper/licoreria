"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { Alert } from "@/components/ui/alert";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, Input, PasswordInput } from "@/components/ui/field";
import { applyApiError } from "@/lib/api/form-errors";
import { authApi } from "../api";
import { authCopy } from "../copy";
import {
  forgotPasswordSchema,
  resetPasswordSchema,
  type ForgotPasswordValues,
  type ResetPasswordValues,
} from "../schemas";
import { PasswordRules } from "./password-rules";

const linkClass = "text-label font-medium text-amber-ink hover:underline";

export function ForgotPasswordForm() {
  const [sent, setSent] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<ForgotPasswordValues>({ resolver: zodResolver(forgotPasswordSchema), defaultValues: { email: "" } });
  const { errors, isSubmitting } = form.formState;

  const onSubmit = form.handleSubmit(async ({ email }) => {
    setFormError(null);
    try {
      setSent((await authApi.forgotPassword(email)).message);
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["email"]));
    }
  });

  if (sent) {
    return (
      <div className="space-y-6">
        <Alert tone="success">{sent}</Alert>
        <Link href="/login" className={linkClass}>
          {authCopy.forgot.back}
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label={authCopy.forgot.email} error={errors.email?.message}>
        <Input type="email" autoComplete="email" autoFocus {...form.register("email")} />
      </Field>
      <Button type="submit" variant="primary" size="lg" className="w-full" loading={isSubmitting}>
        {authCopy.forgot.submit}
      </Button>
      <Link href="/login" className={`${linkClass} block`}>
        {authCopy.forgot.back}
      </Link>
    </form>
  );
}

export function ResetPasswordForm({ token }: { token: string | null }) {
  const [done, setDone] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { newPassword: "", confirmPassword: "" },
  });
  const { errors, isSubmitting } = form.formState;
  const newPassword = useWatch({ control: form.control, name: "newPassword" });

  if (!token) {
    return (
      <div className="space-y-6">
        <Alert>{authCopy.reset.missingToken}</Alert>
        <Link href="/recuperar" className={linkClass}>
          {authCopy.reset.requestAnother}
        </Link>
      </div>
    );
  }

  if (done) {
    return (
      <div className="space-y-6">
        <Alert tone="success">{authCopy.reset.done}</Alert>
        <Link href="/login" className={buttonVariants({ variant: "primary", size: "lg", className: "w-full" })}>
          {authCopy.reset.goToLogin}
        </Link>
      </div>
    );
  }

  const onSubmit = form.handleSubmit(async ({ newPassword: value }) => {
    setFormError(null);
    try {
      await authApi.resetPassword(token, value);
      setDone(true);
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["newPassword"]));
    }
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && (
        <Alert>
          {formError}{" "}
          <Link href="/recuperar" className="font-medium underline">
            {authCopy.reset.requestAnother}
          </Link>
        </Alert>
      )}
      <div>
        <Field label={authCopy.changePassword.next} error={errors.newPassword?.message}>
          <PasswordInput autoComplete="new-password" autoFocus {...form.register("newPassword")} />
        </Field>
        <PasswordRules value={newPassword} />
      </div>
      <Field label={authCopy.changePassword.confirm} error={errors.confirmPassword?.message}>
        <PasswordInput autoComplete="new-password" {...form.register("confirmPassword")} />
      </Field>
      <Button type="submit" variant="primary" size="lg" className="w-full" loading={isSubmitting}>
        {authCopy.reset.submit}
      </Button>
    </form>
  );
}
