"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, PasswordInput } from "@/components/ui/field";
import { applyApiError } from "@/lib/api/form-errors";
import { authApi } from "../api";
import { authCopy } from "../copy";
import { loginSchema, type LoginValues } from "../schemas";
import { useSession } from "../session-store";

const copy = authCopy.login;

/** Stores the session on success; LoginScreen takes care of where to go next. */
export function LoginForm() {
  const setSession = useSession((s) => s.setSession);
  const [formError, setFormError] = useState<string | null>(null);

  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { identifier: "", password: "" } });
  const { errors, isSubmitting } = form.formState;

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      setSession(await authApi.login(values.identifier, values.password));
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["identifier", "password"]));
      form.resetField("password", { keepError: true });
      form.setFocus("password");
    }
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label={copy.identifier} error={errors.identifier?.message}>
        <Input autoComplete="username" autoFocus autoCapitalize="none" spellCheck={false} {...form.register("identifier")} />
      </Field>
      <Field
        label={copy.password}
        error={errors.password?.message}
        aside={
          <Link href="/recuperar" className="text-caption font-medium text-amber-ink hover:underline">
            {copy.forgot}
          </Link>
        }
      >
        <PasswordInput autoComplete="current-password" {...form.register("password")} />
      </Field>
      <Button type="submit" variant="primary" size="lg" className="w-full" loading={isSubmitting}>
        {copy.submit}
      </Button>
      <p className="text-label text-ink-muted">
        {copy.hasCode}{" "}
        <Link href="/activar" className="font-medium text-amber-ink hover:underline">
          {copy.activate}
        </Link>
      </p>
    </form>
  );
}
