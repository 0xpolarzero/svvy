import type { AppAppearance } from "../shared/agent-settings";

export type EffectiveTheme = "light" | "dark";

export function resolveEffectiveTheme(
  appearance: AppAppearance,
  prefersDark: boolean,
): EffectiveTheme {
  if (appearance === "dark" || appearance === "light") return appearance;
  return prefersDark ? "dark" : "light";
}

export function applyAppAppearance(appearance: AppAppearance): () => void {
  const root = document.documentElement;
  const media = window.matchMedia("(prefers-color-scheme: dark)");

  const sync = () => {
    const effectiveTheme = resolveEffectiveTheme(appearance, media.matches);
    root.classList.toggle("dark", effectiveTheme === "dark");
    root.classList.toggle("theme-light", appearance === "light");
    root.classList.toggle("theme-dark", appearance === "dark");
    root.classList.toggle("theme-system", appearance === "system");
    root.dataset.appearance = appearance;
    root.dataset.theme = effectiveTheme;
    root.style.colorScheme = effectiveTheme;
  };

  sync();
  if (appearance === "system") {
    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", sync);
      return () => media.removeEventListener("change", sync);
    }
    const legacyMedia = media as MediaQueryList & {
      addListener(listener: () => void): void;
      removeListener(listener: () => void): void;
    };
    legacyMedia.addListener(sync);
    return () => legacyMedia.removeListener(sync);
  }
  return () => {};
}
