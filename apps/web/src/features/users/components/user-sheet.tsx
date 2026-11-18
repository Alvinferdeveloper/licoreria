"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, Input, inputClassName } from "@/components/ui/field";
import { Sheet } from "@/components/ui/sheet";
import type { RoleDetail } from "@/features/roles/api";
import { applyApiError } from "@/lib/api/form-errors";
import type { User, UserInput } from "../api";
import { usersCopy } from "../copy";

const { form: copy, validation } = usersCopy;

const schema = z.object({
  fullName: z.string().trim().min(3, validation.fullName).max(120),
  email: z.union([z.literal(""), z.email(validation.email)]),
  username: z.string().trim().regex(/^[a-zA-Z0-9._-]{3,32}$/, validation.username),
  roleId: z.string().min(1, validation.required),
});

const FIELDS = ["fullName", "email", "username", "roleId"] as const;

interface UserSheetProps {
  open: boolean;
  user: User | null;
  roles: RoleDetail[];
  onClose: () => void;
  onSubmit: (input: UserInput) => Promise<void>;
}

export function UserSheet({ open, user, roles, onClose, onSubmit }: UserSheetProps) {
  return (
    <Sheet open={open} onClose={onClose} title={user ? copy.editTitle : copy.createTitle} description={user ? undefined : copy.createDescription}>
      {/* Keyed so each opening starts from fresh values without syncing state in an effect. */}
      <UserForm key={user?.id ?? "new"} user={user} roles={roles} onCancel={onClose} onSubmit={onSubmit} />
    </Sheet>
  );
}

interface UserFormProps {
  user: User | null;
  roles: RoleDetail[];
  onCancel: () => void;
  onSubmit: (input: UserInput) => Promise<void>;
}

function UserForm({ user, roles, onCancel, onSubmit }: UserFormProps) {
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<UserInput>({
    resolver: zodResolver(schema),
    defaultValues: user
      ? { fullName: user.fullName, email: user.email ?? "", username: user.username, roleId: user.role.id }
      : { fullName: "", email: "", username: "", roleId: "" },
  });
  const { errors, isSubmitting } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await onSubmit(values);
    } catch (error) {
      setFormError(applyApiError(error, form.setError, FIELDS));
    }
  });

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      {formError && <Alert>{formError}</Alert>}
      <Field label={copy.fullName} error={errors.fullName?.message}>
        <Input autoComplete="off" autoFocus placeholder={copy.fullNamePlaceholder} {...form.register("fullName")} />
      </Field>
      <Field label={copy.email} hint={copy.emailHint} error={errors.email?.message}>
        <Input type="email" autoComplete="off" placeholder={copy.emailPlaceholder} {...form.register("email", { setValueAs: (v: string) => v.trim() })} />
      </Field>
      <Field label={copy.username} hint={copy.usernameHint} error={errors.username?.message}>
        <Input
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          className="font-mono"
          placeholder={copy.usernamePlaceholder}
          {...form.register("username")}
        />
      </Field>
      <Field label={copy.role} error={errors.roleId?.message}>
        <select className={inputClassName} {...form.register("roleId")}>
          <option value="">{copy.rolePlaceholder}</option>
          {roles.map((role) => (
            <option key={role.id} value={role.id}>
              {role.name}
            </option>
          ))}
        </select>
      </Field>
      <div className="flex justify-end gap-2 border-t border-line pt-5">
        <Button variant="ghost" onClick={onCancel}>
          {copy.cancel}
        </Button>
        <Button type="submit" variant="primary" loading={isSubmitting}>
          {user ? copy.save : copy.createSubmit}
        </Button>
      </div>
    </form>
  );
}
