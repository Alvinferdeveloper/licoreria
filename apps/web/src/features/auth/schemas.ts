import { z } from "zod";
import { authCopy } from "./copy";

const { validation, rules } = authCopy;

/** Mirrors the API policy so most mistakes are caught before submitting. */
export const PASSWORD_RULES = [
  { label: rules.length, test: (v: string) => v.length >= 10 },
  { label: rules.upper, test: (v: string) => /\p{Lu}/u.test(v) },
  { label: rules.digit, test: (v: string) => /\d/.test(v) },
] as const;

const newPassword = z
  .string()
  .min(1, validation.required)
  .refine((v) => PASSWORD_RULES.every((rule) => rule.test(v)), validation.weak);

const confirmPassword = z.string().min(1, validation.required);

const passwordsMatch = {
  check: (data: { newPassword: string; confirmPassword: string }) => data.newPassword === data.confirmPassword,
  options: { path: ["confirmPassword"], message: validation.mismatch },
};

export const loginSchema = z.object({
  identifier: z.string().trim().min(1, validation.required),
  password: z.string().min(1, validation.required),
});

export const changePasswordSchema = z
  .object({ currentPassword: z.string().min(1, validation.required), newPassword, confirmPassword })
  .refine(passwordsMatch.check, passwordsMatch.options);

export const resetPasswordSchema = z
  .object({ newPassword, confirmPassword })
  .refine(passwordsMatch.check, passwordsMatch.options);

export const forgotPasswordSchema = z.object({
  email: z.string().trim().min(1, validation.required).pipe(z.email(validation.email)),
});

export type LoginValues = z.infer<typeof loginSchema>;
export type ChangePasswordValues = z.infer<typeof changePasswordSchema>;
export type ResetPasswordValues = z.infer<typeof resetPasswordSchema>;
export type ForgotPasswordValues = z.infer<typeof forgotPasswordSchema>;
