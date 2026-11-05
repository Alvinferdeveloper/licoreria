import type { FieldValues, Path, UseFormSetError } from "react-hook-form";
import { ApiError } from "./client";

/**
 * Puts the API's field errors on the matching form inputs and returns the message to show
 * above the form when the error isn't tied to a field the form renders.
 */
export function applyApiError<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  fields: readonly Path<T>[],
): string | null {
  if (!(error instanceof ApiError)) return "Ocurrió un error inesperado. Intenta de nuevo.";

  const matched = error.fields.filter((f) => fields.includes(f.field as Path<T>));
  for (const f of matched) {
    setError(f.field as Path<T>, { type: "server", message: f.message });
  }
  return matched.length > 0 ? null : error.message;
}
