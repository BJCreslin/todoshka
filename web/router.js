const routes = [];

export function route(pattern, handler) { routes.push({ pattern, handler }); }

export function startRouter(rootEl) {
  async function run() {
    const raw = location.hash.slice(1) || '/';
    const qIndex = raw.indexOf('?');
    const path = qIndex === -1 ? raw : raw.slice(0, qIndex);
    const query = qIndex === -1 ? '' : raw.slice(qIndex + 1);
    for (const r of routes) {
      const m = match(r.pattern, path);
      if (m) {
        if (query) m.query = query;
        rootEl.innerHTML = '<div class="loading">Loading…</div>';
        try { await r.handler(m, rootEl); }
        catch (e) { rootEl.innerHTML = `<div class="error">${esc(e.message)}</div>`; }
        return;
      }
    }
    rootEl.innerHTML = '<div class="error">404 — page not found</div>';
  }
  window.addEventListener('hashchange', run);
  run();
}

function match(pattern, path) {
  const p = pattern.split('/').filter(Boolean);
  const a = path.split('/').filter(Boolean);
  if (p.length !== a.length) return null;
  const params = {};
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith('{')) params[p[i].slice(1, -1)] = decodeURIComponent(a[i]);
    else if (p[i] !== a[i]) return null;
  }
  return params;
}

export function escapeHtml(s) {
  return String(s).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

export function go(path) { location.hash = '#' + path; }

function esc(s) { return escapeHtml(s); }
