import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/shared', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/shared', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const items = await api.shared();
  app.innerHTML = layout('/shared', listHtml(items));
  bindLayout();
}

function listHtml(items) {
  if (items.length === 0) return '<h2>Общие со мной</h2><p class="empty">Никто пока не поделился.</p>';
  return `
    <h2>Общие со мной</h2>
    <ul class="note-list">
      ${items.map((it) => {
        const d = it.data || {};
        const title = escapeHtml(d.title || '(без названия)');
        const link  = it.type === 'task' ? `#/tasks/${d.id}` : `#/notes/${d.id}`;
        return `<li class="note-item"><a href="${link}">
          <strong>${it.type === 'task' ? '📋 ' : '📝 '}${title}</strong>
        </a></li>`;
      }).join('')}
    </ul>`;
}
