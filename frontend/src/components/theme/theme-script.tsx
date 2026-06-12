/**
 * Inline script that runs before first paint to set data-theme from localStorage.
 * Prevents flash of wrong theme. Must be in root layout.
 *
 * Carries the per-request CSP nonce (set by middleware) so it executes under a
 * strict `script-src` that no longer allows 'unsafe-inline'.
 */
export function ThemeScript({ nonce }: { nonce?: string }) {
  const script = `
(function() {
  var key = 'microblog_theme';
  var pref = localStorage.getItem(key) || 'dark';
  var resolved = pref;
  if (pref === 'system') {
    resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  document.documentElement.setAttribute('data-theme', resolved);
})();
`;
  return <script nonce={nonce} dangerouslySetInnerHTML={{ __html: script }} />;
}
