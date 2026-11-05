"use client";

import { Moon, Sun } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useTheme } from "./use-theme";

interface ThemeToggleProps {
  lightLabel: string;
  darkLabel: string;
}

export function ThemeToggle({ lightLabel, darkLabel }: ThemeToggleProps) {
  const { theme, toggleTheme } = useTheme();
  const label = theme === "dark" ? lightLabel : darkLabel;
  return (
    <Button variant="ghost" size="icon-sm" onClick={toggleTheme} aria-label={label} title={label}>
      {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
    </Button>
  );
}
