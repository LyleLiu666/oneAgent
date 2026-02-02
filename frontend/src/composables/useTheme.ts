import { ref, onMounted } from "vue";

type Theme = "light" | "dark";

const THEME_STORAGE_KEY = "theme";

const readStoredTheme = (): Theme | undefined => {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === "dark" || saved === "light") return saved;
    return undefined;
  } catch {
    return undefined;
  }
};

const prefersDarkTheme = (): boolean => {
  try {
    return Boolean(window.matchMedia?.("(prefers-color-scheme: dark)")?.matches);
  } catch {
    return false;
  }
};

export const initTheme = (): Theme => {
  if (typeof document === "undefined") return "light";

  const saved = readStoredTheme();
  const theme: Theme = saved ?? (prefersDarkTheme() ? "dark" : "light");

  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }

  return theme;
};

export function useTheme() {
  const isDark = ref(false);

  const toggleTheme = () => {
    isDark.value = !isDark.value;
    updateTheme();
  };

  const updateTheme = () => {
    if (isDark.value) {
      document.documentElement.classList.add("dark");
      localStorage.setItem(THEME_STORAGE_KEY, "dark");
    } else {
      document.documentElement.classList.remove("dark");
      localStorage.setItem(THEME_STORAGE_KEY, "light");
    }
  };

  onMounted(() => {
    try {
      const theme = initTheme();
      isDark.value = theme === "dark";
    } catch {
      // ignore
    }
  });

  return {
    isDark,
    toggleTheme,
  };
}
