/**
 * Inline script that runs before first paint to set data-theme from localStorage.
 * Prevents flash of wrong theme. Must be in root layout.
 *
 * Carries the per-request CSP nonce (set by src/proxy.ts) so it executes under a
 * strict `script-src` that no longer allows 'unsafe-inline'.
 */
export function ThemeScript({ nonce }: { nonce?: string }) {
  const script = `
(function() {
  var pref = 'dark';
  try {
    // zustand persist stores {"state":{"preference":"light"},"version":0}.
    var stored = JSON.parse(localStorage.getItem('microblog_theme') || 'null');
    if (stored && stored.state && stored.state.preference) pref = stored.state.preference;
  } catch (e) {}
  var resolved = pref;
  if (pref === 'system') {
    resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  document.documentElement.setAttribute('data-theme', resolved);
})();
`;
  return <script nonce={nonce} dangerouslySetInnerHTML={{ __html: script }} />;
}
