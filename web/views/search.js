import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/search', (m) => render(m));

async function render(m) {
  const app = document.getElementById('app');
  app.innerHTML = layout('/search', '<div class="loading">Поиск…</div>');
  bindLayout();
  const q = (m && m.query) ? new URLSearchParams(m.query).get('q') : '';
  if (!q) { app.innerHTML = layout('/search', '<h2>Поиск</h2><p>Введите запрос в боковой панели.</p>'); bindLayout(); return; }

  const [tasks, notes] = await Promise.all([api.listTasks({ q }), api.listNotes({ q })]);
  const taskItems = tasks.map((t) => `<li class="note-item"><a href="#/tasks/${t.id}">📋 ${escapeHtml(t.title)} <small>${escapeHtml(t.status)}</small></a></li>`).join('') || '<li class="empty">—</li>';
  const noteItems = notes.map((n) => `<li class="note-item"><a href="#/notes/${n.id}">📝 ${escapeHtml(n.title)}</a></li>`).join('') || '<li class="empty">—</li>';

  app.innerHTML = layout('/search', `
    <h2>Поиск: «${escapeHtml(q)}»</h2>
    <h3>Задачи</h3><ul class="note-list">${taskItems}</ul>
    <h3>Заметки</h3><ul class="note-list">${noteItems}</ul>
  `);
  bindLayout();
}
