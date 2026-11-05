"use client";

import { useEffect, useState } from "react";
import { Button, type ButtonProps } from "./button";

interface ConfirmButtonProps extends Omit<ButtonProps, "onClick"> {
  confirmLabel: string;
  onConfirm: () => void;
}

/** Destructive action confirmed in place: the first click arms it, a second click within 3 s runs it. */
export function ConfirmButton({ confirmLabel, onConfirm, children, variant = "ghost", ...props }: ConfirmButtonProps) {
  const [armed, setArmed] = useState(false);

  useEffect(() => {
    if (!armed) return;
    const timer = setTimeout(() => setArmed(false), 3000);
    return () => clearTimeout(timer);
  }, [armed]);

  return (
    <Button
      {...props}
      variant={armed ? "danger" : variant}
      onClick={() => {
        if (!armed) return setArmed(true);
        setArmed(false);
        onConfirm();
      }}
      onBlur={() => setArmed(false)}
    >
      {armed ? confirmLabel : children}
    </Button>
  );
}
