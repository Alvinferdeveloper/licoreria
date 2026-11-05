import { Check, Circle } from "@phosphor-icons/react/dist/ssr";
import { cn } from "@/lib/cn";
import { PASSWORD_RULES } from "../schemas";

/** Live checklist under a new-password field; each rule ticks as soon as it's met. */
export function PasswordRules({ value }: { value: string }) {
  return (
    <ul className="mt-2 grid gap-1" aria-label="Requisitos de la contraseña">
      {PASSWORD_RULES.map((rule) => {
        const met = rule.test(value);
        return (
          <li
            key={rule.label}
            className={cn("flex items-center gap-2 text-caption transition-colors duration-150", met ? "text-success" : "text-ink-muted")}
          >
            {met ? <Check size={14} weight="bold" aria-hidden /> : <Circle size={14} aria-hidden />}
            <span>
              {rule.label}
              <span className="sr-only">{met ? ", cumplido" : ", pendiente"}</span>
            </span>
          </li>
        );
      })}
    </ul>
  );
}
