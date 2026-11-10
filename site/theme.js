// Theme: dark unless the visitor picked light. The first half runs before first paint, so there is
// no flash; it is a file, not an inline script, so the page's content policy can forbid inline code.
(function () {
  'use strict';
  var KEY = 'ai-security-command-center.theme', DEFAULT = 'dark';
  var root = document.documentElement;

  function set(theme) {
    root.classList.remove('light', 'dark');
    root.classList.add(theme);
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', theme === 'light' ? '#fafafa' : '#060606');
    document.querySelectorAll('#themeSw button').forEach(function (b) {
      b.setAttribute('aria-pressed', String(b.dataset.theme === theme));
    });
  }

  var theme = DEFAULT;
  try { var saved = localStorage.getItem(KEY); if (saved === 'light' || saved === 'dark') theme = saved; } catch (e) {}
  set(theme);

  document.addEventListener('DOMContentLoaded', function () {
    set(theme); // the switch exists now
    document.querySelectorAll('#themeSw button').forEach(function (b) {
      b.addEventListener('click', function () {
        theme = b.dataset.theme;
        set(theme);
        try { localStorage.setItem(KEY, theme); } catch (e) {}
      });
    });
  });
})();
