"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, PasswordInput } from "@/components/ui/field";
import { applyApiError } from "@/lib/api/form-errors";
import { authApi } from "../api";
import { authCopy } from "../copy";
import { homePath } from "../navigation";
import { changePasswordSchema, type ChangePasswordValues } from "../schemas";
import { logout, useSession } from "../session-store";
import { PasswordRules } from "./password-rules";

const copy = authCopy.changePassword;

export function ChangePasswordForm() {
  const router = useRouter();
  const user = useSession((s) => s.user);
  const setUser = useSession((s) => s.setUser);
  const [formError, setFormError] = useState<string | null>(null);
  const required = user?.mustChangePassword ?? false;

  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "", confirmPassword: "" },
  });
  const { errors, isSubmitting } = form.formState;
  const newPassword = useWatch({ control: form.control, name: "newPassword" });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await authApi.changePassword(values.currentPassword, values.newPassword);
      const fresh = await authApi.me();
      setUser(fresh);
      router.replace(homePath(fresh));
    } catch (error) {
      setFormError(applyApiError(error, form.setError, ["currentPassword", "newPassword"]));
    }
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label={required ? copy.currentTemporary : copy.current} error={errors.currentPassword?.message}>
        <PasswordInput autoComplete="current-password" autoFocus {...form.register("currentPassword")} />
      </Field>
      <div>
        <Field label={copy.next} error={errors.newPassword?.message}>
          <PasswordInput autoComplete="new-password" {...form.register("newPassword")} />
        </Field>
        <PasswordRules value={newPassword} />
      </div>
      <Field label={copy.confirm} error={errors.confirmPassword?.message}>
        <PasswordInput autoComplete="new-password" {...form.register("confirmPassword")} />
      </Field>
      <div className="flex items-center gap-3 pt-1">
        <Button type="submit" variant="primary" size="lg" className="flex-1" loading={isSubmitting}>
          {copy.submit}
        </Button>
        {required && (
          <Button
            variant="ghost"
            size="lg"
            onClick={async () => {
              await logout();
              router.replace("/login");
            }}
          >
            {copy.logout}
          </Button>
        )}
      </div>
    </form>
  );
}
