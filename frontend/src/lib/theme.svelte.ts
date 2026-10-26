/** 主题状态：light / dark / system，持久化到 localStorage。 */

export type ThemeMode = 'light' | 'dark' | 'system';

const KEY = 'incuspanel-theme';

function initial(): ThemeMode {
  const saved = localStorage.getItem(KEY);
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved;
  return 'light';
}

export const theme = $state<{ mode: ThemeMode; resolved: 'light' | 'dark' }>({
  mode: initial(),
  resolved: 'light',
});

function systemDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function applyTheme() {
  theme.resolved = theme.mode === 'system' ? (systemDark() ? 'dark' : 'light') : theme.mode;
  const root = document.documentElement;
  root.classList.toggle('dark', theme.resolved === 'dark');
  root.style.colorScheme = theme.resolved;
}

export function setTheme(mode: ThemeMode) {
  theme.mode = mode;
  localStorage.setItem(KEY, mode);
  applyTheme();
}

export function toggleTheme() {
  setTheme(theme.resolved === 'dark' ? 'light' : 'dark');
}

export function initTheme() {
  applyTheme();
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme.mode === 'system') applyTheme();
  });
}
