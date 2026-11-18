"use client";

import { useWatch, type UseFormReturn } from "react-hook-form";
import { Field, PasswordInput } from "@/components/ui/field";
import { authCopy } from "../copy";
import { PasswordRules } from "./password-rules";

type PasswordPair = { newPassword: string; confirmPassword: string };

/** New password with live rules plus its confirmation, for any form that contains both fields. */
export function NewPasswordFields<T extends PasswordPair>({ form, autoFocus }: { form: UseFormReturn<T>; autoFocus?: boolean }) {
  const pair = form as unknown as UseFormReturn<PasswordPair>;
  const { errors } = pair.formState;
  const value = useWatch({ control: pair.control, name: "newPassword" });
  return (
    <>
      <div>
        <Field label={authCopy.changePassword.next} error={errors.newPassword?.message}>
          <PasswordInput autoComplete="new-password" autoFocus={autoFocus} {...pair.register("newPassword")} />
        </Field>
        <PasswordRules value={value} />
      </div>
      <Field label={authCopy.changePassword.confirm} error={errors.confirmPassword?.message}>
        <PasswordInput autoComplete="new-password" {...pair.register("confirmPassword")} />
      </Field>
    </>
  );
}
