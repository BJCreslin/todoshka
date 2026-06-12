import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/notes', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/notes', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const notes = await api.listNotes();
  app.innerHTML = layout('/notes', listHtml(notes));
  bindLayout();
  bindList();
}

function listHtml(notes) {
  return `
    <h2>Заметки</h2>
    <p><button id="newNote" class="primary">+ Новая заметка</button></p>
    ${notes.length === 0 ? '<p class="empty">Пока нет заметок</p>' : `
    <ul class="note-list">
      ${notes.map((n) => `
        <li class="note-item ${n.pinned ? 'pinned' : ''}">
          <a href="#/notes/${n.id}">
            <strong>${n.pinned ? '📌 ' : ''}${escapeHtml(n.title)}</strong>
            <small>${new Date(n.updated_at).toLocaleString('ru')}</small>
            ${n.tags && n.tags.length ? `<div class="tags-mini">${n.tags.map(escapeHtml).join(', ')}</div>` : ''}
          </a>
        </li>`).join('')}
    </ul>`}`;
}

function bindList() {
  document.getElementById('newNote').onclick = async () => {
    const title = prompt('Название заметки?');
    if (!title) return;
    const n = await api.createNote({ title, body_md: '' });
    go('/notes/' + n.id);
  };
}
