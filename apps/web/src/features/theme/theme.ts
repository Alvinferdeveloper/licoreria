export type Theme = "light" | "dark";

export const THEME_STORAGE_KEY = "anejo-theme";

/** Runs before paint: applies the saved theme or the system preference to <html>. */
export const themeInitScript = `(() => {
  try {
    const saved = localStorage.getItem("${THEME_STORAGE_KEY}");
    const theme = saved === "light" || saved === "dark"
      ? saved
      : matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    document.documentElement.dataset.theme = theme;
  } catch {}
})();`;
