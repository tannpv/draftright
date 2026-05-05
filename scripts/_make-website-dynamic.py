"""
One-time edit: turn dist/index.html into a JSON-driven download page.

After this runs:
  - The two download grids are empty containers populated at runtime by JS.
  - JS fetches /downloads/versions.json and renders cards via DOM APIs.
  - All install-instructions code blocks are made version-generic.
  - Future releases: edit versions.json only. Never touch HTML again.

Idempotent — safe to run multiple times.
"""
from pathlib import Path
import re
import sys

DIST = Path("/opt/openAi/DraftRight/website/dist/index.html")
html = DIST.read_text()

# ── 1. Mark the two grid containers with ids ────────────────────────────────
html, n1 = re.subn(
    r'<!-- Mobile downloads --> <h3([^>]*)>Mobile</h3> <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-16 max-w-3xl mx-auto">',
    r'<!-- Mobile downloads --> <h3\1>Mobile</h3> <div id="dl-grid-mobile" class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-16 max-w-3xl mx-auto">',
    html, count=1)
html, n2 = re.subn(
    r'<!-- Desktop downloads --> <h3([^>]*)>Desktop</h3> <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-16">',
    r'<!-- Desktop downloads --> <h3\1>Desktop</h3> <div id="dl-grid-desktop" class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-16">',
    html, count=1)
print(f"Tagged grids: mobile={n1} desktop={n2}")

# ── 2. Strip version-specific commands from install instructions ────────────
INSTALL_REPLACEMENTS = [
    (r'<pre class="bg-dark-bg/50 rounded p-3 overflow-x-auto"><code>unzip DraftRight-macOS-[^<]*</code></pre>',
     '<p class="text-gray-300">Open the .dmg, drag DraftRight to /Applications. First launch: right-click → Open (unsigned).</p>'),
    (r'<pre class="bg-dark-bg/50 rounded p-3 overflow-x-auto"><code>Expand-Archive[^<]*?dotnet run --project DraftRightWindows</code></pre>',
     '<p class="text-gray-300">Run the .exe installer. SmartScreen may warn — click "More info" → "Run anyway" (unsigned). After install, press Ctrl+Shift+R anywhere to rewrite selected text.</p>'),
    (r'tar -xzf DraftRight-Linux-[\d.]+\.tar\.gz', 'tar -xzf DraftRight-Linux-*.tar.gz'),
    (r'unzip DraftRight-iOS-Simulator-[\d.]+\.zip', 'unzip DraftRight-iOS-Simulator-*.zip'),
]
for pat, rep in INSTALL_REPLACEMENTS:
    html, n = re.subn(pat, rep, html, flags=re.DOTALL)
    print(f"  install fix: {n} hits for {pat[:40]!r}")

# ── 3. Inject hydration script just before </body> ──────────────────────────
# Builds DOM via createElement + textContent (no innerHTML; XSS-safe).
HYDRATION_SCRIPT = r"""
<script>
/* DraftRight downloads hydration —— /downloads/versions.json drives the cards */
(function () {
  function el(tag, opts) {
    var n = document.createElement(tag);
    if (!opts) return n;
    if (opts.cls) n.className = opts.cls;
    if (opts.text != null) n.textContent = opts.text;
    if (opts.href != null) n.setAttribute('href', opts.href);
    return n;
  }
  function buildCard(p, layout) {
    var a = el('a', {
      cls: 'group rounded-2xl border border-dark-border bg-dark-card p-6 hover:border-brand-400 transition-all',
      href: p.url || '#'
    });
    var icon = el('div', { cls: layout === 'desktop' ? 'text-4xl mb-3' : 'text-4xl', text: p.emoji || '⬇' });
    var title = el('h4', { cls: 'text-lg font-semibold text-white', text: p.label || '' });
    if (p.version) {
      var vs = el('span', { cls: 'text-xs text-gray-500', text: ' v' + p.version });
      title.appendChild(vs);
    }
    var meta = el('p', { cls: 'text-sm text-gray-400 mt-1', text: p.meta || '' });
    var cta = el('div', {
      cls: 'mt-' + (layout === 'desktop' ? '4' : '3') +
           ' inline-flex items-center gap-2 text-brand-400 font-medium text-sm group-hover:translate-x-1 transition-transform',
      text: (p.cta || 'Download') + ' →'
    });
    if (layout === 'desktop') {
      var center = el('div', { cls: 'text-center' });
      center.appendChild(icon); center.appendChild(title);
      center.appendChild(meta); center.appendChild(cta);
      a.appendChild(center);
    } else {
      var row = el('div', { cls: 'flex items-start gap-4' });
      var col = el('div', { cls: 'flex-1' });
      col.appendChild(title); col.appendChild(meta); col.appendChild(cta);
      row.appendChild(icon); row.appendChild(col);
      a.appendChild(row);
    }
    return a;
  }
  function fill(target, items, layout) {
    if (!target || !Array.isArray(items)) return;
    while (target.firstChild) target.removeChild(target.firstChild);
    items.forEach(function (p) { target.appendChild(buildCard(p, layout)); });
  }
  fetch('/downloads/versions.json', { cache: 'no-store' })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (d) {
      if (!d) return;
      fill(document.getElementById('dl-grid-mobile'), d.mobile, 'mobile');
      fill(document.getElementById('dl-grid-desktop'), d.desktop, 'desktop');
    })
    .catch(function (e) { console.warn('DraftRight: manifest fetch failed', e); });
})();
</script>
"""

if 'dl-grid-mobile' in html and 'DraftRight downloads hydration' not in html:
    html = html.replace('</body>', HYDRATION_SCRIPT + '</body>')
    print("Injected hydration script.")
elif 'DraftRight downloads hydration' in html:
    print("Hydration script already present — skipping.")
else:
    print("ERROR: dl-grid-mobile not found in HTML; aborting", file=sys.stderr)
    sys.exit(1)

DIST.write_text(html)
print(f"Wrote {DIST} ({len(html)} bytes)")
